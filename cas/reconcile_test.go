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

func TestInferredCASNamesAddsCommonMediaExtensionVariantsForSTRMBase(t *testing.T) {
	job := STRMJob{
		STRMPath: filepath.Join("/strm", "订阅2026", "田曦薇", "Avengers", "Season 3", "S03E11.1080p.DSNP.WEB-DL.DDP5.1.H.264-HiveWeb.strm"),
		URL:      "http://127.0.0.1/api/cas/play/123",
	}
	names := inferredCASNames(job)
	wants := map[string]bool{
		"S03E11.1080p.DSNP.WEB-DL.DDP5.1.H.264-HiveWeb.cas":     false,
		"S03E11.1080p.DSNP.WEB-DL.DDP5.1.H.264-HiveWeb.mkv.cas": false,
		"S03E11.1080p.DSNP.WEB-DL.DDP5.1.H.264-HiveWeb.mp4.cas": false,
	}
	for _, name := range names {
		if _, ok := wants[name]; ok {
			wants[name] = true
		}
	}
	for want, ok := range wants {
		if !ok {
			t.Fatalf("expected %q in inferred names, got %#v", want, names)
		}
	}
}
