package main

import "time"

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
