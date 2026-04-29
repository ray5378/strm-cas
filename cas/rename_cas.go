package cas

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type RenameCASDetail struct {
	Action  string `json:"action"`
	OldPath string `json:"old_path,omitempty"`
	NewPath string `json:"new_path,omitempty"`
	Message string `json:"message,omitempty"`
}

type RenameCASSummary struct {
	TotalCAS int               `json:"total_cas"`
	Renamed  int               `json:"renamed"`
	Skipped  int               `json:"skipped"`
	Conflicts int              `json:"conflicts"`
	Details  []RenameCASDetail `json:"details,omitempty"`
}

func RenameEncodedCASFiles(downloadRoot string) (*RenameCASSummary, error) {
	if strings.TrimSpace(downloadRoot) == "" {
		return &RenameCASSummary{}, nil
	}
	summary := &RenameCASSummary{Details: make([]RenameCASDetail, 0)}
	err := filepath.WalkDir(downloadRoot, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.ToLower(filepath.Ext(d.Name())) != ".cas" {
			return nil
		}
		summary.TotalCAS++
		base := strings.TrimSuffix(d.Name(), ".cas")
		decoded := decodeURLFileName(base)
		decoded = sanitizeFileName(decoded)
		if decoded == "" {
			decoded = base
		}
		newName := decoded + ".cas"
		if newName == d.Name() {
			summary.Skipped++
			return nil
		}
		newPath := filepath.Join(filepath.Dir(p), newName)
		if _, statErr := os.Stat(newPath); statErr == nil {
			summary.Conflicts++
			summary.Details = append(summary.Details, RenameCASDetail{Action: "conflict", OldPath: p, NewPath: newPath, Message: "target file already exists"})
			return nil
		} else if !os.IsNotExist(statErr) {
			return fmt.Errorf("stat target %s: %w", newPath, statErr)
		}
		if err := os.Rename(p, newPath); err != nil {
			return fmt.Errorf("rename %s -> %s: %w", p, newPath, err)
		}
		summary.Renamed++
		summary.Details = append(summary.Details, RenameCASDetail{Action: "renamed", OldPath: p, NewPath: newPath})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return summary, nil
}
