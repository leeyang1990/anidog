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
	zap.L().Info(fmt.Sprintf("启动 %s v%s...", cfg.ProjectName, cfg.ProjectVersion))

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

	// 下载根目录：BT/RSS 都按 <mediaRoot>/<番剧名 (年份)>/Season NN 组织
	mediaRoot := cfg.MediaRoot
	if mediaRoot == "" {
		mediaRoot = "/downloads"
	}
	rssEngine.SetMediaRoot(mediaRoot)

	// 5d2. Orchestrator：多源剧集填坑调度器（替代旧的 bangumi.CheckAllSubscribed）
	orch := orchestrator.New(db, dlSvc, streamManager, settingSvc, nil, mediaRoot)

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
	sched.Register(scheduler.NewRSSRefreshJob(rssEngine), rssInterval, true)
	renameInterval := time.Duration(cfg.RenameInterval) * time.Second
	if renameInterval <= 0 {
		renameInterval = 300 * time.Second
	}
	sched.Register(scheduler.NewRenameJob(), renameInterval, false)
	// 追番更新检查（每 30 分钟）
	// 番剧更新检查：改为由 Orchestrator 驱动多源下载
	sched.Register(scheduler.NewBangumiCheckJob(orch), 30*time.Minute, false)
	// 源健康检测（每 3 分钟）
	sourceHealthSvc := bangumisvc.NewSourceHealthService(db)
	sched.Register(scheduler.NewSourceHealthJob(sourceHealthSvc), 3*time.Minute, true)

	// qBit 进度同步：每 15 秒把 qBit 种子的 size/progress 写回 DB
	if qbitClient != nil {
		qbitSync := dlservice.NewQBitSyncer(db, cfg)
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
	handler.NewSettingsHandler(settingSvc).RegisterRoutes(v1)
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
