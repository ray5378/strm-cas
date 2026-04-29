package cas

import (
	"path/filepath"
	"testing"
)

func TestInferredCASNamesDecodesPlusStyleURLBase(t *testing.T) {
	job := STRMJob{
		STRMPath: filepath.Join("/strm", "订阅2026", "田曦薇", "9号房间.strm"),
		URL:      "http://127.0.0.1/download/Room+No.9.2018.S01E01.1080p.30fps.AVC.AAC+2.0.mkv",
	}
	names := inferredCASNames(job)
	want := "Room No.9.2018.S01E01.1080p.30fps.AVC.AAC 2.0.mkv.cas"
	for _, name := range names {
		if name == want {
			return
		}
	}
	t.Fatalf("expected %q in inferred names, got %#v", want, names)
}

func TestInferredCASNamesDecodesPercentStyleURLBase(t *testing.T) {
	job := STRMJob{
		STRMPath: filepath.Join("/strm", "movie.strm"),
		URL:      "http://127.0.0.1/download/The%20Movie%20%E4%B8%AD%E6%96%87.mkv",
	}
	names := inferredCASNames(job)
	want := "The Movie 中文.mkv.cas"
	for _, name := range names {
		if name == want {
			return
		}
	}
	t.Fatalf("expected %q in inferred names, got %#v", want, names)
}
