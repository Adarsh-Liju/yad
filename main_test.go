package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadSuccess(t *testing.T) {
	// Create a test HTTP server that returns a fixed response.
	expectedContent := "Hello, World!"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, expectedContent)
	}))
	defer server.Close()

	// Create a temporary directory for downloads.
	tempDir := t.TempDir()

	// Call download with our test server URL.
	if err := download(server.URL, tempDir); err != nil {
		t.Fatalf("download() returned error: %v", err)
	}

	// The download function uses filepath.Base(url) for the filename.
	filename := filepath.Base(server.URL)
	downloadedPath := filepath.Join(tempDir, filename)

	// Read the downloaded file.
	data, err := os.ReadFile(downloadedPath)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}

	// Compare file content with the expected content.
	if string(data) != expectedContent {
		t.Errorf("downloaded content mismatch: got %q, want %q", string(data), expectedContent)
	}
}

func TestDownloadInvalidURL(t *testing.T) {
	// Create a temporary directory.
	tempDir := t.TempDir()

	// Provide an invalid URL.
	invalidURL := "http://[::1]:NamedPort"

	// Expect an error.
	err := download(invalidURL, tempDir)
	if err == nil {
		t.Error("expected error for invalid URL, got nil")
	}
}
