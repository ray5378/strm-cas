package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"strm-cas/cas"
)

func main() {
	var out string
	var mode string
	var printJSON bool
	var printBase64 bool
	var scanSTRM bool
	var strmRoot string
	var cacheDir string
	var downloadDir string
	var keepDownload bool
	var timeout time.Duration
	var userAgent string
	var skipExistingCAS bool
	var logPath string
	var dbPath string
	var showStats bool

	flag.StringVar(&out, "o", "", "output .cas file path")
	flag.StringVar(&mode, "mode", string(cas.Mode189PC), "cas generation mode")
	flag.BoolVar(&printJSON, "print-json", false, "print CAS JSON instead of writing file")
	flag.BoolVar(&printBase64, "print-base64", false, "print base64 CAS content instead of writing file")
	flag.BoolVar(&scanSTRM, "scan-strm", false, "scan /strm recursively, download links serially, generate .cas, then delete downloaded source")
	flag.StringVar(&strmRoot, "strm-root", "/data/strm", "root directory containing .strm files")
	flag.StringVar(&cacheDir, "cache-dir", "/data/cache", "directory for incomplete downloads")
	flag.StringVar(&downloadDir, "download-dir", "/data/download", "directory for completed downloads before CAS generation")
	flag.BoolVar(&keepDownload, "keep-download", false, "keep downloaded source files after CAS generation")
	flag.DurationVar(&timeout, "http-timeout", 0, "HTTP timeout, e.g. 30m; 0 means no timeout")
	flag.StringVar(&userAgent, "user-agent", "strm-cas/1.0", "HTTP User-Agent for downloads")
	flag.BoolVar(&skipExistingCAS, "skip-existing-cas", true, "skip jobs when target .cas already exists in /download")
	flag.StringVar(&logPath, "log-path", "/download/strm-cas-summary.json", "write JSON summary log to this path; empty disables logging")
	flag.StringVar(&dbPath, "db-path", "/download/strm-cas.db", "state database path")
	flag.BoolVar(&showStats, "stats", false, "scan current /strm and print database-backed processing stats")
	flag.Parse()

	if showStats {
		jobs, err := cas.DiscoverSTRMJobs(strmRoot)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		db, err := cas.OpenStateDB(dbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		defer db.Close()
		if err := cas.SyncJobsToState(db, jobs); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		stats, err := cas.ComputeStats(db, jobs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("stats: total=%d processed=%d unprocessed=%d done=%d skipped=%d failed=%d pending=%d\n", stats.Total, stats.Processed, stats.Unprocessed, stats.Done, stats.Skipped, stats.Failed, stats.Pending)
		fmt.Printf("db: %s\n", dbPath)
		return
	}

	if scanSTRM {
		summary, err := cas.ProcessSTRMTree(cas.STRMProcessOptions{
			STRMRoot:        strmRoot,
			CacheDir:        cacheDir,
			DownloadDir:     downloadDir,
			Mode:            cas.Mode(mode),
			HTTPTimeout:     timeout,
			UserAgent:       userAgent,
			KeepDownload:    keepDownload,
			SkipExistingCAS: skipExistingCAS,
			LogPath:         logPath,
			DBPath:          dbPath,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		var done, skipped, failed, exception int
		for _, res := range summary.Results {
			switch res.Status {
			case "done":
				done++
				fmt.Printf("done: strm=%s cas=%s size=%d\n", res.Job.STRMPath, res.CASPath, res.Size)
			case "skipped":
				skipped++
				fmt.Printf("skipped: strm=%s cas=%s msg=%s\n", res.Job.STRMPath, res.CASPath, res.Message)
			case "failed":
				failed++
				fmt.Printf("failed: strm=%s msg=%s\n", res.Job.STRMPath, res.Message)
			case "exception":
				exception++
				fmt.Printf("exception: strm=%s msg=%s\n", res.Job.STRMPath, res.Message)
			}
		}
		fmt.Printf("summary: total=%d done=%d skipped=%d failed=%d exception=%d\n", len(summary.Results), done, skipped, failed, exception)
		if logPath != "" {
			fmt.Printf("log: %s\n", logPath)
		}
		fmt.Printf("db: %s\n", dbPath)
		if failed > 0 {
			os.Exit(1)
		}
		return
	}

	if flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "usage: strm-cas [flags] <source-file>\n")
		flag.PrintDefaults()
		os.Exit(2)
	}

	src := flag.Arg(0)
	info, err := cas.GenerateFromPath(src, cas.Mode(mode))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if printJSON {
		body, err := cas.MarshalJSONBytes(info)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(body))
		return
	}

	if printBase64 {
		body, err := cas.MarshalBase64(info)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(body)
		return
	}

	if out == "" {
		out = cas.DefaultOutputPath(src)
	}
	if err := cas.WriteCASFile(out, info); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("generated: %s\n", out)
	fmt.Printf("name=%s size=%d md5=%s sliceMd5=%s\n", info.Name, info.Size, info.MD5, info.SliceMD5)
}
