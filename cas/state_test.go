package cas

import (
	"testing"
	"time"
)

func TestStateStats(t *testing.T) {
	dir := t.TempDir()
	db, err := OpenStateDB(dir + "/state.db")
	if err != nil {
		t.Fatalf("OpenStateDB err: %v", err)
	}
	defer db.Close()

	jobs := []STRMJob{
		{STRMPath: "/strm/a.strm", URL: "http://a", RelativeDir: ""},
		{STRMPath: "/strm/b.strm", URL: "http://b", RelativeDir: ""},
		{STRMPath: "/strm/c.strm", URL: "http://c", RelativeDir: ""},
	}
	if err := SyncJobsToState(db, jobs); err != nil {
		t.Fatalf("SyncJobsToState err: %v", err)
	}
	_ = UpdateResult(db, STRMProcessResult{Job: jobs[0], Status: "done"})
	_ = UpdateResult(db, STRMProcessResult{Job: jobs[1], Status: "failed"})

	stats, err := ComputeStats(db, jobs)
	if err != nil {
		t.Fatalf("ComputeStats err: %v", err)
	}
	if stats.Total != 3 || stats.Done != 1 || stats.Failed != 1 || stats.Pending != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestRecordsIndexQuery(t *testing.T) {
	records := []StateRecord{
		{STRMPath: "/strm/c.strm", URL: "http://c", Status: "failed", LastMessage: "network error", LastProcessedAt: time.Now().Add(-time.Minute).Format(time.RFC3339)},
		{STRMPath: "/strm/a.strm", URL: "http://a", Status: "done", LastMessage: "ok", LastProcessedAt: time.Now().Format(time.RFC3339)},
		{STRMPath: "/strm/b.strm", URL: "http://b", Status: "pending", LastMessage: "", LastProcessedAt: time.Now().Add(-2 * time.Minute).Format(time.RFC3339)},
	}

	idx := BuildRecordsIndexFromRecords(records)
	if idx == nil {
		t.Fatal("expected index")
	}

	paths, total := idx.QueryPagePaths(QueryOptions{Page: 1, PageSize: 2})
	if total != 3 {
		t.Fatalf("unexpected total: %d", total)
	}
	if len(paths) != 2 {
		t.Fatalf("unexpected page size: %d", len(paths))
	}
	if paths[0] != "/strm/a.strm" {
		t.Fatalf("expected newest record first, got %s", paths[0])
	}

	failed := idx.QueryPaths(QueryOptions{Status: "failed"})
	if len(failed) != 1 || failed[0] != "/strm/c.strm" {
		t.Fatalf("unexpected failed query: %+v", failed)
	}

	search := idx.QueryPaths(QueryOptions{Search: "network"})
	if len(search) != 1 || search[0] != "/strm/c.strm" {
		t.Fatalf("unexpected search query: %+v", search)
	}

	stats := idx.Stats()
	if stats.Total != 3 || stats.Done != 1 || stats.Failed != 1 || stats.Pending != 1 {
		t.Fatalf("unexpected indexed stats: %+v", stats)
	}
}
