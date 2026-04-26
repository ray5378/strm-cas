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
}

func NewRuntimeStore(_ int) *RuntimeStore {
	return &RuntimeStore{active: make(map[string]ProgressInfo), samples: make(map[string]progressSample)}
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
			if deltaBytes > 0 && deltaTime >= 0.5 {
				speed := int64(float64(deltaBytes) / deltaTime)
				if speed > 0 {
					p.SpeedBytesPerSec = speed
					if p.TotalBytes > p.DownloadedBytes {
						remaining := p.TotalBytes - p.DownloadedBytes
						p.ETASeconds = remaining / speed
					}
				}
				r.samples[p.Job.STRMPath] = progressSample{DownloadedBytes: p.DownloadedBytes, At: now}
			} else {
				p.SpeedBytesPerSec = 0
				if existing, ok := r.active[p.Job.STRMPath]; ok {
					p.SpeedBytesPerSec = existing.SpeedBytesPerSec
					p.ETASeconds = existing.ETASeconds
				}
			}
		} else {
			r.samples[p.Job.STRMPath] = progressSample{DownloadedBytes: p.DownloadedBytes, At: now}
		}
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

func (r *RuntimeStore) AddCompleted(res STRMProcessResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.active, res.Job.STRMPath)
	delete(r.samples, res.Job.STRMPath)
}

type RuntimeSnapshot struct {
	Running               bool           `json:"running"`
	GracefulStopping      bool           `json:"graceful_stopping"`
	StartedAt             string         `json:"started_at,omitempty"`
	EndedAt               string         `json:"ended_at,omitempty"`
	Current               *ProgressInfo  `json:"current,omitempty"`
	ActiveCount           int            `json:"active_count"`
	ActiveItems           []ProgressInfo `json:"active_items,omitempty"`
	TotalSpeedBytesPerSec int64          `json:"total_speed_bytes_per_sec"`
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
	var totalSpeed int64
	for _, item := range r.active {
		activeItems = append(activeItems, item)
		totalSpeed += item.SpeedBytesPerSec
	}
	sort.Slice(activeItems, func(i, j int) bool {
		return activeItems[i].Job.STRMPath < activeItems[j].Job.STRMPath
	})
	return RuntimeSnapshot{
		Running:               r.running,
		GracefulStopping:      r.gracefulStopping,
		StartedAt:             r.startedAt,
		EndedAt:               r.endedAt,
		Current:               cur,
		ActiveCount:           len(r.active),
		ActiveItems:           append([]ProgressInfo(nil), activeItems...),
		TotalSpeedBytesPerSec: totalSpeed,
	}
}
