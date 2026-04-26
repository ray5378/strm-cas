package cas

import (
	"fmt"
	"path/filepath"
	"testing"

	bolt "go.etcd.io/bbolt"
)

func seedBenchmarkRecords(b *testing.B, total int) *boltDBWrap {
	b.Helper()
	dir := b.TempDir()
	db, err := OpenStateDB(filepath.Join(dir, "bench.db"))
	if err != nil {
		b.Fatalf("OpenStateDB err: %v", err)
	}
	statuses := []string{"pending", "done", "failed", "exception", "skipped"}
	for i := 0; i < total; i++ {
		job := STRMJob{
			STRMPath:    fmt.Sprintf("/strm/%06d.strm", i),
			URL:         fmt.Sprintf("http://example.com/%06d", i),
			RelativeDir: fmt.Sprintf("group/%02d", i%100),
		}
		if err := UpsertDiscoveredJob(db, job); err != nil {
			b.Fatalf("UpsertDiscoveredJob err: %v", err)
		}
		res := STRMProcessResult{
			Job:          job,
			Status:       statuses[i%len(statuses)],
			Message:      fmt.Sprintf("msg-%d", i),
			CASPath:      fmt.Sprintf("/cas/%06d.cas", i),
			DownloadPath: fmt.Sprintf("/download/%06d.bin", i),
		}
		if err := UpdateResult(db, res); err != nil {
			b.Fatalf("UpdateResult err: %v", err)
		}
	}
	return &boltDBWrap{DB: db}
}

type boltDBWrap struct { *bolt.DB }

func (w *boltDBWrap) Close() { _ = w.DB.Close() }

func BenchmarkBuildRecordsIndexFromDB_5k(b *testing.B) {
	wrap := seedBenchmarkRecords(b, 5000)
	defer wrap.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx, err := BuildRecordsIndexFromDB(wrap.DB)
		if err != nil {
			b.Fatalf("BuildRecordsIndexFromDB err: %v", err)
		}
		if idx == nil {
			b.Fatal("nil index")
		}
	}
}

func BenchmarkQueryPageAndFetch_5k(b *testing.B) {
	wrap := seedBenchmarkRecords(b, 5000)
	defer wrap.Close()
	idx, err := BuildRecordsIndexFromDB(wrap.DB)
	if err != nil {
		b.Fatalf("BuildRecordsIndexFromDB err: %v", err)
	}
	opts := QueryOptions{Status: "failed", Search: "msg-1", Page: 1, PageSize: 10}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		paths, total := idx.QueryPagePaths(opts)
		if total < 0 {
			b.Fatal("invalid total")
		}
		items, err := GetRecordsByPaths(wrap.DB, paths)
		if err != nil {
			b.Fatalf("GetRecordsByPaths err: %v", err)
		}
		if len(items) > 10 {
			b.Fatalf("unexpected items len: %d", len(items))
		}
	}
}

func BenchmarkGetRecordStatusesByPaths_5k(b *testing.B) {
	wrap := seedBenchmarkRecords(b, 5000)
	defer wrap.Close()
	idx, err := BuildRecordsIndexFromDB(wrap.DB)
	if err != nil {
		b.Fatalf("BuildRecordsIndexFromDB err: %v", err)
	}
	paths := idx.QueryPaths(QueryOptions{Status: "failed"})
	if len(paths) > 500 {
		paths = paths[:500]
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		statuses, err := GetRecordStatusesByPaths(wrap.DB, paths)
		if err != nil {
			b.Fatalf("GetRecordStatusesByPaths err: %v", err)
		}
		if len(statuses) == 0 {
			b.Fatal("expected statuses")
		}
	}
}
