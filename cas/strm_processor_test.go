package cas

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExtractSTRMLink(t *testing.T) {
	got, err := ExtractSTRMLink([]byte("\n  http://127.0.0.1:1234/test.mp4  \n"))
	if err != nil {
		t.Fatalf("ExtractSTRMLink err: %v", err)
	}
	if got != "http://127.0.0.1:1234/test.mp4" {
		t.Fatalf("unexpected link: %s", got)
	}
}

func TestResolveDownloadNameFromHeader(t *testing.T) {
	job := STRMJob{STRMPath: "/strm/a/test.strm", URL: "http://127.0.0.1/download/1"}
	resp := &http.Response{Header: make(http.Header)}
	resp.Header.Set("Content-Disposition", `attachment; filename="movie.mkv"`)
	if got := resolveDownloadName(job, resp); got != "movie.mkv" {
		t.Fatalf("unexpected name: %s", got)
	}
}

func TestResolveDownloadNameFromHeaderDecodesURLFilename(t *testing.T) {
	job := STRMJob{STRMPath: "/strm/a/test.strm", URL: "http://127.0.0.1/download/1"}
	resp := &http.Response{Header: make(http.Header)}
	resp.Header.Set("Content-Disposition", `attachment; filename="The%20Movie%20%E4%B8%AD%E6%96%87.mkv"`)
	if got := resolveDownloadName(job, resp); got != "The Movie 中文.mkv" {
		t.Fatalf("unexpected decoded name: %s", got)
	}
}

func TestResolveDownloadNameFromURLPathDecodesURLFilename(t *testing.T) {
	job := STRMJob{STRMPath: "/strm/a/test.strm", URL: "http://127.0.0.1/download/The%20Movie%20%E4%B8%AD%E6%96%87.mkv"}
	resp := &http.Response{Header: make(http.Header)}
	if got := resolveDownloadName(job, resp); got != "The Movie 中文.mkv" {
		t.Fatalf("unexpected decoded url name: %s", got)
	}
}

func TestResolveDownloadNameFromURLPathDecodesPlusToSpace(t *testing.T) {
	job := STRMJob{STRMPath: "/strm/a/test.strm", URL: "http://127.0.0.1/download/Room+No.9.2018.S01E01.1080p.30fps.AVC.AAC+2.0.mkv"}
	resp := &http.Response{Header: make(http.Header)}
	if got := resolveDownloadName(job, resp); got != "Room No.9.2018.S01E01.1080p.30fps.AVC.AAC 2.0.mkv" {
		t.Fatalf("unexpected decoded plus url name: %s", got)
	}
}

func TestResolveDownloadNameFallbackToSTRMBase(t *testing.T) {
	job := STRMJob{STRMPath: "/strm/a/test.strm", URL: "http://127.0.0.1/api/cas/play/164"}
	resp := &http.Response{Header: make(http.Header)}
	resp.Header.Set("Content-Type", "video/mp4")
	if got := resolveDownloadName(job, resp); got != "test.mp4" {
		t.Fatalf("unexpected name: %s", got)
	}
}

func TestWriteSummaryLog(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "logs", "summary.json")
	s := &STRMProcessSummary{StartedAt: "a", EndedAt: "b", Results: []STRMProcessResult{{Status: "done", Message: "ok"}}}
	if err := writeSummaryLog(logPath, s); err != nil {
		t.Fatalf("writeSummaryLog err: %v", err)
	}
	body, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log err: %v", err)
	}
	var out STRMProcessSummary
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("unmarshal err: %v", err)
	}
	if len(out.Results) != 1 || out.Results[0].Status != "done" {
		t.Fatalf("unexpected log content: %+v", out)
	}
}

func TestBytesToWholeGBCeil(t *testing.T) {
	oneGB := int64(1024 * 1024 * 1024)
	if got := bytesToWholeGBCeil(oneGB); got != 1 {
		t.Fatalf("unexpected ceil gb for exact 1GB: %d", got)
	}
	if got := bytesToWholeGBCeil(oneGB + 1); got != 2 {
		t.Fatalf("unexpected ceil gb for 1GB+1B: %d", got)
	}
}

func TestProcessSingleSTRMFiltersByGetContentLengthWhenHeadUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", 2*1024*1024*1024))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("mkdir cache err: %v", err)
	}
	tempPath := filepath.Join(cacheDir, urlHash(server.URL)+".part")
	if err := os.WriteFile(tempPath, []byte("stale-cache"), 0o644); err != nil {
		t.Fatalf("write temp cache err: %v", err)
	}
	job := STRMJob{STRMPath: filepath.Join(dir, "a.strm"), URL: server.URL}
	res, err := ProcessSingleSTRMWithContext(nil, server.Client(), newRateLimiter(0), STRMProcessOptions{
		CacheDir:         cacheDir,
		DownloadDir:      filepath.Join(dir, "download"),
		Mode:             Mode189PC,
		MaxFileSizeBytes: 1 * 1024 * 1024 * 1024,
	}, job)
	if err != nil {
		t.Fatalf("ProcessSingleSTRMWithContext err: %v", err)
	}
	if res == nil {
		t.Fatalf("expected filtered result, got nil")
	}
	if res.Status != "filtered" {
		t.Fatalf("expected filtered status, got: %+v", res)
	}
	if res.FilteredMaxGB != 1 || res.FilteredRemoteGB != 3 {
		t.Fatalf("unexpected filtered gb info: %+v", res)
	}
	if _, statErr := os.Stat(tempPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected temp cache removed, stat err: %v", statErr)
	}
}

func TestProcessSingleSTRMRejectsMismatchedContentRange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", "10")
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Header.Get("Range") != "bytes=4-" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Range", "bytes 0-5/10")
		w.Header().Set("Content-Length", "6")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = io.WriteString(w, "efghij")
	}))
	defer server.Close()

	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("mkdir cache err: %v", err)
	}
	tempPath := filepath.Join(cacheDir, urlHash(server.URL)+".part")
	if err := os.WriteFile(tempPath, []byte("abcd"), 0o644); err != nil {
		t.Fatalf("write temp part err: %v", err)
	}
	job := STRMJob{STRMPath: filepath.Join(dir, "a.strm"), URL: server.URL}
	_, err := ProcessSingleSTRMWithContext(nil, server.Client(), newRateLimiter(0), STRMProcessOptions{
		CacheDir:    cacheDir,
		DownloadDir: filepath.Join(dir, "download"),
		Mode:        Mode189PC,
	}, job)
	if err == nil {
		t.Fatalf("expected content-range mismatch error, got nil")
	}
}

func TestCleanupStaleCachePartsKeepsNewestByConcurrency(t *testing.T) {
	dir := t.TempDir()
	files := []string{"old-a.part", "mid-b.part", "new-c.part", "new-d.part"}
	base := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	for i, name := range files {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(name), 0o644); err != nil {
			t.Fatalf("write %s err: %v", name, err)
		}
		mtime := base.Add(time.Duration(i) * time.Minute)
		if err := os.Chtimes(p, mtime, mtime); err != nil {
			t.Fatalf("chtimes %s err: %v", name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "ignore.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write ignore err: %v", err)
	}
	if err := cleanupStaleCacheParts(dir, 2); err != nil {
		t.Fatalf("cleanupStaleCacheParts err: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "new-c.part")); err != nil {
		t.Fatalf("expected new-c.part kept, err: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "new-d.part")); err != nil {
		t.Fatalf("expected new-d.part kept, err: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "mid-b.part")); !os.IsNotExist(err) {
		t.Fatalf("expected mid-b.part removed, stat err: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "old-a.part")); !os.IsNotExist(err) {
		t.Fatalf("expected old-a.part removed, stat err: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "ignore.txt")); err != nil {
		t.Fatalf("expected ignore.txt untouched, err: %v", err)
	}
}

func TestValidateFinalFileSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.bin")
	if err := os.WriteFile(path, []byte("12345"), 0o644); err != nil {
		t.Fatalf("write file err: %v", err)
	}
	if err := validateFinalFileSize(path, 5); err != nil {
		t.Fatalf("expected size match, got err: %v", err)
	}
	if err := validateFinalFileSize(path, 6); err == nil {
		t.Fatalf("expected size mismatch error, got nil")
	}
}
