package cas

import (
	"encoding/json"
	"sort"
	"strings"

	bolt "go.etcd.io/bbolt"
)

type QueryOptions struct {
	Status   string
	Search   string
	Page     int
	PageSize int
}

type QueryResult struct {
	Total int           `json:"total"`
	Items []StateRecord `json:"items"`
}

func ListRecords(db *bolt.DB, jobs []STRMJob, opts QueryOptions) (QueryResult, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize <= 0 {
		opts.PageSize = 20
	}
	records, err := buildCurrentRecords(db, jobs)
	if err != nil {
		return QueryResult{}, err
	}
	filtered := make([]StateRecord, 0, len(records))
	for _, rec := range records {
		if opts.Status != "" && rec.Status != opts.Status {
			continue
		}
		if q := strings.ToLower(strings.TrimSpace(opts.Search)); q != "" {
			blob := strings.ToLower(strings.Join([]string{rec.STRMPath, rec.URL, rec.RelativeDir, rec.LastMessage, rec.CASPath, rec.DownloadPath}, " "))
			if !strings.Contains(blob, q) {
				continue
			}
		}
		filtered = append(filtered, rec)
	}
	sort.Slice(filtered, func(i, j int) bool {
		ai := filtered[i].LastProcessedAt
		aj := filtered[j].LastProcessedAt
		if ai == aj {
			return filtered[i].STRMPath < filtered[j].STRMPath
		}
		return ai > aj
	})
	total := len(filtered)
	start := (opts.Page - 1) * opts.PageSize
	if start > total {
		start = total
	}
	end := start + opts.PageSize
	if end > total {
		end = total
	}
	return QueryResult{Total: total, Items: filtered[start:end]}, nil
}

func GetRecord(db *bolt.DB, strmPath string) (*StateRecord, error) {
	if db == nil {
		return nil, nil
	}
	var rec *StateRecord
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(stateBucket))
		raw := b.Get([]byte(strmPath))
		if raw == nil {
			return nil
		}
		var tmp StateRecord
		if err := json.Unmarshal(raw, &tmp); err != nil {
			return err
		}
		rec = &tmp
		return nil
	})
	return rec, err
}

func buildCurrentRecords(db *bolt.DB, jobs []STRMJob) ([]StateRecord, error) {
	items := make([]StateRecord, 0, len(jobs))
	byPath := make(map[string]StateRecord, len(jobs))
	if db != nil {
		err := db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(stateBucket))
			for _, job := range jobs {
				raw := b.Get([]byte(job.STRMPath))
				if raw == nil {
					continue
				}
				var rec StateRecord
				if err := json.Unmarshal(raw, &rec); err != nil {
					return err
				}
				byPath[job.STRMPath] = rec
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	for _, job := range jobs {
		rec, ok := byPath[job.STRMPath]
		if !ok {
			rec = StateRecord{
				STRMPath:    job.STRMPath,
				URL:         job.URL,
				RelativeDir: job.RelativeDir,
				Status:      "pending",
			}
		} else {
			rec.STRMPath = job.STRMPath
			rec.URL = job.URL
			rec.RelativeDir = job.RelativeDir
			if rec.Status == "" {
				rec.Status = "pending"
			}
		}
		items = append(items, rec)
	}
	return items, nil
}
