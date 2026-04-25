package cas

import (
	"sort"
	"sync"
	"time"
)

type ProgressInfo struct {
	Job              STRMJob `json:"job"`
	Stage            string  `json:"stage"`
	FileName         string  `json:"file_name,omitempty"`
	DownloadPath     string  `json:"download_path,omitempty"`
	CASPath          string  `json:"cas_path,omitempty"`
	DownloadedBytes  int64   `json:"downloaded_bytes,omitempty"`
	TotalBytes       int64   `json:"total_bytes,omitempty"`
	SpeedBytesPerSec int64   `json:"speed_bytes_per_sec,omitempty"`
	ETASeconds       int64   `json:"eta_seconds,omitempty"`
	Message          string  `json:"message,omitempty"`
	UpdatedAt        string  `json:"updated_at"`
}

type progressSample struct {
	DownloadedBytes int64
	At              time.Time
}

type RuntimeStore struct {
	mu               sync.RWMutex
	running          bool
	gracefulStopping bool
	startedAt        string
	endedAt          string
	current          *ProgressInfo
	active           map[string]ProgressInfo
	samples          map[string]progressSample
	downloaded       []ProgressInfo
	completed        []STRMProcessResult
	maxHistory       int
}

func NewRuntimeStore(maxHistory int) *RuntimeStore {
	if maxHistory <= 0 {
		maxHistory = 500
	}
	return &RuntimeStore{maxHistory: maxHistory, active: make(map[string]ProgressInfo), samples: make(map[string]progressSample)}
}

func (r *RuntimeStore) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.running = false
	r.gracefulStopping = false
	r.startedAt = ""
	r.endedAt = ""
	r.current = nil
	r.active = make(map[string]ProgressInfo)
	r.samples = make(map[string]progressSample)
	r.downloaded = nil
	r.completed = nil
}

func (r *RuntimeStore) MarkStarted() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.running = true
	r.gracefulStopping = false
	r.startedAt = time.Now().Format(time.RFC3339)
	r.endedAt = ""
	r.current = nil
	r.active = make(map[string]ProgressInfo)
	r.samples = make(map[string]progressSample)
	r.downloaded = nil
	r.completed = nil
}

func (r *RuntimeStore) MarkFinished() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.running = false
	r.gracefulStopping = false
	r.endedAt = time.Now().Format(time.RFC3339)
	r.current = nil
	r.active = make(map[string]ProgressInfo)
	r.samples = make(map[string]progressSample)
}

func (r *RuntimeStore) SetGracefulStopping(v bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.gracefulStopping = v
}

func (r *RuntimeStore) IsGracefulStopping() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.gracefulStopping
}

func (r *RuntimeStore) SetCurrent(p ProgressInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	if p.Job.STRMPath != "" {
		if prev, ok := r.samples[p.Job.STRMPath]; ok {
			deltaBytes := p.DownloadedBytes - prev.DownloadedBytes
			deltaTime := now.Sub(prev.At).Seconds()
			if deltaBytes > 0 && deltaTime > 0.2 {
				speed := int64(float64(deltaBytes) / deltaTime)
				if speed > 0 {
					p.SpeedBytesPerSec = speed
					if p.TotalBytes > p.DownloadedBytes {
						remaining := p.TotalBytes - p.DownloadedBytes
						p.ETASeconds = remaining / speed
					}
				}
			}
		}
		r.samples[p.Job.STRMPath] = progressSample{DownloadedBytes: p.DownloadedBytes, At: now}
	}
	p.UpdatedAt = now.Format(time.RFC3339)
	cp := p
	r.current = &cp
	if p.Job.STRMPath != "" {
		r.active[p.Job.STRMPath] = cp
	}
}

func (r *RuntimeStore) RemoveActive(strmPath string) {
	if strmPath == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.active, strmPath)
	delete(r.samples, strmPath)
}

func (r *RuntimeStore) AddDownloaded(p ProgressInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p.UpdatedAt = time.Now().Format(time.RFC3339)
	r.downloaded = append([]ProgressInfo{p}, r.downloaded...)
	if len(r.downloaded) > r.maxHistory {
		r.downloaded = r.downloaded[:r.maxHistory]
	}
}

func (r *RuntimeStore) AddCompleted(res STRMProcessResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.active, res.Job.STRMPath)
	delete(r.samples, res.Job.STRMPath)
	r.completed = append([]STRMProcessResult{res}, r.completed...)
	if len(r.completed) > r.maxHistory {
		r.completed = r.completed[:r.maxHistory]
	}
}

type RuntimeSnapshot struct {
	Running          bool           `json:"running"`
	GracefulStopping bool           `json:"graceful_stopping"`
	StartedAt        string         `json:"started_at,omitempty"`
	EndedAt          string         `json:"ended_at,omitempty"`
	Current          *ProgressInfo  `json:"current,omitempty"`
	ActiveCount      int            `json:"active_count"`
	ActiveItems      []ProgressInfo `json:"active_items,omitempty"`
	DownloadedCount  int            `json:"downloaded_count"`
	CompletedCount   int            `json:"completed_count"`
}

func (r *RuntimeStore) Snapshot() RuntimeSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var cur *ProgressInfo
	if r.current != nil {
		c := *r.current
		cur = &c
	}
	activeItems := make([]ProgressInfo, 0, len(r.active))
	for _, item := range r.active {
		activeItems = append(activeItems, item)
	}
	sort.Slice(activeItems, func(i, j int) bool {
		return activeItems[i].UpdatedAt > activeItems[j].UpdatedAt
	})
	if len(activeItems) > 8 {
		activeItems = activeItems[:8]
	}
	return RuntimeSnapshot{
		Running:          r.running,
		GracefulStopping: r.gracefulStopping,
		StartedAt:        r.startedAt,
		EndedAt:          r.endedAt,
		Current:          cur,
		ActiveCount:      len(r.active),
		ActiveItems:      append([]ProgressInfo(nil), activeItems...),
		DownloadedCount:  len(r.downloaded),
		CompletedCount:   len(r.completed),
	}
}

func (r *RuntimeStore) PaginateDownloaded(page, pageSize int) QueryRuntimeProgress {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return paginateProgress(r.downloaded, page, pageSize)
}

func (r *RuntimeStore) PaginateCompleted(page, pageSize int, status string) QueryRuntimeResults {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := r.completed
	if status != "" {
		filtered := make([]STRMProcessResult, 0, len(items))
		for _, item := range items {
			if item.Status == status {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	sort.SliceStable(items, func(i, j int) bool { return i < j })
	return paginateResults(items, page, pageSize)
}

type QueryRuntimeProgress struct {
	Total int            `json:"total"`
	Items []ProgressInfo `json:"items"`
}

type QueryRuntimeResults struct {
	Total int                 `json:"total"`
	Items []STRMProcessResult `json:"items"`
}

func paginateProgress(items []ProgressInfo, page, pageSize int) QueryRuntimeProgress {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	total := len(items)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	out := append([]ProgressInfo(nil), items[start:end]...)
	return QueryRuntimeProgress{Total: total, Items: out}
}

func paginateResults(items []STRMProcessResult, page, pageSize int) QueryRuntimeResults {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	total := len(items)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	out := append([]STRMProcessResult(nil), items[start:end]...)
	return QueryRuntimeResults{Total: total, Items: out}
}
