package cas

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type STRMJob struct {
	STRMPath    string `json:"strm_path"`
	URL         string `json:"url"`
	RelativeDir string `json:"relative_dir"`
	ParseError  string `json:"parse_error,omitempty"`
}

type STRMProcessOptions struct {
	STRMRoot            string
	CacheDir            string
	DownloadDir         string
	Mode                Mode
	HTTPTimeout         time.Duration
	UserAgent           string
	KeepDownload        bool
	SkipExistingCAS     bool
	MaxFileSizeBytes    int64
	LogPath             string
	DBPath              string
	Concurrency         int
	TotalRateLimit      int64
	Context             context.Context
	OnProgress          func(ProgressInfo)
	OnResult            func(STRMProcessResult)
}

type STRMProcessResult struct {
	Job              STRMJob `json:"job"`
	DownloadPath     string  `json:"download_path,omitempty"`
	CASPath          string  `json:"cas_path,omitempty"`
	Size             int64   `json:"size,omitempty"`
	FilteredMaxGB    int     `json:"filtered_max_gb,omitempty"`
	FilteredRemoteGB int64   `json:"filtered_remote_gb,omitempty"`
	Status           string  `json:"status"`
	Message          string  `json:"message,omitempty"`
}

type STRMProcessSummary struct {
	StartedAt string              `json:"started_at"`
	EndedAt   string              `json:"ended_at"`
	Results   []STRMProcessResult `json:"results"`
}

func ProcessSTRMTree(opts STRMProcessOptions) (*STRMProcessSummary, error) {
	if opts.STRMRoot == "" {
		opts.STRMRoot = "/strm"
	}
	if opts.CacheDir == "" {
		opts.CacheDir = "/cache"
	}
	if opts.DownloadDir == "" {
		opts.DownloadDir = "/download"
	}
	if opts.Mode == "" {
		opts.Mode = Mode189PC
	}
	if opts.HTTPTimeout <= 0 {
		opts.HTTPTimeout = 0
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = 1
	}
	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}
	startedAt := time.Now()
	jobs, err := DiscoverSTRMJobs(opts.STRMRoot)
	if err != nil {
		return nil, err
	}
	if err := cleanupStaleCacheParts(opts.CacheDir, opts.Concurrency); err != nil {
		return nil, err
	}
	db, err := OpenStateDB(opts.DBPath)
	if err != nil {
		return nil, err
	}
	if db != nil {
		defer db.Close()
		if err := SyncJobsToState(db, jobs); err != nil {
			return nil, err
		}
	}
	client := &http.Client{Timeout: opts.HTTPTimeout}
	limiter := newRateLimiter(opts.TotalRateLimit)
	summary := &STRMProcessSummary{StartedAt: startedAt.Format(time.RFC3339), Results: make([]STRMProcessResult, 0, len(jobs))}
	var mu sync.Mutex
	jobCh := make(chan STRMJob)
	wg := sync.WaitGroup{}
	workers := opts.Concurrency
	if workers < 1 {
		workers = 1
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobCh {
				res, err := ProcessSingleSTRMWithContext(ctx, client, limiter, opts, job)
				mu.Lock()
				if err != nil {
					status := "failed"
					if job.ParseError != "" {
						status = "exception"
					}
					failed := STRMProcessResult{Job: job, Status: status, Message: err.Error()}
					summary.Results = append(summary.Results, failed)
					if db != nil {
						_ = UpdateResult(db, failed)
					}
					mu.Unlock()
					continue
				}
				if res != nil {
					summary.Results = append(summary.Results, *res)
					if db != nil {
						_ = UpdateResult(db, *res)
					}
				}
				mu.Unlock()
			}
		}()
	}
	for _, job := range jobs {
		select {
		case <-ctx.Done():
			break
		case jobCh <- job:
		}
	}
	close(jobCh)
	wg.Wait()
	summary.EndedAt = time.Now().Format(time.RFC3339)
	if opts.LogPath != "" {
		if err := writeSummaryLog(opts.LogPath, summary); err != nil {
			return summary, err
		}
	}
	return summary, nil
}

func cleanupStaleCacheParts(cacheDir string, keep int) error {
	if strings.TrimSpace(cacheDir) == "" {
		return nil
	}
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	type cachePart struct {
		path    string
		modTime time.Time
	}
	parts := make([]cachePart, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".part") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		parts = append(parts, cachePart{path: filepath.Join(cacheDir, entry.Name()), modTime: info.ModTime()})
	}
	if keep < 0 {
		keep = 0
	}
	if len(parts) <= keep {
		return nil
	}
	sort.Slice(parts, func(i, j int) bool {
		if parts[i].modTime.Equal(parts[j].modTime) {
			return parts[i].path > parts[j].path
		}
		return parts[i].modTime.After(parts[j].modTime)
	})
	for _, part := range parts[keep:] {
		if err := os.Remove(part.path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func DiscoverSTRMJobs(root string) ([]STRMJob, error) {
	jobs := make([]STRMJob, 0)
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.ToLower(filepath.Ext(d.Name())) != ".strm" {
			return nil
		}
		relDir, err := filepath.Rel(root, filepath.Dir(p))
		if err != nil {
			return err
		}
		if relDir == "." {
			relDir = ""
		}
		job := STRMJob{STRMPath: p, RelativeDir: relDir}
		body, readErr := os.ReadFile(p)
		if readErr != nil {
			job.ParseError = fmt.Sprintf("read strm: %v", readErr)
			jobs = append(jobs, job)
			return nil
		}
		link, parseErr := ExtractSTRMLink(body)
		if parseErr != nil {
			job.ParseError = fmt.Sprintf("parse strm %s: %v", p, parseErr)
			jobs = append(jobs, job)
			return nil
		}
		job.URL = link
		jobs = append(jobs, job)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return jobs, nil
}

func ExtractSTRMLink(body []byte) (string, error) {
	text := strings.ReplaceAll(string(body), "\r\n", "\n")
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		u, err := url.Parse(line)
		if err != nil {
			return "", err
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return "", fmt.Errorf("unsupported scheme: %s", u.Scheme)
		}
		if u.Host == "" {
			return "", fmt.Errorf("missing host")
		}
		return line, nil
	}
	return "", fmt.Errorf("empty strm link")
}

func ProcessSingleSTRM(client *http.Client, opts STRMProcessOptions, job STRMJob) (*STRMProcessResult, error) {
	return ProcessSingleSTRMWithContext(opts.Context, client, newRateLimiter(opts.TotalRateLimit), opts, job)
}

func ProcessSingleSTRMWithContext(ctx context.Context, client *http.Client, limiter *RateLimiter, opts STRMProcessOptions, job STRMJob) (*STRMProcessResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if job.ParseError != "" {
		return nil, fmt.Errorf(job.ParseError)
	}
	if strings.TrimSpace(job.URL) == "" {
		return nil, fmt.Errorf("empty strm url")
	}
	if client == nil {
		client = &http.Client{Timeout: opts.HTTPTimeout}
	}
	progress := func(p ProgressInfo) {
		if opts.OnProgress != nil {
			opts.OnProgress(p)
		}
	}
	if err := os.MkdirAll(opts.CacheDir, 0o755); err != nil {
		return nil, err
	}
	downloadDir := filepath.Join(opts.DownloadDir, job.RelativeDir)
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		return nil, err
	}

	meta, _ := probeRemoteMeta(client, job, opts.UserAgent)
	nameHint := resolveDownloadName(job, metaResp(meta))
	casHintPath := filepath.Join(downloadDir, nameHint+".cas")
	progress(ProgressInfo{Job: job, Stage: "queued", FileName: nameHint, CASPath: casHintPath, Message: "queued"})
	if opts.SkipExistingCAS && fileExists(casHintPath) {
		res := &STRMProcessResult{Job: job, DownloadPath: filepath.Join(downloadDir, nameHint), CASPath: casHintPath, Status: "skipped", Message: "cas already exists"}
		if opts.OnResult != nil {
			opts.OnResult(*res)
		}
		return res, nil
	}

	tempPath := filepath.Join(opts.CacheDir, urlHash(job.URL)+".part")
	if opts.MaxFileSizeBytes > 0 && meta != nil && meta.TotalSize > opts.MaxFileSizeBytes {
		return filteredResult(job, filepath.Join(downloadDir, nameHint), casHintPath, tempPath, meta.TotalSize, opts)
	}
	partialSize := fileSizeIfExists(tempPath)
	finalPath := filepath.Join(downloadDir, nameHint)
	casPath := filepath.Join(downloadDir, nameHint+".cas")

	if partialSize > 0 && meta != nil && meta.TotalSize > 0 && partialSize == meta.TotalSize {
		progress(ProgressInfo{Job: job, Stage: "cache_recovered", FileName: nameHint, DownloadPath: finalPath, DownloadedBytes: partialSize, TotalBytes: meta.TotalSize, Message: "cache recovered"})
		return finalizeRecoveredPart(tempPath, finalPath, casPath, partialSize, opts, job, progress)
	}
	if opts.SkipExistingCAS && fileExists(casPath) {
		res := &STRMProcessResult{Job: job, DownloadPath: finalPath, CASPath: casPath, Status: "skipped", Message: "cas already exists"}
		if opts.OnResult != nil {
			opts.OnResult(*res)
		}
		return res, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, job.URL, nil)
	if err != nil {
		return nil, err
	}
	if opts.UserAgent != "" {
		req.Header.Set("User-Agent", opts.UserAgent)
	}
	if partialSize > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", partialSize))
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable && partialSize > 0 {
		if meta == nil {
			meta, _ = probeRemoteMeta(client, job, opts.UserAgent)
		}
		name := resolveDownloadName(job, metaResp(meta))
		finalPath = filepath.Join(downloadDir, name)
		casPath = filepath.Join(downloadDir, name+".cas")
		if meta != nil && meta.TotalSize > 0 && partialSize >= meta.TotalSize {
			progress(ProgressInfo{Job: job, Stage: "cache_recovered", FileName: name, DownloadPath: finalPath, DownloadedBytes: partialSize, TotalBytes: meta.TotalSize, Message: "cache recovered from 416"})
			return finalizeRecoveredPart(tempPath, finalPath, casPath, partialSize, opts, job, progress)
		}
		return nil, fmt.Errorf("range not satisfiable: local part=%d remote=%d", partialSize, metaSize(meta))
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}
	if resp.StatusCode == http.StatusPartialContent && partialSize > 0 {
		if err := validateContentRange(resp.Header.Get("Content-Range"), partialSize); err != nil {
			return nil, err
		}
	}

	name := resolveDownloadName(job, resp)
	finalPath = filepath.Join(downloadDir, name)
	casPath = filepath.Join(downloadDir, name+".cas")
	if opts.SkipExistingCAS && fileExists(casPath) {
		res := &STRMProcessResult{Job: job, DownloadPath: finalPath, CASPath: casPath, Status: "skipped", Message: "cas already exists"}
		if opts.OnResult != nil {
			opts.OnResult(*res)
		}
		return res, nil
	}
	remoteTotal := contentTotal(resp.ContentLength, partialSize)
	if opts.MaxFileSizeBytes > 0 && remoteTotal > opts.MaxFileSizeBytes {
		return filteredResult(job, finalPath, casPath, tempPath, remoteTotal, opts)
	}
	progress(ProgressInfo{Job: job, Stage: "downloading", FileName: nameHint, DownloadedBytes: partialSize, TotalBytes: remoteTotal, Message: "downloading"})

	var f *os.File
	if resp.StatusCode == http.StatusPartialContent && partialSize > 0 {
		f, err = os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	} else {
		f, err = os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		partialSize = 0
	}
	if err != nil {
		return nil, err
	}
	cr := &countingReader{ctx: ctx, reader: resp.Body, limiter: limiter, onRead: func(n int64) {
		progress(ProgressInfo{Job: job, Stage: "downloading", FileName: name, DownloadPath: finalPath, DownloadedBytes: partialSize + n, TotalBytes: contentTotal(resp.ContentLength, partialSize), Message: "downloading"})
	}}
	written, copyErr := io.Copy(f, cr)
	closeErr := f.Close()
	if copyErr != nil {
		return nil, copyErr
	}
	if closeErr != nil {
		return nil, closeErr
	}

	totalSize := partialSize + written
	expectedTotal := contentTotal(resp.ContentLength, partialSize)
	if expectedTotal > 0 && totalSize != expectedTotal {
		return nil, fmt.Errorf("download size mismatch: got %d want %d", totalSize, expectedTotal)
	}
	progress(ProgressInfo{Job: job, Stage: "downloaded", FileName: name, DownloadPath: finalPath, DownloadedBytes: totalSize, TotalBytes: totalSize, Message: "downloaded"})
	return finalizeRecoveredPart(tempPath, finalPath, casPath, totalSize, opts, job, progress)
}

func finalizeRecoveredPart(tempPath, finalPath, casPath string, totalSize int64, opts STRMProcessOptions, job STRMJob, progress func(ProgressInfo)) (*STRMProcessResult, error) {
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o755); err != nil {
		return nil, err
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		return nil, err
	}
	if err := validateFinalFileSize(finalPath, totalSize); err != nil {
		return nil, err
	}
	progress(ProgressInfo{Job: job, Stage: "downloaded", FileName: filepath.Base(finalPath), DownloadPath: finalPath, DownloadedBytes: totalSize, TotalBytes: totalSize, Message: "downloaded"})
	info, err := GenerateFromPath(finalPath, opts.Mode)
	if err != nil {
		return nil, err
	}
	progress(ProgressInfo{Job: job, Stage: "generating_cas", FileName: filepath.Base(finalPath), DownloadPath: finalPath, CASPath: casPath, DownloadedBytes: totalSize, TotalBytes: totalSize, Message: "generating cas"})
	if err := WriteCASFile(casPath, info); err != nil {
		return nil, err
	}
	if !opts.KeepDownload {
		if err := os.Remove(finalPath); err != nil {
			return nil, err
		}
	}
	progress(ProgressInfo{Job: job, Stage: "completed", FileName: filepath.Base(finalPath), CASPath: casPath, DownloadedBytes: totalSize, TotalBytes: totalSize, Message: "completed"})
	res := &STRMProcessResult{Job: job, DownloadPath: finalPath, CASPath: casPath, Size: totalSize, Status: "done", Message: "ok"}
	if opts.OnResult != nil {
		opts.OnResult(*res)
	}
	return res, nil
}

type remoteMeta struct {
	TotalSize int64
	Resp      *http.Response
}

func probeRemoteMeta(client *http.Client, job STRMJob, userAgent string) (*remoteMeta, error) {
	req, err := http.NewRequest(http.MethodHead, job.URL, nil)
	if err != nil {
		return nil, err
	}
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, fmt.Errorf("head status: %s", resp.Status)
	}
	return &remoteMeta{TotalSize: contentLengthFromHeader(resp.Header.Get("Content-Length")), Resp: resp}, nil
}

func metaSize(meta *remoteMeta) int64 {
	if meta == nil {
		return 0
	}
	return meta.TotalSize
}

func metaResp(meta *remoteMeta) *http.Response {
	if meta == nil {
		return nil
	}
	return meta.Resp
}

func contentLengthFromHeader(v string) int64 {
	if strings.TrimSpace(v) == "" {
		return 0
	}
	n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func validateContentRange(header string, expectedStart int64) error {
	header = strings.TrimSpace(header)
	if header == "" {
		return fmt.Errorf("missing Content-Range for partial response")
	}
	if !strings.HasPrefix(header, "bytes ") {
		return fmt.Errorf("invalid Content-Range: %s", header)
	}
	rangePart := strings.TrimPrefix(header, "bytes ")
	slash := strings.Index(rangePart, "/")
	if slash <= 0 {
		return fmt.Errorf("invalid Content-Range: %s", header)
	}
	span := rangePart[:slash]
	dash := strings.Index(span, "-")
	if dash <= 0 {
		return fmt.Errorf("invalid Content-Range: %s", header)
	}
	start, err := strconv.ParseInt(strings.TrimSpace(span[:dash]), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid Content-Range start: %s", header)
	}
	if start != expectedStart {
		return fmt.Errorf("content-range start mismatch: got %d want %d", start, expectedStart)
	}
	return nil
}

func validateFinalFileSize(path string, expected int64) error {
	if expected <= 0 {
		return nil
	}
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if st.IsDir() {
		return fmt.Errorf("final path is a directory: %s", path)
	}
	if st.Size() != expected {
		return fmt.Errorf("final file size mismatch: got %d want %d", st.Size(), expected)
	}
	return nil
}

func bytesToWholeGB(size int64) int {
	if size <= 0 {
		return 0
	}
	return int(size / (1024 * 1024 * 1024))
}

func filteredResult(job STRMJob, downloadPath, casPath, tempPath string, remoteTotal int64, opts STRMProcessOptions) (*STRMProcessResult, error) {
	if tempPath != "" {
		if err := os.Remove(tempPath); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	limitGB := bytesToWholeGB(opts.MaxFileSizeBytes)
	remoteGB := bytesToWholeGBCeil(remoteTotal)
	res := &STRMProcessResult{
		Job:              job,
		DownloadPath:     downloadPath,
		CASPath:          casPath,
		Size:             remoteTotal,
		FilteredMaxGB:    limitGB,
		FilteredRemoteGB: remoteGB,
		Status:           "filtered",
		Message:          fmt.Sprintf("file size %d GB exceeds limit %d GB", remoteGB, limitGB),
	}
	if opts.OnResult != nil {
		opts.OnResult(*res)
	}
	return res, nil
}

func bytesToWholeGBCeil(size int64) int64 {
	if size <= 0 {
		return 0
	}
	gb := int64(1024 * 1024 * 1024)
	return (size + gb - 1) / gb
}

func resolveDownloadName(job STRMJob, resp *http.Response) string {
	if resp != nil {
		if cd := resp.Header.Get("Content-Disposition"); cd != "" {
			if _, params, err := mime.ParseMediaType(cd); err == nil {
				if name := strings.TrimSpace(params["filename*"]); name != "" {
					if decoded := decodeRFC5987(name); decoded != "" {
						return sanitizeFileName(decoded)
					}
				}
				if name := strings.TrimSpace(params["filename"]); name != "" {
					return sanitizeFileName(decodeURLFileName(name))
				}
			}
		}
	}
	if u, err := url.Parse(job.URL); err == nil {
		base := path.Base(u.Path)
		if base != "" && base != "/" && base != "." {
			if strings.Contains(base, ".") {
				return sanitizeFileName(decodeURLFileName(base))
			}
		}
	}
	base := strings.TrimSuffix(filepath.Base(job.STRMPath), filepath.Ext(job.STRMPath))
	if resp != nil {
		if ext := extensionFromContentType(resp.Header.Get("Content-Type")); ext != "" && filepath.Ext(base) == "" {
			base += ext
		}
	}
	if base == "" {
		base = urlHash(job.URL)
	}
	return sanitizeFileName(base)
}

func decodeRFC5987(v string) string {
	parts := strings.SplitN(v, "''", 2)
	if len(parts) == 2 {
		decoded, err := url.QueryUnescape(parts[1])
		if err == nil {
			return decoded
		}
	}
	return decodeURLFileName(strings.Trim(v, "\""))
}

func decodeURLFileName(v string) string {
	v = strings.TrimSpace(strings.Trim(v, "\""))
	if v == "" {
		return ""
	}
	pathDecoded := v
	if decoded, err := url.PathUnescape(v); err == nil && strings.TrimSpace(decoded) != "" {
		pathDecoded = decoded
	}
	queryDecoded := v
	if decoded, err := url.QueryUnescape(v); err == nil && strings.TrimSpace(decoded) != "" {
		queryDecoded = decoded
	}
	if strings.Contains(v, "+") && queryDecoded != v {
		return queryDecoded
	}
	if strings.Contains(v, "%") {
		if pathDecoded != v {
			return pathDecoded
		}
		if queryDecoded != v {
			return queryDecoded
		}
	}
	if pathDecoded != v {
		return pathDecoded
	}
	if queryDecoded != v {
		return queryDecoded
	}
	return v
}

func sanitizeFileName(name string) string {
	name = strings.TrimSpace(strings.Trim(name, "\""))
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, `\\`, "_")
	if name == "" || name == "." || name == ".." {
		return "download.bin"
	}
	return name
}

func extensionFromContentType(contentType string) string {
	if contentType == "" {
		return ""
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err == nil {
		contentType = mediaType
	}
	switch strings.ToLower(contentType) {
	case "video/mp4":
		return ".mp4"
	case "video/x-matroska":
		return ".mkv"
	case "video/x-msvideo":
		return ".avi"
	case "video/webm":
		return ".webm"
	case "audio/mpeg":
		return ".mp3"
	case "audio/flac":
		return ".flac"
	case "audio/mp4":
		return ".m4a"
	case "application/pdf":
		return ".pdf"
	case "application/zip":
		return ".zip"
	default:
		return ""
	}
}

func fileSizeIfExists(path string) int64 {
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return 0
	}
	return st.Size()
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func urlHash(raw string) string {
	sum := sha1.Sum([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func writeSummaryLog(path string, summary *STRMProcessSummary) error {
	if summary == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}
