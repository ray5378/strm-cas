package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
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
	settingsMu       sync.RWMutex
	settings         taskSettings
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

	app := &app{
		cfg:      cfg,
		runtime:  cas.NewRuntimeStore(1000),
		db:       db,
		settings: taskSettings{Concurrency: concurrency, TotalRateLimit: int64(rateMB) * 1024 * 1024, TotalRateLimitMB: rateMB},
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
	mux.HandleFunc("/api/runtime/downloaded", app.handleRuntimeDownloaded)
	mux.HandleFunc("/api/runtime/completed", app.handleRuntimeCompleted)
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
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/web/", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	})
	mux.Handle("/web/", http.StripPrefix("/web/", http.FileServer(http.FS(webSub))))

	log.Printf("strm-cas api listening on %s", listen)
	log.Fatal(http.ListenAndServe(listen, withCORS(mux)))
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

func (a *app) handleOverview(w http.ResponseWriter, r *http.Request) {
	stats, err := cas.ComputeStatsFromDB(a.db)
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
		if a.runtime.Snapshot().Running {
			writeErr(w, fmt.Errorf("task is running, stop it before changing settings"), 409)
			return
		}
		var req struct {
			Concurrency      int `json:"concurrency"`
			TotalRateLimitMB int `json:"total_rate_limit_mb"`
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
		a.settingsMu.Lock()
		a.settings = taskSettings{Concurrency: req.Concurrency, TotalRateLimitMB: req.TotalRateLimitMB, TotalRateLimit: int64(req.TotalRateLimitMB) * 1024 * 1024}
		a.cfg.Concurrency = req.Concurrency
		a.cfg.TotalRateLimit = int64(req.TotalRateLimitMB) * 1024 * 1024
		a.settingsMu.Unlock()
		writeJSON(w, a.getSettings())
	default:
		writeErr(w, fmt.Errorf("method not allowed"), 405)
	}
}

func (a *app) handleRecords(w http.ResponseWriter, r *http.Request) {
	page, size := parsePage(r), parsePageSize(r)
	result, err := cas.ListStoredRecords(a.db, cas.QueryOptions{Status: r.URL.Query().Get("status"), Search: r.URL.Query().Get("search"), Page: page, PageSize: size})
	if err != nil {
		writeErr(w, err, 500)
		return
	}
	writeJSON(w, result)
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

func (a *app) handleRuntimeDownloaded(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, a.runtime.PaginateDownloaded(parsePage(r), parsePageSize(r)))
}

func (a *app) handleRuntimeCompleted(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, a.runtime.PaginateCompleted(parsePage(r), parsePageSize(r), r.URL.Query().Get("status")))
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
	stats, err := cas.ComputeStatsFromDB(a.db)
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
	stored, err := cas.ListStoredRecordsAll(a.db, cas.QueryOptions{Status: req.Status, Search: req.Search})
	if err != nil {
		writeErr(w, err, 500)
		return
	}
	byPath := make(map[string]cas.STRMJob, len(jobs))
	for _, job := range jobs {
		byPath[job.STRMPath] = job
	}
	filtered := make([]cas.STRMJob, 0)
	for _, rec := range stored {
		job, ok := byPath[rec.STRMPath]
		if !ok {
			continue
		}
		switch req.Mode {
		case "failed":
			if rec.Status == "failed" {
				filtered = append(filtered, job)
			}
		case "current_filter":
			filtered = append(filtered, job)
		default:
			if rec.Status == "pending" {
				filtered = append(filtered, job)
			}
		}
	}
	requested, matched := len(stored), len(filtered)
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
	stored, err := cas.ListStoredRecordsAll(a.db, cas.QueryOptions{Status: "failed"})
	if err != nil {
		writeErr(w, err, 500)
		return
	}
	byPath := make(map[string]cas.STRMJob, len(jobs))
	for _, job := range jobs {
		byPath[job.STRMPath] = job
	}
	filtered := make([]cas.STRMJob, 0)
	for _, rec := range stored {
		if job, ok := byPath[rec.STRMPath]; ok {
			filtered = append(filtered, job)
		}
	}
	requested, matched := len(stored), len(filtered)
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
		stored, err := cas.ListStoredRecordsAll(a.db, cas.QueryOptions{Status: req.Status, Search: req.Search})
		if err != nil {
			writeErr(w, err, 500)
			return
		}
		requested = len(stored)
		byPath := make(map[string]cas.STRMJob, len(jobs))
		for _, job := range jobs {
			byPath[job.STRMPath] = job
		}
		for _, rec := range stored {
			if rec.Status == "failed" {
				if job, ok := byPath[rec.STRMPath]; ok {
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
	a.setGracefulStop(false)
	settings := a.getSettings()
	cfg := a.cfg
	cfg.Concurrency = settings.Concurrency
	cfg.TotalRateLimit = settings.TotalRateLimit
	ctx, cancel := context.WithCancel(context.Background())
	cfg.Context = ctx
	cfg.OnProgress = func(p cas.ProgressInfo) {
		a.runtime.SetCurrent(p)
		if p.Stage == "downloaded" || p.Stage == "cache_recovered" {
			a.runtime.AddDownloaded(p)
		}
	}
	cfg.OnResult = func(res cas.STRMProcessResult) {
		a.runtime.AddCompleted(res)
		_ = cas.UpdateResult(a.db, res)
	}
	a.cancelMu.Lock()
	a.cancelRun = cancel
	a.cancelMu.Unlock()
	go func(selected []cas.STRMJob) {
		defer a.runtime.MarkFinished()
		defer a.setGracefulStop(false)
		defer a.mu.Unlock()
		defer func() { a.cancelMu.Lock(); a.cancelRun = nil; a.cancelMu.Unlock() }()
		client := &http.Client{Timeout: cfg.HTTPTimeout}
		limiter := cas.NewSharedRateLimiter(cfg.TotalRateLimit)
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
					res, err := cas.ProcessSingleSTRMWithContext(ctx, client, limiter, cfg, job)
					if err != nil {
						a.runtime.RemoveActive(job.STRMPath)
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
						continue
					}
					if res != nil {
						a.runtime.AddCompleted(*res)
						_ = cas.UpdateResult(a.db, *res)
					}
				}
			}()
		}
		for _, job := range selected {
			select {
			case <-ctx.Done():
				close(jobCh)
				wg.Wait()
				return
			case jobCh <- job:
			}
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
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
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
