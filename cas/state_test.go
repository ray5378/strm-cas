package cas

import (
	"testing"
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
