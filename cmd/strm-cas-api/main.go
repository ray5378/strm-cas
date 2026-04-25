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
	mu      sync.Mutex
}

func main() {
	listen := envOr("STRM_CAS_LISTEN", ":8096")
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
	app := &app{cfg: cfg, runtime: cas.NewRuntimeStore(1000)}
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
	mux.HandleFunc("/api/scan/start", app.handleScanStart)
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
	jobs, db, err := a.openCurrent()
	if err != nil {
		writeErr(w, err, 500)
		return
	}
	defer closeDB(db)
	stats, err := cas.ComputeStats(db, jobs)
	if err != nil {
		writeErr(w, err, 500)
		return
	}
	writeJSON(w, map[string]any{"stats": stats, "runtime": a.runtime.Snapshot()})
}

func (a *app) handleRecords(w http.ResponseWriter, r *http.Request) {
	jobs, db, err := a.openCurrent()
	if err != nil {
		writeErr(w, err, 500)
		return
	}
	defer closeDB(db)
	page, size := parsePage(r), parsePageSize(r)
	result, err := cas.ListRecords(db, jobs, cas.QueryOptions{Status: r.URL.Query().Get("status"), Search: r.URL.Query().Get("search"), Page: page, PageSize: size})
	if err != nil {
		writeErr(w, err, 500)
		return
	}
	writeJSON(w, result)
}

func (a *app) handleRecordDetail(w http.ResponseWriter, r *http.Request) {
	_, db, err := a.openCurrent()
	if err != nil {
		writeErr(w, err, 500)
		return
	}
	defer closeDB(db)
	p := r.URL.Query().Get("path")
	if p == "" {
		writeErr(w, fmt.Errorf("missing path"), 400)
		return
	}
	rec, err := cas.GetRecord(db, p)
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

func (a *app) handleScanStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, fmt.Errorf("method not allowed"), 405)
		return
	}
	a.mu.Lock()
	if a.runtime.Snapshot().Running {
		a.mu.Unlock()
		writeErr(w, fmt.Errorf("scan already running"), 409)
		return
	}
	a.runtime.MarkStarted()
	cfg := a.cfg
	cfg.OnProgress = func(p cas.ProgressInfo) {
		a.runtime.SetCurrent(p)
		if p.Stage == "downloaded" {
			a.runtime.AddDownloaded(p)
		}
	}
	cfg.OnResult = func(res cas.STRMProcessResult) {
		a.runtime.AddCompleted(res)
	}
	go func() {
		defer a.runtime.MarkFinished()
		_, _ = cas.ProcessSTRMTree(cfg)
	}()
	a.mu.Unlock()
	writeJSON(w, map[string]any{"ok": true})
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
	if err := cas.ClearStateDB(a.cfg.DBPath); err != nil {
		writeErr(w, err, 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (a *app) openCurrent() ([]cas.STRMJob, *bolt.DB, error) {
	jobs, err := cas.DiscoverSTRMJobs(a.cfg.STRMRoot)
	if err != nil {
		return nil, nil, err
	}
	db, err := cas.OpenStateDB(a.cfg.DBPath)
	if err != nil {
		return nil, nil, err
	}
	if err := cas.SyncJobsToState(db, jobs); err != nil {
		_ = db.Close()
		return nil, nil, err
	}
	return jobs, db, nil
}

func closeDB(c interface{ Close() error }) { _ = c.Close() }

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

func init() {
	_ = filepath.Separator
}
