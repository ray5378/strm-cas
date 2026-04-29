package cas

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRenameEncodedCASFiles(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "The%20Movie%20%E4%B8%AD%E6%96%87.mkv.cas")
	if err := os.WriteFile(oldPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("write old cas err: %v", err)
	}
	summary, err := RenameEncodedCASFiles(dir)
	if err != nil {
		t.Fatalf("RenameEncodedCASFiles err: %v", err)
	}
	if summary.Renamed != 1 || summary.Conflicts != 0 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	newPath := filepath.Join(dir, "The Movie 中文.mkv.cas")
	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("expected renamed cas exists: %v", err)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("expected old cas removed, got: %v", err)
	}
}

func TestRenameEncodedCASFilesConflict(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "The%20Movie%20%E4%B8%AD%E6%96%87.mkv.cas")
	newPath := filepath.Join(dir, "The Movie 中文.mkv.cas")
	if err := os.WriteFile(oldPath, []byte("old"), 0o644); err != nil {
		t.Fatalf("write old cas err: %v", err)
	}
	if err := os.WriteFile(newPath, []byte("new"), 0o644); err != nil {
		t.Fatalf("write target cas err: %v", err)
	}
	summary, err := RenameEncodedCASFiles(dir)
	if err != nil {
		t.Fatalf("RenameEncodedCASFiles err: %v", err)
	}
	if summary.Renamed != 0 || summary.Conflicts != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if _, err := os.Stat(oldPath); err != nil {
		t.Fatalf("expected old cas kept on conflict: %v", err)
	}
}
