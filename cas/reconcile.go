package cas

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"
)

type ReconcileDetail struct {
	Action      string `json:"action"`
	STRMPath    string `json:"strm_path,omitempty"`
	RelativeDir string `json:"relative_dir,omitempty"`
	OldStatus   string `json:"old_status,omitempty"`
	NewStatus   string `json:"new_status,omitempty"`
	MatchMode   string `json:"match_mode,omitempty"`
	CASPath     string `json:"cas_path,omitempty"`
	Message     string `json:"message,omitempty"`
}

type ReconcileSummary struct {
	TotalSTRM       int               `json:"total_strm"`
	TotalCAS        int               `json:"total_cas"`
	Done            int               `json:"done"`
	Pending         int               `json:"pending"`
	Exception       int               `json:"exception"`
	Updated         int               `json:"updated"`
	DeletedStale    int               `json:"deleted_stale"`
	MatchedExisting int               `json:"matched_existing"`
	MatchedInferred int               `json:"matched_inferred"`
	Details         []ReconcileDetail `json:"details,omitempty"`
}

func ReconcileStateWithFS(db *bolt.DB, strmRoot, downloadRoot string) (*ReconcileSummary, error) {
	if db == nil {
		return nil, fmt.Errorf("nil db")
	}
	jobs, err := DiscoverSTRMJobs(strmRoot)
	if err != nil {
		return nil, err
	}
	casIndex, totalCAS, err := scanCASIndex(downloadRoot)
	if err != nil {
		return nil, err
	}
	jobCounts := make(map[string]int)
	for _, job := range jobs {
		jobCounts[job.RelativeDir]++
	}
	summary := &ReconcileSummary{TotalSTRM: len(jobs), TotalCAS: totalCAS, Details: make([]ReconcileDetail, 0, len(jobs))}
	now := time.Now().Format(time.RFC3339)
	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(stateBucket))
		if b == nil {
			return fmt.Errorf("state bucket missing")
		}
		current := make(map[string]STRMJob, len(jobs))
		for _, job := range jobs {
			current[job.STRMPath] = job
		}
		staleRecords := make([]StateRecord, 0)
		if err := b.ForEach(func(k, raw []byte) error {
			key := string(k)
			if _, ok := current[key]; ok {
				return nil
			}
			rec := StateRecord{STRMPath: key}
			if raw != nil {
				_ = json.Unmarshal(raw, &rec)
			}
			staleRecords = append(staleRecords, rec)
			return nil
		}); err != nil {
			return err
		}
		for _, rec := range staleRecords {
			if err := b.Delete([]byte(rec.STRMPath)); err != nil {
				return err
			}
			summary.DeletedStale++
			summary.Details = append(summary.Details, ReconcileDetail{
				Action:      "delete_stale",
				STRMPath:    rec.STRMPath,
				RelativeDir: rec.RelativeDir,
				OldStatus:   rec.Status,
				Message:     "record removed because .strm no longer exists",
			})
		}
		for _, job := range jobs {
			oldRec := StateRecord{}
			if raw := b.Get([]byte(job.STRMPath)); raw != nil {
				_ = json.Unmarshal(raw, &oldRec)
			}
			rec := oldRec
			before, _ := json.Marshal(oldRec)
			rec.STRMPath = job.STRMPath
			rec.URL = job.URL
			rec.RelativeDir = job.RelativeDir
			rec.LastSeenAt = now
			rec.LastProcessedAt = now

			detail := ReconcileDetail{STRMPath: job.STRMPath, RelativeDir: job.RelativeDir, OldStatus: oldRec.Status}
			if job.ParseError != "" {
				rec.Status = "exception"
				rec.LastMessage = job.ParseError
				rec.CASPath = ""
				rec.DownloadPath = ""
				rec.Size = 0
				summary.Exception++
				detail.Action = "mark_exception"
				detail.NewStatus = rec.Status
				detail.Message = job.ParseError
			} else if casPath, mode := matchExistingCAS(oldRec, job, casIndex, jobCounts); casPath != "" {
				rec.Status = "done"
				rec.LastMessage = "reconciled from existing .cas"
				rec.CASPath = casPath
				rec.DownloadPath = strings.TrimSuffix(casPath, ".cas")
				if st, statErr := os.Stat(rec.DownloadPath); statErr == nil && !st.IsDir() {
					rec.Size = st.Size()
				} else {
					rec.DownloadPath = ""
					rec.Size = 0
				}
				if mode == "existing" {
					summary.MatchedExisting++
				} else {
					summary.MatchedInferred++
				}
				summary.Done++
				detail.Action = "mark_done"
				detail.NewStatus = rec.Status
				detail.MatchMode = mode
				detail.CASPath = casPath
				detail.Message = rec.LastMessage
			} else {
				rec.Status = "pending"
				rec.LastMessage = "reconciled: no corresponding .cas found"
				rec.CASPath = ""
				rec.DownloadPath = ""
				rec.Size = 0
				summary.Pending++
				detail.Action = "mark_pending"
				detail.NewStatus = rec.Status
				detail.Message = rec.LastMessage
			}
			after, _ := json.Marshal(rec)
			changed := string(before) != string(after)
			if changed {
				summary.Updated++
				if detail.OldStatus == "" {
					detail.OldStatus = "(empty)"
				}
				summary.Details = append(summary.Details, detail)
			}
			if err := b.Put([]byte(job.STRMPath), after); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return summary, nil
}

func ReconcileState(db *bolt.DB, strmRoot, downloadRoot string) (*ReconcileSummary, error) {
	return ReconcileStateWithFS(db, strmRoot, downloadRoot)
}

func scanCASIndex(downloadRoot string) (map[string][]string, int, error) {
	index := make(map[string][]string)
	total := 0
	if strings.TrimSpace(downloadRoot) == "" {
		return index, 0, nil
	}
	err := filepath.WalkDir(downloadRoot, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.ToLower(filepath.Ext(d.Name())) != ".cas" {
			return nil
		}
		relDir, relErr := filepath.Rel(downloadRoot, filepath.Dir(p))
		if relErr != nil {
			return relErr
		}
		if relDir == "." {
			relDir = ""
		}
		index[relDir] = append(index[relDir], p)
		total++
		return nil
	})
	return index, total, err
}

func matchExistingCAS(rec StateRecord, job STRMJob, casIndex map[string][]string, jobCounts map[string]int) (string, string) {
	if rec.CASPath != "" && fileExists(rec.CASPath) {
		return rec.CASPath, "existing"
	}
	candidates := casIndex[job.RelativeDir]
	if len(candidates) == 0 {
		return "", ""
	}
	byBase := make(map[string]string, len(candidates))
	for _, p := range candidates {
		byBase[strings.ToLower(filepath.Base(p))] = p
	}
	for _, name := range inferredCASNames(job) {
		if p := byBase[strings.ToLower(name)]; p != "" {
			return p, "inferred"
		}
	}
	if len(candidates) == 1 && jobCounts[job.RelativeDir] == 1 {
		return candidates[0], "inferred"
	}
	return "", ""
}

func inferredCASNames(job STRMJob) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 6)
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		name = sanitizeFileName(name)
		if filepath.Ext(name) != ".cas" {
			name += ".cas"
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, name)
	}
	add(strings.TrimSuffix(filepath.Base(job.STRMPath), filepath.Ext(job.STRMPath)))
	if u, err := url.Parse(job.URL); err == nil {
		base := path.Base(u.Path)
		if base != "" && base != "/" && base != "." {
			add(base)
			add(decodeURLFileName(base))
		}
		for _, key := range []string{"filename", "fileName", "name"} {
			if v := strings.TrimSpace(u.Query().Get(key)); v != "" {
				add(v)
				add(decodeURLFileName(v))
			}
		}
	}
	return out
}
