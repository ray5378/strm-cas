package main

import (
	"context"
	"crypto/sha1"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
	"strm-cas/cas"
)

//go:embed all:web
var webFS embed.FS

type taskSettings struct {
	Concurrency      int   `json:"concurrency"`
	TotalRateLimit   int64 `json:"total_rate_limit_bytes"`
	TotalRateLimitMB int   `json:"total_rate_limit_mb"`
	MaxFileSizeGB    int   `json:"max_file_size_gb"`
	MaxFileSizeBytes int64 `json:"max_file_size_bytes"`
}

type wsClient struct {
	ch         chan []byte
	filters    cas.QueryOptions
	detailPath string
}

type app struct {
	cfg              cas.STRMProcessOptions
	runtime          *cas.RuntimeStore
	db               *bolt.DB
	mu               sync.Mutex
	cancelMu         sync.Mutex
	cancelRun        context.CancelFunc
	gracefulStopMu   sync.Mutex
	gracefulStopFlag bool
	settingsMu        sync.RWMutex
	settings          taskSettings
	settingsPath      string
	limiterMu         sync.RWMutex
	activeLimiter     *cas.RateLimiter
	statsMu           sync.RWMutex
	statsCache        cas.Stats
	statsCacheValid   bool
	recordsIndexMu    sync.RWMutex
	recordsIndexCache *cas.RecordsIndex
	wsClientsMu       sync.RWMutex
	wsClients         map[*wsClient]struct{}
	runtimePushMu     sync.Mutex
	runtimePushTimer  *time.Timer
	runtimePushDelay  time.Duration
}

func main() {
	listen := envOr("STRM_CAS_LISTEN", ":18457")
	concurrency := envOrInt("STRM_CAS_CONCURRENCY", 2)
	rateMB := envOrInt("STRM_CAS_TOTAL_RATE_MB", 0)
	cfg := cas.STRMProcessOptions{
		STRMRoot:        envOr("STRM_CAS_STRM_ROOT", "/data/strm"),
		CacheDir:        envOr("STRM_CAS_CACHE_DIR", "/data/cache"),
		DownloadDir:     envOr("STRM_CAS_DOWNLOAD_DIR", "/data/download"),
		Mode:            cas.Mode189PC,
		UserAgent:       envOr("STRM_CAS_USER_AGENT", "strm-cas-api/1.0"),
		SkipExistingCAS: true,
		LogPath:         envOr("STRM_CAS_LOG_PATH", "/data/strm-cas-summary.json"),
		DBPath:          envOr("STRM_CAS_DB_PATH", "/data/strm-cas.db"),
		Concurrency:     concurrency,
		TotalRateLimit:  int64(rateMB) * 1024 * 1024,
	}
	if timeoutStr := os.Getenv("STRM_CAS_HTTP_TIMEOUT"); timeoutStr != "" {
		if d, err := time.ParseDuration(timeoutStr); err == nil {
			cfg.HTTPTimeout = d
		}
	}

	db, err := cas.OpenStateDB(cfg.DBPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	settingsPath := envOr("STRM_CAS_SETTINGS_PATH", "/data/strm-cas-settings.json")
	initialSettings := taskSettings{Concurrency: concurrency, TotalRateLimit: int64(rateMB) * 1024 * 1024, TotalRateLimitMB: rateMB, MaxFileSizeGB: 0, MaxFileSizeBytes: 0}
	if saved, err := loadTaskSettings(settingsPath, initialSettings); err == nil {
		initialSettings = saved
		cfg.Concurrency = saved.Concurrency
		cfg.TotalRateLimit = saved.TotalRateLimit
		cfg.MaxFileSizeBytes = saved.MaxFileSizeBytes
	} else {
		log.Printf("load settings skipped: %v", err)
	}

	app := &app{
		cfg:              cfg,
		runtime:          cas.NewRuntimeStore(1000),
		db:               db,
		settings:         initialSettings,
		settingsPath:     settingsPath,
		wsClients:        make(map[*wsClient]struct{}),
		runtimePushDelay: 250 * time.Millisecond,
	}
	webSub, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/overview", app.handleOverview)
	mux.HandleFunc("/api/settings", app.handleSettings)
	mux.HandleFunc("/api/records", app.handleRecords)
	mux.HandleFunc("/api/records/detail", app.handleRecordDetail)
	mux.HandleFunc("/api/runtime", app.handleRuntime)
	mux.HandleFunc("/api/runtime/ws", app.handleRuntimeWS)
	mux.HandleFunc("/api/scan/refresh", app.handleScanRefresh)
	mux.HandleFunc("/api/db/reconcile", app.handleDBReconcile)
	mux.HandleFunc("/api/tasks/start", app.handleTasksStart)
	mux.HandleFunc("/api/tasks/start-selected", app.handleStartSelected)
	mux.HandleFunc("/api/tasks/stop", app.handleTasksStop)
	mux.HandleFunc("/api/tasks/stop-after-current", app.handleTasksStopAfterCurrent)
	mux.HandleFunc("/api/tasks/retry", app.handleTaskRetry)
	mux.HandleFunc("/api/tasks/retry-failed", app.handleRetryFailed)
	mux.HandleFunc("/api/tasks/retry-selected", app.handleRetrySelected)
	mux.HandleFunc("/api/db/clear", app.handleDBClear)
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		data, err := webFS.ReadFile("web/favicon.ico")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/x-icon")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/web/", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	})
	mux.Handle("/web/", http.StripPrefix("/web/", http.FileServer(http.FS(webSub))))

	go app.prewarmCaches()

	log.Printf("strm-cas api listening on %s", listen)
	log.Fatal(http.ListenAndServe(listen, withCORS(mux)))
}

func (a *app) prewarmCaches() {
	start := time.Now()
	if _, err := a.getRecordsIndex(); err != nil {
		log.Printf("prewarm records index skipped: %v", err)
		return
	}
	if _, err := a.getStats(); err != nil {
		log.Printf("prewarm stats skipped: %v", err)
		return
	}
	log.Printf("prewarm caches ready in %s", time.Since(start).Round(time.Millisecond))
}

func (a *app) getSettings() taskSettings {
	a.settingsMu.RLock()
	defer a.settingsMu.RUnlock()
	return a.settings
}

func (a *app) setGracefulStop(v bool) {
	a.gracefulStopMu.Lock()
	defer a.gracefulStopMu.Unlock()
	a.gracefulStopFlag = v
	a.runtime.SetGracefulStopping(v)
}

func (a *app) isGracefulStop() bool {
	a.gracefulStopMu.Lock()
	defer a.gracefulStopMu.Unlock()
	return a.gracefulStopFlag
}

func (a *app) invalidateStatsCache() {
	a.statsMu.Lock()
	defer a.statsMu.Unlock()
	a.statsCacheValid = false
}

func (a *app) invalidateRecordsIndexCache() {
	a.recordsIndexMu.Lock()
	defer a.recordsIndexMu.Unlock()
	a.recordsIndexCache = nil
}

func (a *app) invalidateStateCaches() {
	a.invalidateStatsCache()
	a.invalidateRecordsIndexCache()
}

func (a *app) getStats() (cas.Stats, error) {
	a.statsMu.RLock()
	if a.statsCacheValid {
		stats := a.statsCache
		a.statsMu.RUnlock()
		return stats, nil
	}
	a.statsMu.RUnlock()

	idx, err := a.getRecordsIndex()
	if err != nil {
		return cas.Stats{}, err
	}
	stats := idx.Stats()

	a.statsMu.Lock()
	a.statsCache = stats
	a.statsCacheValid = true
	a.statsMu.Unlock()
	return stats, nil
}

func (a *app) getRecordsIndex() (*cas.RecordsIndex, error) {
	a.recordsIndexMu.RLock()
	if a.recordsIndexCache != nil {
		idx := a.recordsIndexCache
		a.recordsIndexMu.RUnlock()
		return idx, nil
	}
	a.recordsIndexMu.RUnlock()

	idx, err := cas.BuildRecordsIndexFromDB(a.db)
	if err != nil {
		return nil, err
	}

	a.recordsIndexMu.Lock()
	a.recordsIndexCache = idx
	a.recordsIndexMu.Unlock()
	return idx, nil
}

func (a *app) handleOverview(w http.ResponseWriter, r *http.Request) {
	stats, err := a.getStats()
	if err != nil {
		writeErr(w, err, 500)
		return
	}
	writeJSON(w, map[string]any{"stats": stats, "runtime": a.runtime.Snapshot(), "settings": a.getSettings()})
}

func (a *app) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, a.getSettings())
	case http.MethodPost:
		var req struct {
			Concurrency      int `json:"concurrency"`
			TotalRateLimitMB int `json:"total_rate_limit_mb"`
			MaxFileSizeGB    int `json:"max_file_size_gb"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, fmt.Errorf("invalid body"), 400)
			return
		}
		if req.Concurrency <= 0 {
			req.Concurrency = 1
		}
		if req.TotalRateLimitMB < 0 {
			req.TotalRateLimitMB = 0
		}
		if req.MaxFileSizeGB < 0 {
			req.MaxFileSizeGB = 0
		}
		newSettings := taskSettings{
			Concurrency:      req.Concurrency,
			TotalRateLimitMB: req.TotalRateLimitMB,
			TotalRateLimit:   int64(req.TotalRateLimitMB) * 1024 * 1024,
			MaxFileSizeGB:    req.MaxFileSizeGB,
			MaxFileSizeBytes: int64(req.MaxFileSizeGB) * 1024 * 1024 * 1024,
		}
		if err := saveTaskSettings(a.settingsPath, newSettings); err != nil {
			writeErr(w, err, 500)
			return
		}
		a.settingsMu.Lock()
		a.settings = newSettings
		a.cfg.Concurrency = req.Concurrency
		a.cfg.TotalRateLimit = int64(req.TotalRateLimitMB) * 1024 * 1024
		a.cfg.MaxFileSizeBytes = int64(req.MaxFileSizeGB) * 1024 * 1024 * 1024
		a.settingsMu.Unlock()
		a.limiterMu.RLock()
		limiter := a.activeLimiter
		a.limiterMu.RUnlock()
		if limiter != nil {
			limiter.SetBytesPerSec(newSettings.TotalRateLimit)
		}
		writeJSON(w, a.getSettings())
	default:
		writeErr(w, fmt.Errorf("method not allowed"), 405)
	}
}

func (a *app) handleRecords(w http.ResponseWriter, r *http.Request) {
	page, size := parsePage(r), parsePageSize(r)
	idx, err := a.getRecordsIndex()
	if err != nil {
		writeErr(w, err, 500)
		return
	}
	opts := cas.QueryOptions{Status: r.URL.Query().Get("status"), Search: r.URL.Query().Get("search"), Page: page, PageSize: size}
	paths, total := idx.QueryPagePaths(opts)
	items, err := cas.GetRecordsByPaths(a.db, paths)
	if err != nil {
		writeErr(w, err, 500)
		return
	}
	writeJSON(w, cas.QueryResult{Total: total, Items: items})
}

func (a *app) handleRecordDetail(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	if p == "" {
		writeErr(w, fmt.Errorf("missing path"), 400)
		return
	}
	rec, err := cas.GetRecord(a.db, p)
	if err != nil {
		writeErr(w, err, 500)
		return
	}
	if rec == nil {
		writeErr(w, fmt.Errorf("not found"), 404)
		return
	}
	writeJSON(w, rec)
}

func (a *app) handleRuntime(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, a.runtime.Snapshot())
}

func (a *app) handleRuntimeWS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, fmt.Errorf("method not allowed"), 405)
		return
	}
	if !headerContainsToken(r.Header, "Connection", "upgrade") || !headerEqualsFold(r.Header, "Upgrade", "websocket") {
		writeErr(w, fmt.Errorf("websocket upgrade required"), 400)
		return
	}
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		writeErr(w, fmt.Errorf("missing websocket key"), 400)
		return
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		writeErr(w, fmt.Errorf("hijacking not supported"), 500)
		return
	}
	conn, rw, err := hj.Hijack()
	if err != nil {
		writeErr(w, err, 500)
		return
	}
	accept := websocketAcceptKey(key)
	if _, err := rw.WriteString("HTTP/1.1 101 Switching Protocols\r\n"); err != nil { _ = conn.Close(); return }
	if _, err := rw.WriteString("Upgrade: websocket\r\n"); err != nil { _ = conn.Close(); return }
	if _, err := rw.WriteString("Connection: Upgrade\r\n"); err != nil { _ = conn.Close(); return }
	if _, err := rw.WriteString("Sec-WebSocket-Accept: " + accept + "\r\n\r\n"); err != nil { _ = conn.Close(); return }
	if err := rw.Flush(); err != nil { _ = conn.Close(); return }

	client := &wsClient{ch: make(chan []byte, 16), filters: cas.QueryOptions{Page: 1, PageSize: 10}}
	a.wsClientsMu.Lock()
	a.wsClients[client] = struct{}{}
	a.wsClientsMu.Unlock()
	defer func() {
		a.wsClientsMu.Lock()
		delete(a.wsClients, client)
		a.wsClientsMu.Unlock()
		close(client.ch)
		_ = conn.Close()
	}()

	a.pushClientOverview(client)
	go a.pushClientSnapshot(client)
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
	go func() {
		defer func() { _ = conn.Close() }()
		for {
			opcode, payload, err := readWebSocketFrame(conn)
			if err != nil {
				return
			}
			_ = conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
			if opcode == 0x8 {
				return
			}
			if opcode != 0x1 {
				continue
			}
			var req struct {
				Type       string `json:"type"`
				Status     string `json:"status"`
				Search     string `json:"search"`
				Page       int    `json:"page"`
				PageSize   int    `json:"page_size"`
				DetailPath string `json:"detail_path"`
			}
			if err := json.Unmarshal(payload, &req); err != nil {
				continue
			}
			if req.Type == "subscribe" {
				a.wsClientsMu.Lock()
				client.filters = cas.QueryOptions{Status: req.Status, Search: req.Search, Page: req.Page, PageSize: req.PageSize}
				if client.filters.Page <= 0 { client.filters.Page = 1 }
				if client.filters.PageSize <= 0 { client.filters.PageSize = 10 }
				client.detailPath = req.DetailPath
				a.wsClientsMu.Unlock()
				a.pushClientOverview(client)
				go a.pushClientSnapshot(client)
			}
		}
	}()

	for msg := range client.ch {
		if err := writeWebSocketTextFrame(conn, msg); err != nil {
			return
		}
	}
}

func (a *app) handleScanRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, fmt.Errorf("method not allowed"), 405)
		return
	}
	jobs, err := a.currentJobs(true)
	if err != nil {
		writeErr(w, err, 500)
		return
	}
	a.invalidateStateCaches()
	stats, err := a.getStats()
	if err != nil {
		writeErr(w, err, 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "stats": stats, "total": len(jobs)})
}

func (a *app) handleDBReconcile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, fmt.Errorf("method not allowed"), 405)
		return
	}
	if a.runtime.Snapshot().Running {
		writeErr(w, fmt.Errorf("task is running, cannot reconcile database"), 409)
		return
	}
	summary, err := cas.ReconcileState(a.db, a.cfg.STRMRoot, a.cfg.DownloadDir)
	if err != nil {
		writeErr(w, err, 500)
		return
	}
	a.invalidateStateCaches()
	writeJSON(w, summary)
}

func writeStartSummary(w http.ResponseWriter, requested, matched, started int) {
	writeJSON(w, map[string]any{"ok": true, "requested": requested, "matched": matched, "started": started, "skipped": requested - started})
}

func (a *app) handleTasksStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, fmt.Errorf("method not allowed"), 405)
		return
	}
	var req struct {
		Mode   string `json:"mode"`
		Status string `json:"status"`
		Search string `json:"search"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	jobs, err := a.currentJobs(false)
	if err != nil {
		writeErr(w, err, 500)
		return
	}
	idx, err := a.getRecordsIndex()
	if err != nil {
		writeErr(w, err, 500)
		return
	}
	paths := idx.QueryPaths(cas.QueryOptions{Status: req.Status, Search: req.Search})
	byPath := make(map[string]cas.STRMJob, len(jobs))
	for _, job := range jobs {
		byPath[job.STRMPath] = job
	}
	statuses, err := cas.GetRecordStatusesByPaths(a.db, paths)
	if err != nil {
		writeErr(w, err, 500)
		return
	}
	filtered := make([]cas.STRMJob, 0)
	for _, path := range paths {
		job, ok := byPath[path]
		if !ok {
			continue
		}
		status := statuses[path]
		if status == "" {
			status = "pending"
		}
		switch req.Mode {
		case "failed":
			if status == "failed" {
				filtered = append(filtered, job)
			}
		case "current_filter":
			filtered = append(filtered, job)
		default:
			if status == "pending" {
				filtered = append(filtered, job)
			}
		}
	}
	requested, matched := len(paths), len(filtered)
	if matched == 0 {
		writeStartSummary(w, requested, matched, 0)
		return
	}
	if err := a.startJobs(filtered); err != nil {
		writeErr(w, err, 409)
		return
	}
	writeStartSummary(w, requested, matched, matched)
}

func (a *app) handleStartSelected(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, fmt.Errorf("method not allowed"), 405)
		return
	}
	var req struct {
		Paths []string `json:"paths"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Paths) == 0 {
		writeErr(w, fmt.Errorf("missing paths"), 400)
		return
	}
	jobs, err := a.currentJobs(false)
	if err != nil {
		writeErr(w, err, 500)
		return
	}
	selected := a.matchJobsByPaths(jobs, req.Paths, false)
	requested, matched := len(req.Paths), len(selected)
	if matched == 0 {
		writeStartSummary(w, requested, matched, 0)
		return
	}
	if err := a.startJobs(selected); err != nil {
		writeErr(w, err, 409)
		return
	}
	writeStartSummary(w, requested, matched, matched)
}

func (a *app) handleTasksStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, fmt.Errorf("method not allowed"), 405)
		return
	}
	a.cancelMu.Lock()
	cancel := a.cancelRun
	a.cancelMu.Unlock()
	if cancel == nil {
		writeJSON(w, map[string]any{"ok": true, "stopped": false})
		return
	}
	a.setGracefulStop(false)
	cancel()
	writeJSON(w, map[string]any{"ok": true, "stopped": true})
}

func (a *app) handleTasksStopAfterCurrent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, fmt.Errorf("method not allowed"), 405)
		return
	}
	if !a.runtime.Snapshot().Running {
		writeJSON(w, map[string]any{"ok": true, "graceful_stopping": false})
		return
	}
	a.setGracefulStop(true)
	writeJSON(w, map[string]any{"ok": true, "graceful_stopping": true})
}

func (a *app) handleTaskRetry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, fmt.Errorf("method not allowed"), 405)
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		writeErr(w, fmt.Errorf("missing path"), 400)
		return
	}
	jobs, err := a.currentJobs(false)
	if err != nil {
		writeErr(w, err, 500)
		return
	}
	selected := make([]cas.STRMJob, 0, 1)
	for _, job := range jobs {
		if job.STRMPath == req.Path {
			selected = append(selected, job)
			break
		}
	}
	if len(selected) == 0 {
		writeErr(w, fmt.Errorf("task not found"), 404)
		return
	}
	if err := a.startJobs(selected); err != nil {
		writeErr(w, err, 409)
		return
	}
	writeStartSummary(w, 1, 1, 1)
}

func (a *app) handleRetryFailed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, fmt.Errorf("method not allowed"), 405)
		return
	}
	jobs, err := a.currentJobs(false)
	if err != nil {
		writeErr(w, err, 500)
		return
	}
	idx, err := a.getRecordsIndex()
	if err != nil {
		writeErr(w, err, 500)
		return
	}
	paths := idx.QueryPaths(cas.QueryOptions{Status: "failed"})
	byPath := make(map[string]cas.STRMJob, len(jobs))
	for _, job := range jobs {
		byPath[job.STRMPath] = job
	}
	filtered := make([]cas.STRMJob, 0)
	for _, path := range paths {
		if job, ok := byPath[path]; ok {
			filtered = append(filtered, job)
		}
	}
	requested, matched := len(paths), len(filtered)
	if matched == 0 {
		writeStartSummary(w, requested, matched, 0)
		return
	}
	if err := a.startJobs(filtered); err != nil {
		writeErr(w, err, 409)
		return
	}
	writeStartSummary(w, requested, matched, matched)
}

func (a *app) handleRetrySelected(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, fmt.Errorf("method not allowed"), 405)
		return
	}
	var req struct {
		Status string   `json:"status"`
		Search string   `json:"search"`
		Paths  []string `json:"paths"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	jobs, err := a.currentJobs(false)
	if err != nil {
		writeErr(w, err, 500)
		return
	}
	requested := len(req.Paths)
	filtered := make([]cas.STRMJob, 0)
	if len(req.Paths) > 0 {
		filtered = a.matchJobsByPaths(jobs, req.Paths, true)
	} else {
		idx, err := a.getRecordsIndex()
		if err != nil {
			writeErr(w, err, 500)
			return
		}
		paths := idx.QueryPaths(cas.QueryOptions{Status: req.Status, Search: req.Search})
		requested = len(paths)
		byPath := make(map[string]cas.STRMJob, len(jobs))
		for _, job := range jobs {
			byPath[job.STRMPath] = job
		}
		statuses, err := cas.GetRecordStatusesByPaths(a.db, paths)
		if err != nil {
			writeErr(w, err, 500)
			return
		}
		for _, path := range paths {
			if job, ok := byPath[path]; ok {
				if statuses[path] == "failed" {
					filtered = append(filtered, job)
				}
			}
		}
	}
	matched := len(filtered)
	if matched == 0 {
		writeStartSummary(w, requested, matched, 0)
		return
	}
	if err := a.startJobs(filtered); err != nil {
		writeErr(w, err, 409)
		return
	}
	writeStartSummary(w, requested, matched, matched)
}

func (a *app) handleDBClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, fmt.Errorf("method not allowed"), 405)
		return
	}
	if a.runtime.Snapshot().Running {
		writeErr(w, fmt.Errorf("task is running, cannot clear database"), 409)
		return
	}
	if err := cas.ClearStateDBHandle(a.db); err != nil {
		writeErr(w, err, 500)
		return
	}
	a.invalidateStateCaches()
	a.runtime.Reset()
	writeJSON(w, map[string]any{"ok": true})
}

func (a *app) matchJobsByPaths(jobs []cas.STRMJob, paths []string, onlyFailed bool) []cas.STRMJob {
	set := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		set[path] = struct{}{}
	}
	filtered := make([]cas.STRMJob, 0, len(paths))
	for _, job := range jobs {
		if _, ok := set[job.STRMPath]; !ok {
			continue
		}
		if onlyFailed {
			rec, _ := cas.GetRecord(a.db, job.STRMPath)
			if rec == nil || rec.Status != "failed" {
				continue
			}
		}
		filtered = append(filtered, job)
	}
	return filtered
}

func (a *app) currentJobs(sync bool) ([]cas.STRMJob, error) {
	jobs, err := cas.DiscoverSTRMJobs(a.cfg.STRMRoot)
	if err != nil {
		return nil, err
	}
	if sync {
		if err := cas.SyncJobsToState(a.db, jobs); err != nil {
			return nil, err
		}
		a.invalidateStateCaches()
	}
	return jobs, nil
}

func (a *app) startJobs(jobs []cas.STRMJob) error {
	a.mu.Lock()
	if a.runtime.Snapshot().Running {
		a.mu.Unlock()
		return fmt.Errorf("task already running")
	}
	a.runtime.MarkStarted()
	a.flushRuntimeSnapshot()
	a.setGracefulStop(false)
	settings := a.getSettings()
	cfg := a.cfg
	cfg.Concurrency = settings.Concurrency
	cfg.TotalRateLimit = settings.TotalRateLimit
	cfg.MaxFileSizeBytes = settings.MaxFileSizeBytes
	ctx, cancel := context.WithCancel(context.Background())
	cfg.Context = ctx
	cfg.OnProgress = func(p cas.ProgressInfo) {
		a.runtime.SetCurrent(p)
		a.pushRuntimeSnapshot()
	}
	cfg.OnResult = func(res cas.STRMProcessResult) {
		a.runtime.AddCompleted(res)
		_ = cas.UpdateResult(a.db, res)
		a.invalidateStateCaches()
		a.flushRuntimeSnapshot()
	}
	a.cancelMu.Lock()
	a.cancelRun = cancel
	a.cancelMu.Unlock()
	go func(selected []cas.STRMJob) {
		defer func() { a.runtime.MarkFinished(); a.flushRuntimeSnapshot() }()
		defer a.setGracefulStop(false)
		defer a.mu.Unlock()
		defer func() { a.cancelMu.Lock(); a.cancelRun = nil; a.cancelMu.Unlock() }()
		defer func() { a.limiterMu.Lock(); a.activeLimiter = nil; a.limiterMu.Unlock() }()
		client := &http.Client{Timeout: cfg.HTTPTimeout}
		limiter := cas.NewSharedRateLimiter(cfg.TotalRateLimit)
		a.limiterMu.Lock()
		a.activeLimiter = limiter
		a.limiterMu.Unlock()
		jobCh := make(chan cas.STRMJob)
		var wg sync.WaitGroup
		workerCount := cfg.Concurrency
		if workerCount <= 0 {
			workerCount = 1
		}
		if workerCount > len(selected) && len(selected) > 0 {
			workerCount = len(selected)
		}
		for i := 0; i < workerCount; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for job := range jobCh {
					jobCfg := cfg
					jobCfg.MaxFileSizeBytes = a.getSettings().MaxFileSizeBytes
					res, err := cas.ProcessSingleSTRMWithContext(ctx, client, limiter, jobCfg, job)
					if err != nil {
						a.runtime.RemoveActive(job.STRMPath)
						a.flushRuntimeSnapshot()
						status := "failed"
						if ctx.Err() != nil {
							status = "skipped"
						}
						if job.ParseError != "" {
							status = "exception"
						}
						failed := cas.STRMProcessResult{Job: job, Status: status, Message: err.Error()}
						a.runtime.AddCompleted(failed)
						_ = cas.UpdateResult(a.db, failed)
						a.invalidateStateCaches()
						continue
					}
					if res != nil {
						a.runtime.AddCompleted(*res)
						_ = cas.UpdateResult(a.db, *res)
						a.invalidateStateCaches()
					}
				}
			}()
		}
		stopDispatch := false
		for _, job := range selected {
			if a.isGracefulStop() {
				stopDispatch = true
				break
			}
			for {
				if a.isGracefulStop() {
					stopDispatch = true
					break
				}
				select {
				case <-ctx.Done():
					close(jobCh)
					wg.Wait()
					return
				case jobCh <- job:
					goto nextJob
				case <-time.After(100 * time.Millisecond):
				}
			}
			if stopDispatch {
				break
			}
		nextJob:
		}
		close(jobCh)
		wg.Wait()
	}(append([]cas.STRMJob(nil), jobs...))
	return nil
}

func parsePage(r *http.Request) int {
	v, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if v <= 0 {
		v = 1
	}
	return v
}
func parsePageSize(r *http.Request) int {
	v, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if v <= 0 {
		v = 20
	}
	if v > 200 {
		v = 200
	}
	return v
}
func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
func envOrInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func loadTaskSettings(path string, fallback taskSettings) (taskSettings, error) {
	if path == "" {
		return fallback, nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fallback, nil
		}
		return fallback, err
	}
	var s taskSettings
	if err := json.Unmarshal(body, &s); err != nil {
		return fallback, err
	}
	if s.Concurrency <= 0 {
		s.Concurrency = fallback.Concurrency
	}
	if s.TotalRateLimitMB < 0 {
		s.TotalRateLimitMB = 0
	}
	if s.MaxFileSizeGB < 0 {
		s.MaxFileSizeGB = 0
	}
	s.TotalRateLimit = int64(s.TotalRateLimitMB) * 1024 * 1024
	s.MaxFileSizeBytes = int64(s.MaxFileSizeGB) * 1024 * 1024 * 1024
	return s, nil
}

func saveTaskSettings(path string, s taskSettings) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func (a *app) buildOverviewPayload() ([]byte, error) {
	stats, err := a.getStats()
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"type": "overview",
		"overview": map[string]any{"stats": stats, "runtime": a.runtime.Snapshot(), "settings": a.getSettings()},
	})
}

func (a *app) buildDashboardPayload(filters cas.QueryOptions, detailPath string) ([]byte, error) {
	idx, err := a.getRecordsIndex()
	if err != nil {
		return nil, err
	}
	if filters.Page <= 0 {
		filters.Page = 1
	}
	if filters.PageSize <= 0 {
		filters.PageSize = 10
	}
	paths, total := idx.QueryPagePaths(filters)
	items, err := cas.GetRecordsByPaths(a.db, paths)
	if err != nil {
		return nil, err
	}
	var detail any
	if detailPath != "" {
		rec, err := cas.GetRecord(a.db, detailPath)
		if err == nil {
			detail = rec
		}
	}
	return json.Marshal(map[string]any{
		"type": "dashboard",
		"records": cas.QueryResult{Total: total, Items: items},
		"detail": detail,
	})
}

func (a *app) pushClientOverview(client *wsClient) {
	if client == nil {
		return
	}
	payload, err := a.buildOverviewPayload()
	if err != nil {
		return
	}
	select {
	case client.ch <- payload:
	default:
	}
}

func (a *app) pushClientSnapshot(client *wsClient) {
	if client == nil {
		return
	}
	a.wsClientsMu.RLock()
	filters := client.filters
	detailPath := client.detailPath
	a.wsClientsMu.RUnlock()
	payload, err := a.buildDashboardPayload(filters, detailPath)
	if err != nil {
		return
	}
	select {
	case client.ch <- payload:
	default:
	}
}

func (a *app) pushRuntimeSnapshotNow() {
	runtime := a.runtime.Snapshot()
	payload, err := json.Marshal(map[string]any{
		"type":    "runtime",
		"runtime": runtime,
	})
	if err != nil {
		return
	}
	a.wsClientsMu.RLock()
	clients := make([]*wsClient, 0, len(a.wsClients))
	for client := range a.wsClients {
		clients = append(clients, client)
	}
	a.wsClientsMu.RUnlock()
	for _, client := range clients {
		select {
		case client.ch <- payload:
		default:
		}
	}
}

func (a *app) pushRuntimeSnapshot() {
	a.runtimePushMu.Lock()
	defer a.runtimePushMu.Unlock()
	if a.runtimePushTimer != nil {
		return
	}
	a.runtimePushTimer = time.AfterFunc(a.runtimePushDelay, func() {
		a.pushRuntimeSnapshotNow()
		a.runtimePushMu.Lock()
		a.runtimePushTimer = nil
		a.runtimePushMu.Unlock()
	})
}

func (a *app) flushRuntimeSnapshot() {
	a.runtimePushMu.Lock()
	if a.runtimePushTimer != nil {
		a.runtimePushTimer.Stop()
		a.runtimePushTimer = nil
	}
	a.runtimePushMu.Unlock()
	a.pushRuntimeSnapshotNow()
}

func headerContainsToken(h http.Header, key, token string) bool {
	for _, part := range h.Values(key) {
		for _, item := range splitHeaderTokens(part) {
			if item == token {
				return true
			}
		}
	}
	return false
}

func headerEqualsFold(h http.Header, key, expected string) bool {
	for _, value := range h.Values(key) {
		if equalFoldASCII(value, expected) {
			return true
		}
	}
	return false
}

func splitHeaderTokens(v string) []string {
	parts := make([]string, 0)
	for _, item := range splitComma(v) {
		if item != "" {
			parts = append(parts, toLowerASCII(trimASCII(item)))
		}
	}
	return parts
}

func splitComma(v string) []string {
	out := make([]string, 0)
	start := 0
	for i := 0; i < len(v); i++ {
		if v[i] == ',' {
			out = append(out, v[start:i])
			start = i + 1
		}
	}
	out = append(out, v[start:])
	return out
}

func trimASCII(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') { start++ }
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') { end-- }
	return s[start:end]
}

func toLowerASCII(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] = b[i] + ('a' - 'A')
		}
	}
	return string(b)
}

func equalFoldASCII(a, b string) bool {
	return toLowerASCII(trimASCII(a)) == toLowerASCII(trimASCII(b))
}

func websocketAcceptKey(key string) string {
	h := sha1Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(h)
}

func sha1Sum(data []byte) []byte {
	h := sha1.New()
	_, _ = h.Write(data)
	return h.Sum(nil)
}

func writeWebSocketTextFrame(w io.Writer, payload []byte) error {
	header := []byte{0x81}
	n := len(payload)
	switch {
	case n <= 125:
		header = append(header, byte(n))
	case n <= 65535:
		header = append(header, 126, byte(n>>8), byte(n))
	default:
		header = append(header, 127,
			byte(uint64(n)>>56), byte(uint64(n)>>48), byte(uint64(n)>>40), byte(uint64(n)>>32),
			byte(uint64(n)>>24), byte(uint64(n)>>16), byte(uint64(n)>>8), byte(uint64(n)))
	}
	if _, err := w.Write(header); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

func readWebSocketFrame(r io.Reader) (byte, []byte, error) {
	head := make([]byte, 2)
	if _, err := io.ReadFull(r, head); err != nil {
		return 0, nil, err
	}
	opcode := head[0] & 0x0f
	masked := (head[1] & 0x80) != 0
	payloadLen := uint64(head[1] & 0x7f)
	if payloadLen == 126 {
		ext := make([]byte, 2)
		if _, err := io.ReadFull(r, ext); err != nil {
			return 0, nil, err
		}
		payloadLen = uint64(ext[0])<<8 | uint64(ext[1])
	} else if payloadLen == 127 {
		ext := make([]byte, 8)
		if _, err := io.ReadFull(r, ext); err != nil {
			return 0, nil, err
		}
		payloadLen = uint64(ext[0])<<56 | uint64(ext[1])<<48 | uint64(ext[2])<<40 | uint64(ext[3])<<32 |
			uint64(ext[4])<<24 | uint64(ext[5])<<16 | uint64(ext[6])<<8 | uint64(ext[7])
	}
	var maskKey [4]byte
	if masked {
		if _, err := io.ReadFull(r, maskKey[:]); err != nil {
			return 0, nil, err
		}
	}
	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= maskKey[i%4]
		}
	}
	return opcode, payload, nil
}

func writeErr(w http.ResponseWriter, err error, code int) {
	w.WriteHeader(code)
	writeJSON(w, map[string]any{"error": err.Error()})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func init() { _ = filepath.Separator }
