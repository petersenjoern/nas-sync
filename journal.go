package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

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
