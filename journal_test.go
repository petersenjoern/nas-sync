package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestLoadJournalMissingFile(t *testing.T) {
	entries, err := loadJournal("/tmp/nas-sync-test-nonexistent/journal.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty journal, got %d entries", len(entries))
	}
}

func TestAppendAndLoadJournal(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/journal.json"

	entry := JournalEntry{
		StartedAt:        time.Now().UTC(),
		FinishedAt:       time.Now().UTC(),
		Directories:      []string{"~/Documents"},
		Files:            []string{"file1.txt"},
		FilesTransferred: 1,
		BytesTransferred: 100,
		Status:           "success",
		Errors:           []string{},
	}

	if err := appendJournal(path, entry); err != nil {
		t.Fatal(err)
	}

	entries, err := loadJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].FilesTransferred != 1 {
		t.Errorf("FilesTransferred: got %d, want 1", entries[0].FilesTransferred)
	}
}

func TestJournalRotation(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/journal.json"

	for i := 0; i < 105; i++ {
		entry := JournalEntry{
			StartedAt:        time.Now().UTC(),
			FinishedAt:       time.Now().UTC(),
			Directories:      []string{"~/Documents"},
			FilesTransferred: i,
			Status:           "success",
			Errors:           []string{},
		}
		if err := appendJournal(path, entry); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := loadJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 100 {
		t.Errorf("expected 100 entries after rotation, got %d", len(entries))
	}
	// Oldest should have been rotated out (0-4 gone, 5-104 remain)
	if entries[0].FilesTransferred != 5 {
		t.Errorf("oldest entry FilesTransferred: got %d, want 5", entries[0].FilesTransferred)
	}
}

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

func TestParseRsyncFiles(t *testing.T) {
	output := `sending incremental file list
Documents/report.pdf
Documents/notes/meeting.md
Documents/photos/cat.jpg

sent 1,234 bytes  received 56 bytes  2,580.00 bytes/sec
total size is 1,048,576  speedup is 812.94
`
	files := parseRsyncFiles(output)
	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d: %v", len(files), files)
	}
	expected := []string{
		"Documents/report.pdf",
		"Documents/notes/meeting.md",
		"Documents/photos/cat.jpg",
	}
	for i, f := range files {
		if f != expected[i] {
			t.Errorf("file[%d]: got %q, want %q", i, f, expected[i])
		}
	}
}

func TestParseRsyncFilesEmpty(t *testing.T) {
	output := `sending incremental file list

sent 100 bytes  received 12 bytes  224.00 bytes/sec
total size is 0  speedup is 0.00
`
	files := parseRsyncFiles(output)
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d: %v", len(files), files)
	}
}

func TestParseRsyncBytes(t *testing.T) {
	output := `Number of files: 50
Number of files transferred: 3
Total file size: 10,485,760 bytes
Total transferred file size: 1,048,576 bytes
`
	bytes := parseRsyncBytes(output)
	if bytes != 1048576 {
		t.Errorf("expected 1048576 bytes, got %d", bytes)
	}
}

func TestParseRsyncBytesNoMatch(t *testing.T) {
	bytes := parseRsyncBytes("no stats here")
	if bytes != 0 {
		t.Errorf("expected 0 bytes, got %d", bytes)
	}
}
