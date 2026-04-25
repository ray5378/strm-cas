package main

import (
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

//go:embed web/* web/assets/*
var webFS embed.FS

type app struct {
	cfg     cas.STRMProcessOptions
	runtime *cas.RuntimeStore
	db      *bolt.DB
	mu      sync.Mutex
}

func main() {
	listen := envOr("STRM_CAS_LISTEN", ":18457")
	cfg := cas.STRMProcessOptions{
		STRMRoot:        envOr("STRM_CAS_STRM_ROOT", "/strm"),
		CacheDir:        envOr("STRM_CAS_CACHE_DIR", "/cache"),
		DownloadDir:     envOr("STRM_CAS_DOWNLOAD_DIR", "/download"),
		Mode:            cas.Mode189PC,
		UserAgent:       envOr("STRM_CAS_USER_AGENT", "strm-cas-api/1.0"),
		SkipExistingCAS: true,
		LogPath:         envOr("STRM_CAS_LOG_PATH", "/download/strm-cas-summary.json"),
		DBPath:          envOr("STRM_CAS_DB_PATH", "/download/strm-cas.db"),
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

	app := &app{cfg: cfg, runtime: cas.NewRuntimeStore(1000), db: db}
	webSub, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/overview", app.handleOverview)
	mux.HandleFunc("/api/records", app.handleRecords)
	mux.HandleFunc("/api/records/detail", app.handleRecordDetail)
	mux.HandleFunc("/api/runtime", app.handleRuntime)
	mux.HandleFunc("/api/runtime/downloaded", app.handleRuntimeDownloaded)
	mux.HandleFunc("/api/runtime/completed", app.handleRuntimeCompleted)
	mux.HandleFunc("/api/scan/refresh", app.handleScanRefresh)
	mux.HandleFunc("/api/tasks/start", app.handleTasksStart)
	mux.HandleFunc("/api/tasks/retry", app.handleTaskRetry)
	mux.HandleFunc("/api/tasks/retry-failed", app.handleRetryFailed)
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

func (a *app) handleOverview(w http.ResponseWriter, r *http.Request) {
	jobs, err := a.currentJobs(true)
	if err != nil {
		writeErr(w, err, 500)
		return
	}
	stats, err := cas.ComputeStats(a.db, jobs)
	if err != nil {
		writeErr(w, err, 500)
		return
	}
	writeJSON(w, map[string]any{"stats": stats, "runtime": a.runtime.Snapshot()})
}

func (a *app) handleRecords(w http.ResponseWriter, r *http.Request) {
	jobs, err := a.currentJobs(true)
	if err != nil {
		writeErr(w, err, 500)
		return
	}
	page, size := parsePage(r), parsePageSize(r)
	result, err := cas.ListRecords(a.db, jobs, cas.QueryOptions{Status: r.URL.Query().Get("status"), Search: r.URL.Query().Get("search"), Page: page, PageSize: size})
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
	stats, err := cas.ComputeStats(a.db, jobs)
	if err != nil {
		writeErr(w, err, 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "stats": stats, "total": len(jobs)})
}

func (a *app) handleTasksStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, fmt.Errorf("method not allowed"), 405)
		return
	}
	jobs, err := a.currentJobs(true)
	if err != nil {
		writeErr(w, err, 500)
		return
	}
	filtered := make([]cas.STRMJob, 0, len(jobs))
	for _, job := range jobs {
		rec, _ := cas.GetRecord(a.db, job.STRMPath)
		status := "pending"
		if rec != nil && rec.Status != "" {
			status = rec.Status
		}
		if status == "pending" || status == "failed" {
			filtered = append(filtered, job)
		}
	}
	if len(filtered) == 0 {
		writeJSON(w, map[string]any{"ok": true, "started": 0})
		return
	}
	if err := a.startJobs(filtered); err != nil {
		writeErr(w, err, 409)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "started": len(filtered)})
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
	jobs, err := a.currentJobs(true)
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
	writeJSON(w, map[string]any{"ok": true, "started": 1, "path": req.Path})
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
	if len(filtered) == 0 {
		writeJSON(w, map[string]any{"ok": true, "started": 0})
		return
	}
	if err := a.startJobs(filtered); err != nil {
		writeErr(w, err, 409)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "started": len(filtered)})
}

func (a *app) handleDBClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, fmt.Errorf("method not allowed"), 405)
		return
	}
	if a.runtime.Snapshot().Running {
		writeErr(w, fmt.Errorf("scan is running, cannot clear database"), 409)
		return
	}
	if err := cas.ClearStateDBHandle(a.db); err != nil {
		writeErr(w, err, 500)
		return
	}
	a.runtime.Reset()
	writeJSON(w, map[string]any{"ok": true})
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
	cfg := a.cfg
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
	go func(selected []cas.STRMJob) {
		defer a.runtime.MarkFinished()
		defer a.mu.Unlock()
		client := &http.Client{Timeout: cfg.HTTPTimeout}
		for _, job := range selected {
			res, err := cas.ProcessSingleSTRM(client, cfg, job)
			if err != nil {
				status := "failed"
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
