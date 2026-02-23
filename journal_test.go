package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestJournalEntryJSON(t *testing.T) {
	entry := JournalEntry{
		StartedAt:        time.Date(2026, 2, 23, 15, 12, 0, 0, time.UTC),
		FinishedAt:       time.Date(2026, 2, 23, 15, 14, 32, 0, time.UTC),
		Directories:      []string{"~/Documents"},
		Files:            []string{"Documents/report.pdf", "Documents/notes.md"},
		FilesTransferred: 2,
		BytesTransferred: 1048576,
		Status:           "success",
		Errors:           []string{},
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}

	var decoded JournalEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if !decoded.StartedAt.Equal(entry.StartedAt) {
		t.Errorf("StartedAt: got %v, want %v", decoded.StartedAt, entry.StartedAt)
	}
	if decoded.FilesTransferred != 2 {
		t.Errorf("FilesTransferred: got %d, want 2", decoded.FilesTransferred)
	}
	if decoded.Status != "success" {
		t.Errorf("Status: got %q, want %q", decoded.Status, "success")
	}
	if len(decoded.Files) != 2 {
		t.Errorf("Files: got %d items, want 2", len(decoded.Files))
	}
}
