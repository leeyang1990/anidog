package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gorm.io/gorm"

	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/database"
	"github.com/anidog/anidog-go/internal/downloader"
	"github.com/anidog/anidog-go/internal/handler"

	// 导入下载器 provider 以触发注册
	_ "github.com/anidog/anidog-go/internal/downloader/providers/qbittorrent"
	_ "github.com/anidog/anidog-go/internal/downloader/providers/mock"
	"github.com/anidog/anidog-go/internal/middleware"
	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service"
	animesvc "github.com/anidog/anidog-go/internal/service/anime"
	authsvc "github.com/anidog/anidog-go/internal/service/auth"
	bangumisvc "github.com/anidog/anidog-go/internal/service/bangumi"
	dashboardsvc "github.com/anidog/anidog-go/internal/service/dashboard"
	dlservice "github.com/anidog/anidog-go/internal/service/download"
	"github.com/anidog/anidog-go/internal/service/episode"
	notifsvc "github.com/anidog/anidog-go/internal/service/notification"
	"github.com/anidog/anidog-go/internal/service/network"
	"github.com/anidog/anidog-go/internal/service/orchestrator"
	rssservice "github.com/anidog/anidog-go/internal/service/rss"
	"github.com/anidog/anidog-go/internal/service/scheduler"
	settingsvc "github.com/anidog/anidog-go/internal/service/setting"
	"github.com/anidog/anidog-go/internal/service/stream"
	streamrulesvc "github.com/anidog/anidog-go/internal/service/streamrule"
	usersvc "github.com/anidog/anidog-go/internal/service/user"
	"github.com/anidog/anidog-go/internal/ws"
)

func main() {
	// 1. 加载配置
	cfg := config.Load()

	// 2. 初始化日志
	initLogger(cfg)
	zap.L().Info(fmt.Sprintf("启动 %s %s...", cfg.ProjectName, cfg.ProjectVersion))

	// 3. 初始化数据库
	db := database.Init(cfg)

	// 3a. 从 DB setting 覆盖运行时配置（目前只覆盖代理）
	applyDBConfigOverrides(db, cfg)

	// 4. WebSocket Hub
	wsHub := ws.NewHub()
	go wsHub.Run()

	// 5. 构建服务
	httpClient := network.NewHTTPClient(cfg)
	bangumiSvc := service.NewBangumiService(cfg)
	streamManager := stream.NewStreamManager(cfg, httpClient, db)

	// 5a. 认证与用户服务
	authSvc := authsvc.New(db, cfg.SecretKey, cfg.AccessTokenExpireDuration)
	userSvc := usersvc.New(db)
	animeSvc := animesvc.New(db)

	// 5a. 统一下载服务
	dlSvc := dlservice.NewService(db, cfg, wsHub)
	dlSvc.RegisterExecutor(model.DownloadTypeStream, dlservice.NewStreamExecutor(streamManager, db))
	qbitClient, err := downloader.Create(cfg.DownloaderType, cfg)
	if err != nil {
		zap.L().Warn("创建下载器失败，种子下载功能将不可用", zap.Error(err))
		qbitClient = nil
	}
	if qbitClient != nil {
		dlSvc.RegisterExecutor(model.DownloadTypeTorrent, dlservice.NewTorrentExecutor(qbitClient))
	}

	// 5b. Bangumi 自动下载（保留，用于订阅即时触发 + 手动检查）
	bangumiAutoDL := bangumisvc.NewAutoDownloader(db, dlSvc, streamManager)

	// 5c. RSS Engine + CRUD
	rssCrudSvc := rssservice.NewCRUDService(db)
	rssEngine := rssservice.NewEngine(db, dlSvc)

	// 5d. Dashboard / Notification / StreamRule / Settings
	dashboardSvc := dashboardsvc.New(db, bangumiSvc)
	notifSvc := notifsvc.NewService(db)
	streamRuleSvc := streamrulesvc.NewService(db, streamManager)
	settingSvc := settingsvc.NewService(cfg, db)

	// 把通知服务注入下载服务：所有下载完成事件（BT/Stream/手动）走 updateStatus 时
	// 都会触发一次通知。这个调用得放在 dlSvc 创建之后，看上面 5b 阶段。
	dlSvc.SetNotificationService(notifSvc)

	// 下载根目录：BT/RSS 都按 <mediaRoot>/<番剧名 (年份)>/Season NN 组织
	mediaRoot := cfg.MediaRoot
	if mediaRoot == "" {
		mediaRoot = "/downloads"
	}
	rssEngine.SetMediaRoot(mediaRoot)

	// 5d2. Orchestrator：多源剧集填坑调度器（替代旧的 bangumi.CheckAllSubscribed）
	orch := orchestrator.New(db, dlSvc, streamManager, settingSvc, nil, mediaRoot)

	// 5d3. Episode 同步：从 Bangumi 拉取每集的播出时间，让前端能区分
	// "未下载" 和 "待发布"，让 Orchestrator 不去搜未播出的集
	episodeSvc := episode.NewService(db, bangumiSvc)

	// Seed 内置 Kazumi 默认规则 (仅在数据库为空时)
	if err := streamRuleSvc.SeedDefaultRules(context.Background()); err != nil {
		zap.L().Warn("Seed 默认规则失败", zap.Error(err))
	}

	// 5e. 调度器
	sched := scheduler.New()
	rssInterval := time.Duration(cfg.RSSCheckInterval) * time.Minute
	if rssInterval <= 0 {
		rssInterval = 30 * time.Minute
	}
	// RSS 是否启用：每轮 Run 之前从 setting 读取 download.source_enabled.rss，
	// 关闭时直接跳过 RefreshAll（实现"动森设置 → RSS 开关"的纯被动语义）。
	rssEnabled := func(ctx context.Context) bool {
		pref := orchestrator.LoadGlobal(ctx, settingSvc)
		return pref.RSSEnabled
	}
	sched.Register(scheduler.NewRSSRefreshJob(rssEngine, rssEnabled), rssInterval, true)
	renameInterval := time.Duration(cfg.RenameInterval) * time.Second
	if renameInterval <= 0 {
		renameInterval = 300 * time.Second
	}
	sched.Register(scheduler.NewRenameJob(), renameInterval, false)
	// 追番更新检查（每 30 分钟）
	// 番剧更新检查：改为由 Orchestrator 驱动多源下载
	sched.Register(scheduler.NewBangumiCheckJob(orch), 30*time.Minute, false)
	// 剧集元数据同步（每 6h）：拉 Bangumi /v0/episodes 更新 air_date
	sched.Register(episodeSvc, 6*time.Hour, true)
	// 失败重试（每 5min）：扫 transient 失败行，到点触发对应 anime 的 orchestrator 重排
	retryConductor := scheduler.RetryConductor(&orchRetryAdapter{orch: orch})
	retryPrefLoader := func(ctx context.Context) interface{} {
		return orchestrator.LoadGlobal(ctx, settingSvc)
	}
	sched.Register(scheduler.NewRetryFailedJob(db, retryConductor, retryPrefLoader), 5*time.Minute, false)
	// 死种黑名单 TTL 清理（每 6h 一次，14 天过期）
	sched.Register(scheduler.NewAbandonedTorrentTTLJob(db, 14*24*time.Hour), 6*time.Hour, true)
	// 源健康检测（每 3 分钟）
	sourceHealthSvc := bangumisvc.NewSourceHealthService(db)
	sched.Register(scheduler.NewSourceHealthJob(sourceHealthSvc), 3*time.Minute, true)

	// qBit 进度同步：每 15 秒把 qBit 种子的 size/progress 写回 DB
	if qbitClient != nil {
		qbitSync := dlservice.NewQBitSyncer(db, cfg)
		// 注入通知服务：当一条下载从非完成态翻成 completed 时，
		// QBitSyncer 会广播到所有 enabled 渠道（telegram/bark/...）
		qbitSync.SetNotificationService(notifSvc)
		sched.Register(qbitSync, 15*time.Second, true)
	}

	// 6. 启动流媒体管理器（异步，不阻塞主流程）
	go func() {
		if err := streamManager.Start(); err != nil {
			zap.L().Warn("流媒体管理器启动失败，流媒体功能不可用", zap.Error(err))
		}
		// 流媒体就绪后恢复未完成的下载任务
		dlSvc.RecoverPending(context.Background())
	}()

	// 7. 启动调度器
	sched.Start()

	// 8. 创建 Gin 引擎
	if cfg.LogLevel != "DEBUG" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.Logger())
	router.Use(middleware.CORS(cfg))
	router.Use(middleware.AuthMiddleware(cfg))

	// 9. 注册路由
	v1 := router.Group("/api/v1")

	handler.NewAuthHandler(authSvc).RegisterRoutes(v1)
	handler.NewUserHandler(userSvc, authSvc).RegisterRoutes(v1)
	handler.NewAnimeHandler(animeSvc, bangumiAutoDL).RegisterRoutes(v1)
	handler.NewRSSHandler(rssCrudSvc, rssEngine).RegisterRoutes(v1)
	handler.NewDownloadHandler(dlSvc).RegisterRoutes(v1)
	handler.NewSettingsHandler(settingSvc).
		WithSystemDeps(handler.SystemInfoDeps{
			DB: db,
			QBitPing: func(ctx context.Context) (bool, string) {
				if qbitClient == nil {
					return false, ""
				}
				// 用 GetTorrentInfo 当 ping —— 能返回（即便空列表）即视为在线。
				// qbit provider 未暴露版本 API，版本字段留空。
				if _, err := qbitClient.GetTorrentInfo(ctx, ""); err != nil {
					return false, ""
				}
				return true, ""
			},
		}).
		RegisterRoutes(v1)
	handler.NewDashboardHandler(dashboardSvc).RegisterRoutes(v1)
	handler.NewSearchHandler(bangumiSvc).RegisterRoutes(v1)
	handler.NewNotificationHandler(notifSvc).RegisterRoutes(v1)
	handler.NewCalendarHandler(animeSvc, bangumiSvc).RegisterRoutes(v1)
	handler.NewBangumiHandler(animeSvc, bangumiSvc, bangumiAutoDL).RegisterRoutes(v1)
	handler.NewStreamRuleHandler(streamRuleSvc).RegisterRoutes(v1)
	handler.NewStreamHandler(streamRuleSvc, streamManager, dlSvc).RegisterRoutes(v1)
	handler.NewFileSystemHandler("/downloads").RegisterRoutes(v1)
	handler.NewDefaultRulesHandler(cfg).RegisterRoutes(v1)
	handler.NewIndexerHandler(settingSvc).RegisterRoutes(v1)
	handler.NewOrchestratorHandler(db, orch, settingSvc).RegisterRoutes(v1)

	// WebSocket 路由
	router.GET("/ws/:client_id", func(c *gin.Context) {
		ws.HandleWebSocket(wsHub, c)
	})

	// 健康检查
	router.GET("/healthcheck", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"scheduler": "running",
		})
	})

	// 静态文件 (前端) - 必须在所有 API 路由之后注册
	if _, err := os.Stat("../frontend/dist"); err == nil {
		router.NoRoute(func(c *gin.Context) {
			path := c.Request.URL.Path
			filePath := "../frontend/dist" + path
			if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
				c.File(filePath)
				return
			}
			c.File("../frontend/dist/index.html")
		})
		zap.L().Info("已配置前端静态文件服务")
	}

	// 10. 启动 HTTP 服务器
	srv := &http.Server{
		Addr:    ":8088",
		Handler: router,
	}

	go func() {
		zap.L().Info("服务器启动", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zap.L().Fatal("服务器启动失败", zap.Error(err))
		}
	}()

	// 11. 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	zap.L().Info("收到信号，正在关闭...", zap.String("signal", sig.String()))

	sched.Stop()
	streamManager.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		zap.L().Error("服务器关闭失败", zap.Error(err))
	}

	zap.L().Info("服务器已关闭")
}

// orchRetryAdapter 把 *orchestrator.Orchestrator.CheckAnime（接受 typed
// orchestrator.Preference）适配到 scheduler.RetryConductor 的 interface{} 形参，
// 避免 scheduler 包反向 import orchestrator 造成循环依赖。
type orchRetryAdapter struct {
	orch *orchestrator.Orchestrator
}

func (a *orchRetryAdapter) CheckAnime(ctx context.Context, anime *model.Anime, prefAny interface{}) {
	pref, ok := prefAny.(orchestrator.Preference)
	if !ok {
		// 兜底：让 orchestrator 自己 reload（性能不会差，5min 一次）
		// nil 也走 reload，毕竟我们没法在调用前看出"准确的 pref"是啥
		// 由 orchestrator 内部 LoadGlobal 解决
		zap.L().Debug("retry: 未提供 Preference，orchestrator 内部 reload")
		// CheckAnime 本身要 Preference 类型，无法传 nil；用 Defaults 兜底
		pref = orchestrator.Defaults()
	}
	a.orch.CheckAnime(ctx, anime, pref)
}

func initLogger(cfg *config.Config) {
	var zapLevel zapcore.Level
	switch cfg.LogLevel {
	case "DEBUG":
		zapLevel = zapcore.DebugLevel
	case "WARN":
		zapLevel = zapcore.WarnLevel
	case "ERROR":
		zapLevel = zapcore.ErrorLevel
	default:
		zapLevel = zapcore.InfoLevel
	}

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "time"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.AddSync(os.Stdout),
		zapLevel,
	)

	logger := zap.New(core, zap.AddCaller())
	zap.ReplaceGlobals(logger)
}

// applyDBConfigOverrides 把 Setting 表里的可覆盖字段写回 cfg，让后续的 HTTP client
// 构建都用上用户在 UI 里配置的值。目前只处理代理。
func applyDBConfigOverrides(db *gorm.DB, cfg *config.Config) {
	var items []model.Setting
	if err := db.Where("key IN ?", []string{"http_proxy"}).Find(&items).Error; err != nil {
		return
	}
	for _, it := range items {
		switch it.Key {
		case "http_proxy":
			if it.Value != "" {
				cfg.HTTPProxy = it.Value
				zap.L().Info("使用 DB 中配置的 HTTP 代理", zap.String("proxy", it.Value))
			}
		}
	}
}
