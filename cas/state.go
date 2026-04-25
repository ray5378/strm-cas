package cas

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

const stateBucket = "strm_jobs"

type StateRecord struct {
	STRMPath        string `json:"strm_path"`
	URL             string `json:"url"`
	RelativeDir     string `json:"relative_dir"`
	Status          string `json:"status"`
	LastMessage     string `json:"last_message,omitempty"`
	CASPath         string `json:"cas_path,omitempty"`
	DownloadPath    string `json:"download_path,omitempty"`
	Size            int64  `json:"size,omitempty"`
	LastSeenAt      string `json:"last_seen_at,omitempty"`
	LastProcessedAt string `json:"last_processed_at,omitempty"`
}

type Stats struct {
	Total       int `json:"total"`
	Done        int `json:"done"`
	Skipped     int `json:"skipped"`
	Failed      int `json:"failed"`
	Exception   int `json:"exception"`
	Pending     int `json:"pending"`
	Processed   int `json:"processed"`
	Unprocessed int `json:"unprocessed"`
}

func OpenStateDB(path string) (*bolt.DB, error) {
	if path == "" {
		return nil, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, err
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(stateBucket))
		return err
	}); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func ClearStateDB(path string) error {
	if path == "" {
		return fmt.Errorf("empty db path")
	}
	db, err := OpenStateDB(path)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Update(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket([]byte(stateBucket)); err != nil && err != bolt.ErrBucketNotFound {
			return err
		}
		_, err := tx.CreateBucketIfNotExists([]byte(stateBucket))
		return err
	})
}

func UpsertDiscoveredJob(db *bolt.DB, job STRMJob) error {
	if db == nil {
		return nil
	}
	now := time.Now().Format(time.RFC3339)
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(stateBucket))
		rec := StateRecord{}
		if raw := b.Get([]byte(job.STRMPath)); raw != nil {
			_ = json.Unmarshal(raw, &rec)
		}
		if rec.Status == "" {
			rec.Status = "pending"
		}
		rec.STRMPath = job.STRMPath
		rec.URL = job.URL
		rec.RelativeDir = job.RelativeDir
		rec.LastSeenAt = now
		if job.ParseError != "" {
			rec.Status = "failed"
			rec.LastMessage = job.ParseError
			rec.LastProcessedAt = now
		}
		body, err := json.Marshal(rec)
		if err != nil {
			return err
		}
		return b.Put([]byte(job.STRMPath), body)
	})
}

func UpdateResult(db *bolt.DB, res STRMProcessResult) error {
	if db == nil {
		return nil
	}
	now := time.Now().Format(time.RFC3339)
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(stateBucket))
		rec := StateRecord{}
		if raw := b.Get([]byte(res.Job.STRMPath)); raw != nil {
			_ = json.Unmarshal(raw, &rec)
		}
		rec.STRMPath = res.Job.STRMPath
		rec.URL = res.Job.URL
		rec.RelativeDir = res.Job.RelativeDir
		rec.Status = res.Status
		rec.LastMessage = res.Message
		rec.CASPath = res.CASPath
		rec.DownloadPath = res.DownloadPath
		rec.Size = res.Size
		rec.LastSeenAt = now
		rec.LastProcessedAt = now
		body, err := json.Marshal(rec)
		if err != nil {
			return err
		}
		return b.Put([]byte(res.Job.STRMPath), body)
	})
}

func ComputeStats(db *bolt.DB, jobs []STRMJob) (Stats, error) {
	if db == nil {
		return Stats{Total: len(jobs), Pending: len(jobs), Unprocessed: len(jobs)}, nil
	}
	stats := Stats{Total: len(jobs)}
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(stateBucket))
		for _, job := range jobs {
			raw := b.Get([]byte(job.STRMPath))
			if raw == nil {
				stats.Pending++
				continue
			}
			var rec StateRecord
			if err := json.Unmarshal(raw, &rec); err != nil {
				return fmt.Errorf("unmarshal state for %s: %w", job.STRMPath, err)
			}
			switch rec.Status {
			case "done":
				stats.Done++
			case "skipped":
				stats.Skipped++
			case "failed":
				stats.Failed++
			case "exception":
				stats.Exception++
			default:
				stats.Pending++
			}
		}
		return nil
	})
	stats.Processed = stats.Done + stats.Skipped
	stats.Unprocessed = stats.Total - stats.Processed
	return stats, err
}

func SyncJobsToState(db *bolt.DB, jobs []STRMJob) error {
	for _, job := range jobs {
		if err := UpsertDiscoveredJob(db, job); err != nil {
			return err
		}
	}
	return nil
}
