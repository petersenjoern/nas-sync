package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type SyncResult struct {
	Files            []string
	FilesTransferred int
	BytesTransferred int64
	Err              error
}

type JournalEntry struct {
	StartedAt        time.Time `json:"started_at"`
	FinishedAt       time.Time `json:"finished_at"`
	Directories      []string  `json:"directories"`
	Files            []string  `json:"files"`
	FilesTransferred int       `json:"files_transferred"`
	BytesTransferred int64     `json:"bytes_transferred"`
	Status           string    `json:"status"`
	Errors           []string  `json:"errors"`
}

const maxJournalEntries = 100

func journalPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share", "nas-sync", "journal.json")
}

func loadJournal(path string) ([]JournalEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var entries []JournalEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func appendJournal(path string, entry JournalEntry) error {
	entries, err := loadJournal(path)
	if err != nil {
		return err
	}
	entries = append(entries, entry)
	if len(entries) > maxJournalEntries {
		entries = entries[len(entries)-maxJournalEntries:]
	}
	return writeJournalAtomic(path, entries)
}

func writeJournalAtomic(path string, entries []JournalEntry) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func parseRsyncFiles(output string) []string {
	lines := strings.Split(output, "\n")
	var files []string
	inFileList := false
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "sending incremental file list") {
			inFileList = true
			continue
		}
		if inFileList {
			if line == "" || strings.HasPrefix(line, "sent ") || strings.HasPrefix(line, "total size") {
				inFileList = false
				continue
			}
			// Skip directory entries (end with /)
			if strings.HasSuffix(line, "/") {
				continue
			}
			files = append(files, line)
		}
	}
	return files
}

func parseRsyncBytes(output string) int64 {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Total transferred file size:") {
			parts := strings.TrimPrefix(line, "Total transferred file size:")
			parts = strings.TrimSuffix(strings.TrimSpace(parts), "bytes")
			parts = strings.TrimSpace(parts)
			parts = strings.ReplaceAll(parts, ",", "")
			n, err := strconv.ParseInt(parts, 10, 64)
			if err != nil {
				return 0
			}
			return n
		}
	}
	return 0
}

func formatBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func formatTimeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < 2*time.Minute:
		return "1 minute ago"
	case d < time.Hour:
		return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
	case d < 2*time.Hour:
		return "1 hour ago"
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	case d < 48*time.Hour:
		return "1 day ago"
	default:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	}
}
