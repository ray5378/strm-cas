package cas

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
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
