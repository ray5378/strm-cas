package cas

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConvertDownloadDirToCAS(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "movie.mkv")
	if err := os.WriteFile(src, []byte("hello world"), 0o644); err != nil {
		t.Fatalf("write source err: %v", err)
	}

	summary, err := ConvertDownloadDirToCAS(dir, Mode189PC)
	if err != nil {
		t.Fatalf("ConvertDownloadDirToCAS err: %v", err)
	}
	if summary.TotalFiles != 1 || summary.Converted != 1 || summary.Deleted != 1 || summary.Failed != 0 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if _, err := os.Stat(src + ".cas"); err != nil {
		t.Fatalf("expected cas exists: %v", err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("expected source removed, got: %v", err)
	}
}

func TestConvertDownloadDirToCASSkipExistingCASConflict(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "movie.mp4")
	if err := os.WriteFile(src, []byte("hello world"), 0o644); err != nil {
		t.Fatalf("write source err: %v", err)
	}
	if err := os.WriteFile(src+".cas", []byte("existing"), 0o644); err != nil {
		t.Fatalf("write cas err: %v", err)
	}

	summary, err := ConvertDownloadDirToCAS(dir, Mode189PC)
	if err != nil {
		t.Fatalf("ConvertDownloadDirToCAS err: %v", err)
	}
	if summary.TotalFiles != 1 || summary.Conflicts != 1 || summary.Converted != 0 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("expected source kept on conflict: %v", err)
	}
}

func TestConvertDownloadDirToCASRecursiveAndIgnoreCAS(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir err: %v", err)
	}
	src := filepath.Join(nested, "episode.mkv")
	if err := os.WriteFile(src, []byte("nested data"), 0o644); err != nil {
		t.Fatalf("write source err: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "keep.cas"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write keep cas err: %v", err)
	}

	summary, err := ConvertDownloadDirToCAS(dir, Mode189PC)
	if err != nil {
		t.Fatalf("ConvertDownloadDirToCAS err: %v", err)
	}
	if summary.TotalFiles != 1 || summary.Converted != 1 || summary.Deleted != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if _, err := os.Stat(src + ".cas"); err != nil {
		t.Fatalf("expected nested cas exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(nested, "keep.cas")); err != nil {
		t.Fatalf("expected existing cas untouched: %v", err)
	}
}
