package embedded

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/RoaringBitmap/roaring"
	atorrent "github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
	"golang.org/x/time/rate"

	"github.com/anidog/anidog-go/internal/config"
	"github.com/anidog/anidog-go/internal/downloader"
)

const (
	engineVersion          = "anacrolix/torrent v1.61.0"
	manifestVersion        = 1
	maxMetainfoBytes int64 = 16 << 20
)

type manifest struct {
	Version int             `json:"version"`
	Tasks   []manifestEntry `json:"tasks"`
}

type manifestEntry struct {
	InfoHash string    `json:"info_hash"`
	URL      string    `json:"url"`
	SavePath string    `json:"save_path"`
	Paused   bool      `json:"paused"`
	AddedAt  time.Time `json:"added_at"`
}

type torrentEntry struct {
	manifestEntry
	torrent     *atorrent.Torrent
	forceStart  bool
	queuePaused bool
}

// Engine embeds the BitTorrent protocol implementation in the AniDog process.
// It deliberately owns no HTTP API or credentials.
type Engine struct {
	mu           sync.RWMutex
	client       *atorrent.Client
	entries      map[string]*torrentEntry
	stateDir     string
	manifestPath string
	defaultDir   string
	listenPort   int
	maxActive    int
	downloadRate *rate.Limiter
	uploadRate   *rate.Limiter
	httpClient   *http.Client
	proxy        *dynamicProxy
	closed       bool
}

type dynamicProxy struct {
	mu  sync.RWMutex
	url *url.URL
}

func (p *dynamicProxy) proxy(_ *http.Request) (*url.URL, error) {
	p.mu.RLock()
	configured := p.url
	p.mu.RUnlock()
	if configured != nil {
		return configured, nil
	}
	return nil, nil
}

func (p *dynamicProxy) set(raw string) error {
	var configured *url.URL
	if raw = strings.TrimSpace(raw); raw != "" {
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return fmt.Errorf("无效 HTTP 代理: %s", raw)
		}
		configured = parsed
	}
	p.mu.Lock()
	p.url = configured
	p.mu.Unlock()
	return nil
}

func New(cfg *config.Config) (*Engine, error) {
	stateDir := strings.TrimSpace(cfg.BTStateDir)
	if stateDir == "" {
		stateDir = "./data/torrent"
	}
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return nil, fmt.Errorf("创建 BT 状态目录: %w", err)
	}
	defaultDir := strings.TrimSpace(cfg.MediaRoot)
	if defaultDir == "" {
		defaultDir = "/downloads"
	}
	if err := os.MkdirAll(defaultDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建 BT 下载目录: %w", err)
	}

	downloadRate := rate.NewLimiter(rate.Inf, 1<<20)
	uploadRate := rate.NewLimiter(rate.Inf, 1<<20)
	clientCfg := atorrent.NewDefaultClientConfig()
	clientCfg.DataDir = defaultDir
	clientCfg.Seed = cfg.BTSeed
	clientCfg.DownloadRateLimiter = downloadRate
	clientCfg.UploadRateLimiter = uploadRate
	clientCfg.Slogger = slog.New(slog.NewTextHandler(io.Discard, nil))
	listenPort := cfg.BTListenPort
	if listenPort < 0 || listenPort > 65535 {
		return nil, fmt.Errorf("无效 BT 监听端口: %d", listenPort)
	}
	clientCfg.SetListenAddr(fmt.Sprintf(":%d", listenPort))

	dynamicHTTPProxy := &dynamicProxy{}
	if err := dynamicHTTPProxy.set(cfg.HTTPProxy); err != nil {
		return nil, err
	}
	httpTransport := http.DefaultTransport.(*http.Transport).Clone()
	httpTransport.Proxy = dynamicHTTPProxy.proxy
	clientCfg.HTTPProxy = dynamicHTTPProxy.proxy
	clientCfg.WebTransport = httpTransport

	client, err := atorrent.NewClient(clientCfg)
	if err != nil {
		return nil, fmt.Errorf("启动内嵌 BT 引擎: %w", err)
	}
	engine := &Engine{
		client:       client,
		entries:      make(map[string]*torrentEntry),
		stateDir:     stateDir,
		manifestPath: filepath.Join(stateDir, "tasks.json"),
		defaultDir:   defaultDir,
		listenPort:   client.LocalPort(),
		maxActive:    3,
		downloadRate: downloadRate,
		uploadRate:   uploadRate,
		httpClient:   &http.Client{Transport: httpTransport, Timeout: 30 * time.Second},
		proxy:        dynamicHTTPProxy,
	}
	if err := engine.restore(context.Background()); err != nil {
		_ = engine.Close()
		return nil, err
	}
	return engine, nil
}

func (e *Engine) Name() string { return "AniDog Embedded BT" }

func (e *Engine) AddTorrent(ctx context.Context, torrentURL, savePath string) (string, error) {
	return e.add(ctx, torrentURL, savePath, false, time.Now(), true)
}

func (e *Engine) add(
	ctx context.Context,
	torrentURL string,
	savePath string,
	paused bool,
	addedAt time.Time,
	persist bool,
) (string, error) {
	torrentURL = strings.TrimSpace(torrentURL)
	if torrentURL == "" {
		return "", errors.New("torrent URL 不能为空")
	}
	if savePath == "" {
		savePath = e.defaultDir
	}
	cleanSavePath, err := filepath.Abs(filepath.Clean(savePath))
	if err != nil {
		return "", fmt.Errorf("解析下载目录: %w", err)
	}
	if err := os.MkdirAll(cleanSavePath, 0o755); err != nil {
		return "", fmt.Errorf("创建下载目录: %w", err)
	}

	var spec *atorrent.TorrentSpec
	if strings.HasPrefix(strings.ToLower(torrentURL), "magnet:") {
		spec, err = atorrent.TorrentSpecFromMagnetUri(torrentURL)
	} else {
		spec, err = e.specFromURL(ctx, torrentURL)
	}
	if err != nil {
		return "", err
	}
	// Each task gets qBittorrent-compatible save-path semantics: torrent files
	// are rooted below the exact directory selected by AniDog.
	spec.Storage = storage.NewFile(cleanSavePath)
	spec.DisallowDataDownload = paused
	spec.DisallowDataUpload = paused
	t, _, err := e.client.AddTorrentSpec(spec)
	if err != nil {
		return "", fmt.Errorf("添加 BT 任务: %w", err)
	}
	hash := strings.ToUpper(t.InfoHash().HexString())
	if addedAt.IsZero() {
		addedAt = time.Now()
	}

	e.mu.Lock()
	if old, ok := e.entries[hash]; ok {
		old.URL = torrentURL
		old.SavePath = cleanSavePath
		old.Paused = paused
		old.AddedAt = addedAt
		old.torrent = t
	} else {
		e.entries[hash] = &torrentEntry{
			manifestEntry: manifestEntry{
				InfoHash: hash,
				URL:      torrentURL,
				SavePath: cleanSavePath,
				Paused:   paused,
				AddedAt:  addedAt,
			},
			torrent: t,
		}
	}
	if !paused {
		t.DownloadAll()
	}
	e.rebalanceLocked()
	if persist {
		err = e.persistLocked()
	}
	e.mu.Unlock()
	if err != nil {
		return "", err
	}
	return hash, nil
}

func (e *Engine) specFromURL(ctx context.Context, rawURL string) (*atorrent.TorrentSpec, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("无效 torrent URL: %w", err)
	}
	switch parsed.Scheme {
	case "http", "https":
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		resp, err := e.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("下载 torrent 元数据: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("下载 torrent 元数据返回 HTTP %d", resp.StatusCode)
		}
		mi, err := metainfo.Load(io.LimitReader(resp.Body, maxMetainfoBytes+1))
		if err != nil {
			return nil, fmt.Errorf("解析 torrent 元数据: %w", err)
		}
		return atorrent.TorrentSpecFromMetaInfoErr(mi)
	case "", "file":
		path := rawURL
		if parsed.Scheme == "file" {
			path = parsed.Path
		}
		mi, err := metainfo.LoadFromFile(path)
		if err != nil {
			return nil, fmt.Errorf("读取 torrent 文件: %w", err)
		}
		return atorrent.TorrentSpecFromMetaInfoErr(mi)
	default:
		return nil, fmt.Errorf("不支持的 torrent URL 协议: %s", parsed.Scheme)
	}
}

func (e *Engine) PauseTorrent(_ context.Context, torrentID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	entry, err := e.entryLocked(torrentID)
	if err != nil {
		return err
	}
	entry.Paused = true
	entry.forceStart = false
	entry.torrent.DisallowDataDownload()
	entry.torrent.DisallowDataUpload()
	e.rebalanceLocked()
	return e.persistLocked()
}

func (e *Engine) ResumeTorrent(_ context.Context, torrentID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	entry, err := e.entryLocked(torrentID)
	if err != nil {
		return err
	}
	entry.Paused = false
	entry.torrent.AllowDataUpload()
	e.rebalanceLocked()
	return e.persistLocked()
}

func (e *Engine) RemoveTorrent(_ context.Context, torrentID string, removeFiles bool) error {
	hash := strings.ToUpper(strings.TrimSpace(torrentID))
	e.mu.Lock()
	entry, err := e.entryLocked(hash)
	if err != nil {
		e.mu.Unlock()
		return err
	}
	contentPath := e.contentPathLocked(entry)
	savePath := entry.SavePath
	entry.torrent.Drop()
	delete(e.entries, hash)
	e.rebalanceLocked()
	persistErr := e.persistLocked()
	e.mu.Unlock()
	if persistErr != nil {
		return persistErr
	}
	if removeFiles {
		return removeTorrentData(contentPath, savePath)
	}
	return nil
}

func (e *Engine) GetTorrentInfo(ctx context.Context, torrentID string) (map[string]interface{}, error) {
	snapshots, err := e.ListTorrents(ctx)
	if err != nil {
		return nil, err
	}
	for _, snapshot := range snapshots {
		if strings.EqualFold(snapshot.ID, torrentID) {
			return snapshotMap(snapshot), nil
		}
	}
	return nil, fmt.Errorf("未找到种子: %s", torrentID)
}

func (e *Engine) ListTorrents(context.Context) ([]downloader.TorrentSnapshot, error) {
	e.mu.Lock()
	e.rebalanceLocked()
	e.mu.Unlock()

	e.mu.RLock()
	entries := make([]*torrentEntry, 0, len(e.entries))
	for _, entry := range e.entries {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].AddedAt.Before(entries[j].AddedAt) })
	out := make([]downloader.TorrentSnapshot, 0, len(entries))
	for _, entry := range entries {
		out = append(out, e.snapshotLocked(entry))
	}
	e.mu.RUnlock()

	return out, nil
}

func (e *Engine) snapshotLocked(entry *torrentEntry) downloader.TorrentSnapshot {
	t := entry.torrent
	stats := t.Stats()
	downloaded := t.BytesCompleted()
	total := t.Length()
	hasMetadata := t.Info() != nil
	progress := float64(0)
	if total > 0 {
		progress = float64(downloaded) / float64(total)
	}
	speed := int64(0)
	for _, peer := range t.PeerConns() {
		rateNow := peer.DownloadRate()
		if rateNow > 0 {
			speed += int64(rateNow)
		}
	}
	for _, peer := range t.WebseedPeerConns() {
		rateNow := peer.DownloadRate()
		if rateNow > 0 {
			speed += int64(rateNow)
		}
	}
	availability := torrentAvailability(t)
	state := "metaDL"
	switch {
	case entry.Paused:
		state = "pausedDL"
	case !hasMetadata:
		state = "metaDL"
	case t.Complete().Bool():
		state = "uploading"
	case entry.queuePaused:
		state = "queuedDL"
	case speed > 0:
		state = "downloading"
	default:
		state = "stalledDL"
	}
	wasted := stats.BytesReadData.Int64() - stats.BytesReadUsefulIntendedData.Int64()
	if wasted < 0 {
		wasted = 0
	}
	eta := 0
	if speed > 0 && total > downloaded {
		seconds := (total - downloaded) / speed
		if seconds > 0 && seconds < 8640000 {
			eta = int(seconds)
		}
	}
	return downloader.TorrentSnapshot{
		ID:             strings.ToUpper(t.InfoHash().HexString()),
		Name:           t.Name(),
		State:          state,
		HasMetadata:    hasMetadata,
		Size:           total,
		Downloaded:     downloaded,
		DownloadSpeed:  speed,
		Progress:       progress,
		Availability:   availability,
		ConnectedSeeds: stats.ConnectedSeeders,
		ETASeconds:     eta,
		TotalWasted:    wasted,
		ContentPath:    e.contentPathLocked(entry),
	}
}

func torrentAvailability(t *atorrent.Torrent) float64 {
	if t.Info() == nil || t.NumPieces() <= 0 {
		return -1
	}
	if t.Stats().ConnectedSeeders > 0 {
		return 1
	}
	union := roaring.New()
	for _, peer := range t.PeerConns() {
		if pieces := peer.PeerPieces(); pieces != nil {
			union.Or(pieces)
		}
	}
	return float64(union.GetCardinality()) / float64(t.NumPieces())
}

func (e *Engine) GetTotalWasted(_ context.Context, torrentID string) (int64, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	entry, err := e.entryLocked(torrentID)
	if err != nil {
		return 0, err
	}
	stats := entry.torrent.Stats()
	wasted := stats.BytesReadData.Int64() - stats.BytesReadUsefulIntendedData.Int64()
	if wasted < 0 {
		return 0, nil
	}
	return wasted, nil
}

func (e *Engine) SetForceStart(_ context.Context, torrentID string, enabled bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	entry, err := e.entryLocked(torrentID)
	if err != nil {
		return err
	}
	entry.forceStart = enabled
	e.rebalanceLocked()
	return nil
}

func (e *Engine) SetPolicy(_ context.Context, policy downloader.TorrentPolicy) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if policy.MaxActiveDownloads <= 0 {
		policy.MaxActiveDownloads = 3
	}
	e.maxActive = policy.MaxActiveDownloads
	setLimiter(e.downloadRate, policy.DownloadRate)
	setLimiter(e.uploadRate, policy.UploadRate)
	e.rebalanceLocked()
	return nil
}

func (e *Engine) SetHTTPProxy(_ context.Context, proxyURL string) error {
	return e.proxy.set(proxyURL)
}

func setLimiter(limiter *rate.Limiter, bytesPerSecond int64) {
	if bytesPerSecond <= 0 {
		limiter.SetLimit(rate.Inf)
		limiter.SetBurst(1 << 20)
		return
	}
	limiter.SetLimit(rate.Limit(bytesPerSecond))
	const maxBurst = int64(64 << 20)
	if bytesPerSecond > maxBurst {
		bytesPerSecond = maxBurst
	}
	burst := int(bytesPerSecond)
	if burst < 1<<20 {
		burst = 1 << 20
	}
	limiter.SetBurst(burst)
}

func (e *Engine) Health(context.Context) downloader.EngineHealth {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return downloader.EngineHealth{
		Online:       !e.closed && e.client != nil,
		Name:         e.Name(),
		Version:      engineVersion,
		ListenPort:   e.listenPort,
		TorrentCount: len(e.entries),
	}
}

func (e *Engine) Close() error {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil
	}
	e.closed = true
	persistErr := e.persistLocked()
	client := e.client
	e.mu.Unlock()
	if client != nil {
		if errs := client.Close(); len(errs) > 0 && persistErr == nil {
			persistErr = errors.Join(errs...)
		}
	}
	return persistErr
}

func (e *Engine) restore(ctx context.Context) error {
	raw, err := os.ReadFile(e.manifestPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取 BT 任务状态: %w", err)
	}
	var saved manifest
	if err := json.Unmarshal(raw, &saved); err != nil {
		return fmt.Errorf("解析 BT 任务状态: %w", err)
	}
	if saved.Version != manifestVersion {
		return fmt.Errorf("不支持的 BT 状态版本: %d", saved.Version)
	}
	var restoreErrors []error
	for _, item := range saved.Tasks {
		if _, err := e.add(ctx, item.URL, item.SavePath, item.Paused, item.AddedAt, false); err != nil {
			restoreErrors = append(restoreErrors, fmt.Errorf("恢复 %s: %w", item.InfoHash, err))
		}
	}
	if len(restoreErrors) > 0 {
		return errors.Join(restoreErrors...)
	}
	return nil
}

func (e *Engine) persistLocked() error {
	tasks := make([]manifestEntry, 0, len(e.entries))
	for _, entry := range e.entries {
		tasks = append(tasks, entry.manifestEntry)
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].AddedAt.Before(tasks[j].AddedAt) })
	raw, err := json.MarshalIndent(manifest{Version: manifestVersion, Tasks: tasks}, "", "  ")
	if err != nil {
		return err
	}
	tmp := e.manifestPath + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("写入 BT 任务状态: %w", err)
	}
	if err := os.Rename(tmp, e.manifestPath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("提交 BT 任务状态: %w", err)
	}
	return nil
}

func (e *Engine) rebalanceLocked() {
	entries := make([]*torrentEntry, 0, len(e.entries))
	for _, entry := range e.entries {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].AddedAt.Before(entries[j].AddedAt) })
	active := 0
	for _, entry := range entries {
		if entry.Paused {
			entry.queuePaused = false
			entry.torrent.DisallowDataDownload()
			entry.torrent.DisallowDataUpload()
			continue
		}
		entry.torrent.AllowDataUpload()
		if entry.torrent.Complete().Bool() {
			entry.queuePaused = false
			entry.torrent.DisallowDataDownload()
			continue
		}
		if entry.forceStart || active < e.maxActive {
			entry.queuePaused = false
			entry.torrent.AllowDataDownload()
			entry.torrent.DownloadAll()
			active++
		} else {
			entry.queuePaused = true
			entry.torrent.DisallowDataDownload()
		}
	}
}

func (e *Engine) entryLocked(torrentID string) (*torrentEntry, error) {
	entry, ok := e.entries[strings.ToUpper(strings.TrimSpace(torrentID))]
	if !ok {
		return nil, fmt.Errorf("未找到种子: %s", torrentID)
	}
	return entry, nil
}

func (e *Engine) contentPathLocked(entry *torrentEntry) string {
	info := entry.torrent.Info()
	if info == nil {
		return entry.SavePath
	}
	name, err := storage.ToSafeFilePath(info.BestName())
	if err != nil || name == "" || name == "." {
		return entry.SavePath
	}
	return filepath.Join(entry.SavePath, name)
}

func removeTorrentData(contentPath, savePath string) error {
	contentPath = filepath.Clean(strings.TrimSpace(contentPath))
	savePath = filepath.Clean(strings.TrimSpace(savePath))
	if contentPath == "" || savePath == "" || contentPath == "." || savePath == "." {
		return errors.New("拒绝删除空 BT 路径")
	}
	rel, err := filepath.Rel(savePath, contentPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("拒绝删除下载目录之外的路径: %s", contentPath)
	}
	if contentPath == savePath && !strings.Contains(savePath, ".anidog-race") {
		return fmt.Errorf("元数据未就绪，拒绝删除非隔离下载根目录: %s", savePath)
	}
	if err := os.RemoveAll(contentPath); err != nil {
		return err
	}
	// Single-file torrents use a sibling .part file while incomplete.
	if contentPath != savePath {
		_ = os.RemoveAll(contentPath + ".part")
	}
	return nil
}

func snapshotMap(snapshot downloader.TorrentSnapshot) map[string]interface{} {
	return map[string]interface{}{
		"hash":         snapshot.ID,
		"name":         snapshot.Name,
		"state":        snapshot.State,
		"has_metadata": snapshot.HasMetadata,
		"size":         float64(snapshot.Size),
		"downloaded":   float64(snapshot.Downloaded),
		"dlspeed":      float64(snapshot.DownloadSpeed),
		"progress":     snapshot.Progress,
		"availability": snapshot.Availability,
		"num_seeds":    float64(snapshot.ConnectedSeeds),
		"eta":          float64(snapshot.ETASeconds),
		"total_wasted": float64(snapshot.TotalWasted),
		"content_path": snapshot.ContentPath,
	}
}

var _ downloader.TorrentEngine = (*Engine)(nil)
