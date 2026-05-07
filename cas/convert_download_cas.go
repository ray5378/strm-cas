package cas

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ConvertDownloadCASTaskDetail struct {
	Action     string `json:"action"`
	SourcePath string `json:"source_path,omitempty"`
	CASPath    string `json:"cas_path,omitempty"`
	Message    string `json:"message,omitempty"`
}

type ConvertDownloadCASTaskSummary struct {
	TotalFiles int                            `json:"total_files"`
	Converted  int                            `json:"converted"`
	Skipped    int                            `json:"skipped"`
	Conflicts  int                            `json:"conflicts"`
	Deleted    int                            `json:"deleted"`
	Failed     int                            `json:"failed"`
	Details    []ConvertDownloadCASTaskDetail `json:"details,omitempty"`
}

func ConvertDownloadDirToCAS(downloadRoot string, mode Mode) (*ConvertDownloadCASTaskSummary, error) {
	if strings.TrimSpace(downloadRoot) == "" {
		return &ConvertDownloadCASTaskSummary{}, nil
	}
	summary := &ConvertDownloadCASTaskSummary{Details: make([]ConvertDownloadCASTaskDetail, 0)}
	err := filepath.WalkDir(downloadRoot, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.ToLower(filepath.Ext(d.Name())) == ".cas" {
			return nil
		}
		summary.TotalFiles++
		casPath := p + ".cas"
		if _, statErr := os.Stat(casPath); statErr == nil {
			summary.Conflicts++
			summary.Details = append(summary.Details, ConvertDownloadCASTaskDetail{Action: "conflict", SourcePath: p, CASPath: casPath, Message: "target cas already exists"})
			return nil
		} else if !os.IsNotExist(statErr) {
			return fmt.Errorf("stat target cas %s: %w", casPath, statErr)
		}

		if _, err := GenerateAndWrite(p, casPath, mode); err != nil {
			summary.Failed++
			summary.Details = append(summary.Details, ConvertDownloadCASTaskDetail{Action: "failed", SourcePath: p, CASPath: casPath, Message: err.Error()})
			return nil
		}
		summary.Converted++
		summary.Details = append(summary.Details, ConvertDownloadCASTaskDetail{Action: "converted", SourcePath: p, CASPath: casPath})

		if err := os.Remove(p); err != nil {
			summary.Failed++
			summary.Details = append(summary.Details, ConvertDownloadCASTaskDetail{Action: "delete_failed", SourcePath: p, CASPath: casPath, Message: err.Error()})
			return nil
		}
		summary.Deleted++
		summary.Details = append(summary.Details, ConvertDownloadCASTaskDetail{Action: "deleted", SourcePath: p, CASPath: casPath})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return summary, nil
}
