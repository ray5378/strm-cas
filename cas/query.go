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

type indexedRecord struct {
	path       string
	status     string
	searchText string
}

type RecordsIndex struct {
	all      []indexedRecord
	byStatus map[string][]indexedRecord
	stats    Stats
}

func BuildRecordsIndexFromRecords(records []StateRecord) *RecordsIndex {
	normalized := make([]StateRecord, 0, len(records))
	for _, rec := range records {
		if rec.Status == "" {
			rec.Status = "pending"
		}
		normalized = append(normalized, rec)
	}
	sortRecords(normalized)
	idx := &RecordsIndex{
		all:      make([]indexedRecord, 0, len(normalized)),
		byStatus: make(map[string][]indexedRecord),
		stats:    Stats{Total: len(normalized)},
	}
	for _, rec := range normalized {
		item := indexedRecord{
			path:       rec.STRMPath,
			status:     rec.Status,
			searchText: strings.ToLower(strings.Join([]string{rec.STRMPath, rec.URL, rec.RelativeDir, rec.LastMessage, rec.CASPath, rec.DownloadPath}, " ")),
		}
		idx.all = append(idx.all, item)
		idx.byStatus[rec.Status] = append(idx.byStatus[rec.Status], item)
		switch rec.Status {
		case "done":
			idx.stats.Done++
		case "skipped":
			idx.stats.Skipped++
		case "filtered":
			idx.stats.Filtered++
		case "failed":
			idx.stats.Failed++
		case "exception":
			idx.stats.Exception++
		default:
			idx.stats.Pending++
		}
	}
	idx.stats.Processed = idx.stats.Done + idx.stats.Skipped + idx.stats.Filtered
	idx.stats.Unprocessed = idx.stats.Total - idx.stats.Processed
	return idx
}

func BuildRecordsIndexFromDB(db *bolt.DB) (*RecordsIndex, error) {
	records, err := loadAllRecords(db)
	if err != nil {
		return nil, err
	}
	return BuildRecordsIndexFromRecords(records), nil
}

func (idx *RecordsIndex) Count(opts QueryOptions) int {
	if idx == nil {
		return 0
	}
	return len(idx.filter(opts))
}

func (idx *RecordsIndex) QueryPaths(opts QueryOptions) []string {
	if idx == nil {
		return []string{}
	}
	filtered := idx.filter(opts)
	items := make([]string, 0, len(filtered))
	for _, item := range filtered {
		items = append(items, item.path)
	}
	return items
}

func (idx *RecordsIndex) QueryPagePaths(opts QueryOptions) ([]string, int) {
	if idx == nil {
		return []string{}, 0
	}
	filtered := idx.filter(opts)
	return paginateIndexedRecordPaths(filtered, opts), len(filtered)
}

func (idx *RecordsIndex) filter(opts QueryOptions) []indexedRecord {
	base := idx.all
	if opts.Status != "" {
		base = idx.byStatus[opts.Status]
	}
	if q := strings.ToLower(strings.TrimSpace(opts.Search)); q != "" {
		filtered := make([]indexedRecord, 0, len(base))
		for _, item := range base {
			if strings.Contains(item.searchText, q) {
				filtered = append(filtered, item)
			}
		}
		return filtered
	}
	return base
}

func paginateIndexedRecordPaths(records []indexedRecord, opts QueryOptions) []string {
	total := len(records)
	start := (opts.Page - 1) * opts.PageSize
	if start > total {
		start = total
	}
	end := start + opts.PageSize
	if end > total {
		end = total
	}
	items := make([]string, 0, end-start)
	for _, item := range records[start:end] {
		items = append(items, item.path)
	}
	return items
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
	return filterPaginateRecords(records, opts), nil
}

func ListStoredRecords(db *bolt.DB, opts QueryOptions) (QueryResult, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize <= 0 {
		opts.PageSize = 20
	}
	idx, err := BuildRecordsIndexFromDB(db)
	if err != nil {
		return QueryResult{}, err
	}
	paths, total := idx.QueryPagePaths(opts)
	items, err := GetRecordsByPaths(db, paths)
	if err != nil {
		return QueryResult{}, err
	}
	return QueryResult{Total: total, Items: items}, nil
}

func ListStoredRecordsAll(db *bolt.DB, opts QueryOptions) ([]StateRecord, error) {
	idx, err := BuildRecordsIndexFromDB(db)
	if err != nil {
		return nil, err
	}
	paths := idx.QueryPaths(opts)
	return GetRecordsByPaths(db, paths)
}

func (idx *RecordsIndex) Stats() Stats {
	if idx == nil {
		return Stats{}
	}
	return idx.stats
}

func ComputeStatsFromDB(db *bolt.DB) (Stats, error) {
	idx, err := BuildRecordsIndexFromDB(db)
	if err != nil {
		return Stats{}, err
	}
	return idx.Stats(), nil
}

func GetRecord(db *bolt.DB, strmPath string) (*StateRecord, error) {
	if db == nil {
		return nil, nil
	}
	var rec *StateRecord
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(stateBucket))
		if b == nil {
			return nil
		}
		raw := b.Get([]byte(strmPath))
		if raw == nil {
			return nil
		}
		var tmp StateRecord
		if err := json.Unmarshal(raw, &tmp); err != nil {
			return err
		}
		if tmp.Status == "" {
			tmp.Status = "pending"
		}
		rec = &tmp
		return nil
	})
	return rec, err
}

func GetRecordsByPaths(db *bolt.DB, paths []string) ([]StateRecord, error) {
	if db == nil || len(paths) == 0 {
		return []StateRecord{}, nil
	}
	items := make([]StateRecord, 0, len(paths))
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(stateBucket))
		if b == nil {
			return nil
		}
		for _, path := range paths {
			raw := b.Get([]byte(path))
			if raw == nil {
				continue
			}
			var rec StateRecord
			if err := json.Unmarshal(raw, &rec); err != nil {
				return err
			}
			if rec.Status == "" {
				rec.Status = "pending"
			}
			items = append(items, rec)
		}
		return nil
	})
	return items, err
}

func GetRecordStatusesByPaths(db *bolt.DB, paths []string) (map[string]string, error) {
	if db == nil || len(paths) == 0 {
		return map[string]string{}, nil
	}
	statuses := make(map[string]string, len(paths))
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(stateBucket))
		if b == nil {
			return nil
		}
		for _, path := range paths {
			raw := b.Get([]byte(path))
			if raw == nil {
				continue
			}
			var rec StateRecord
			if err := json.Unmarshal(raw, &rec); err != nil {
				return err
			}
			status := rec.Status
			if status == "" {
				status = "pending"
			}
			statuses[path] = status
		}
		return nil
	})
	return statuses, err
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
			rec = StateRecord{STRMPath: job.STRMPath, URL: job.URL, RelativeDir: job.RelativeDir, Status: "pending"}
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

func loadAllRecords(db *bolt.DB) ([]StateRecord, error) {
	if db == nil {
		return []StateRecord{}, nil
	}
	items := make([]StateRecord, 0)
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(stateBucket))
		if b == nil {
			return nil
		}
		return b.ForEach(func(_, raw []byte) error {
			var rec StateRecord
			if err := json.Unmarshal(raw, &rec); err != nil {
				return err
			}
			if rec.Status == "" {
				rec.Status = "pending"
			}
			items = append(items, rec)
			return nil
		})
	})
	return items, err
}

func filterPaginateRecords(records []StateRecord, opts QueryOptions) QueryResult {
	filtered := applyRecordFilters(records, opts)
	sortRecords(filtered)
	total := len(filtered)
	start := (opts.Page - 1) * opts.PageSize
	if start > total {
		start = total
	}
	end := start + opts.PageSize
	if end > total {
		end = total
	}
	return QueryResult{Total: total, Items: filtered[start:end]}
}

func applyRecordFilters(records []StateRecord, opts QueryOptions) []StateRecord {
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
	return filtered
}

func sortRecords(records []StateRecord) {
	sort.Slice(records, func(i, j int) bool {
		ai := records[i].LastProcessedAt
		aj := records[j].LastProcessedAt
		if ai == aj {
			return records[i].STRMPath < records[j].STRMPath
		}
		return ai > aj
	})
}
