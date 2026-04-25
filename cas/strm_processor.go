package cas

import (
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
	"strings"
	"time"
)

type STRMJob struct {
	STRMPath    string `json:"strm_path"`
	URL         string `json:"url"`
	RelativeDir string `json:"relative_dir"`
}

type STRMProcessOptions struct {
	STRMRoot        string
	CacheDir        string
	DownloadDir     string
	Mode            Mode
	HTTPTimeout     time.Duration
	UserAgent       string
	KeepDownload    bool
	SkipExistingCAS bool
	LogPath         string
	DBPath          string
	OnProgress      func(ProgressInfo)
	OnResult        func(STRMProcessResult)
}

type STRMProcessResult struct {
	Job          STRMJob `json:"job"`
	DownloadPath string  `json:"download_path,omitempty"`
	CASPath      string  `json:"cas_path,omitempty"`
	Size         int64   `json:"size,omitempty"`
	Status       string  `json:"status"`
	Message      string  `json:"message,omitempty"`
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
	startedAt := time.Now()
	jobs, err := DiscoverSTRMJobs(opts.STRMRoot)
	if err != nil {
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
	summary := &STRMProcessSummary{
		StartedAt: startedAt.Format(time.RFC3339),
		Results:   make([]STRMProcessResult, 0, len(jobs)),
	}
	for _, job := range jobs {
		res, err := ProcessSingleSTRM(client, opts, job)
		if err != nil {
			failed := STRMProcessResult{
				Job:     job,
				Status:  "failed",
				Message: err.Error(),
			}
			summary.Results = append(summary.Results, failed)
			if db != nil {
				_ = UpdateResult(db, failed)
			}
			continue
		}
		summary.Results = append(summary.Results, *res)
		if db != nil {
			_ = UpdateResult(db, *res)
		}
	}
	summary.EndedAt = time.Now().Format(time.RFC3339)
	if opts.LogPath != "" {
		if err := writeSummaryLog(opts.LogPath, summary); err != nil {
			return summary, err
		}
	}
	return summary, nil
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
		body, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("read strm: %w", err)
		}
		link, err := ExtractSTRMLink(body)
		if err != nil {
			return fmt.Errorf("parse strm %s: %w", p, err)
		}
		relDir, err := filepath.Rel(root, filepath.Dir(p))
		if err != nil {
			return err
		}
		if relDir == "." {
			relDir = ""
		}
		jobs = append(jobs, STRMJob{STRMPath: p, URL: link, RelativeDir: relDir})
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

	nameHint := resolveDownloadName(job, nil)
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
	partialSize := fileSizeIfExists(tempPath)

	req, err := http.NewRequest(http.MethodGet, job.URL, nil)
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
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}
	progress(ProgressInfo{Job: job, Stage: "downloading", FileName: nameHint, DownloadedBytes: partialSize, TotalBytes: resp.ContentLength, Message: "downloading"})

	name := resolveDownloadName(job, resp)
	finalPath := filepath.Join(downloadDir, name)
	casPath := filepath.Join(downloadDir, name+".cas")
	if opts.SkipExistingCAS && fileExists(casPath) {
		res := &STRMProcessResult{Job: job, DownloadPath: finalPath, CASPath: casPath, Status: "skipped", Message: "cas already exists"}
		if opts.OnResult != nil {
			opts.OnResult(*res)
		}
		return res, nil
	}

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
	cr := &countingReader{reader: resp.Body, onRead: func(n int64) {
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
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o755); err != nil {
		return nil, err
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		return nil, err
	}
	progress(ProgressInfo{Job: job, Stage: "downloaded", FileName: name, DownloadPath: finalPath, DownloadedBytes: totalSize, TotalBytes: totalSize, Message: "downloaded"})

	info, err := GenerateFromPath(finalPath, opts.Mode)
	if err != nil {
		return nil, err
	}
	progress(ProgressInfo{Job: job, Stage: "generating_cas", FileName: name, DownloadPath: finalPath, CASPath: casPath, DownloadedBytes: totalSize, TotalBytes: totalSize, Message: "generating cas"})
	if err := WriteCASFile(casPath, info); err != nil {
		return nil, err
	}
	if !opts.KeepDownload {
		if err := os.Remove(finalPath); err != nil {
			return nil, err
		}
	}
	progress(ProgressInfo{Job: job, Stage: "completed", FileName: name, CASPath: casPath, DownloadedBytes: totalSize, TotalBytes: totalSize, Message: "completed"})
	res := &STRMProcessResult{Job: job, DownloadPath: finalPath, CASPath: casPath, Size: totalSize, Status: "done", Message: "ok"}
	if opts.OnResult != nil {
		opts.OnResult(*res)
	}
	return res, nil
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
					return sanitizeFileName(name)
				}
			}
		}
	}
	if u, err := url.Parse(job.URL); err == nil {
		base := path.Base(u.Path)
		if base != "" && base != "/" && base != "." {
			if strings.Contains(base, ".") {
				return sanitizeFileName(base)
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
	return strings.Trim(v, "\"")
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
