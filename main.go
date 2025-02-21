package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// DownloadResult stores the result of a download operation
type DownloadResult struct {
	URL      string
	Filename string
	Success  bool
	Error    error
	Hash     string
}

// Downloader handles concurrent file downloads with rate limiting
type Downloader struct {
	client      *http.Client
	limiter     *rate.Limiter
	outputDir   string
	maxRetries  int
	results     chan DownloadResult
	progress    *Progress
}

// Progress tracks download progress
type Progress struct {
	total     int
	completed int
	mu        sync.Mutex
}

// NewDownloader creates a new Downloader instance
func NewDownloader(outputDir string, rateLimit float64, maxRetries int) (*Downloader, error) {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	return &Downloader{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		limiter: rate.NewLimiter(rate.Limit(rateLimit), 1),
		outputDir: outputDir,
		maxRetries: maxRetries,
		results: make(chan DownloadResult, 100),
		progress: &Progress{},
	}, nil
}

// downloadFile downloads a single file with retries
func (d *Downloader) downloadFile(ctx context.Context, url string) DownloadResult {
	var result DownloadResult
	result.URL = url

	// Generate safe filename from URL
	filename := path.Base(url)
	if filename == "" || filename == "/" || filename == "." {
		filename = fmt.Sprintf("downloaded_file_%x", sha256.Sum256([]byte(url)))
	}
	result.Filename = filepath.Join(d.outputDir, sanitizeFilename(filename))

	// Try download with retries
	for attempt := 0; attempt <= d.maxRetries; attempt++ {
		if err := d.limiter.Wait(ctx); err != nil {
			result.Error = fmt.Errorf("rate limit wait failed: %w", err)
			return result
		}

		if hash, err := d.tryDownload(ctx, url, result.Filename); err == nil {
			result.Success = true
			result.Hash = hash
			return result
		} else if attempt == d.maxRetries {
			result.Error = fmt.Errorf("failed after %d attempts: %w", d.maxRetries, err)
			return result
		}

		// Wait before retry
		select {
		case <-ctx.Done():
			result.Error = ctx.Err()
			return result
		case <-time.After(time.Duration(attempt+1) * time.Second):
		}
	}

	return result
}

// tryDownload attempts a single download
func (d *Downloader) tryDownload(ctx context.Context, url, filename string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	out, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer out.Close()

	hash := sha256.New()
	writer := io.MultiWriter(out, hash)

	if _, err := io.Copy(writer, resp.Body); err != nil {
		os.Remove(filename) // Clean up partial file
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// sanitizeFilename removes potentially dangerous characters from filename
func sanitizeFilename(filename string) string {
	// Replace unsafe characters
	unsafe := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	replacements := make([]string, 0, len(unsafe)*2)
	for _, char := range unsafe {
		replacements = append(replacements, char, "_")
	}
	safe := strings.NewReplacer(replacements...)
	return safe.Replace(filename)
}

// readURLs reads URLs from a text file
func readURLs(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open URL file: %w", err)
	}
	defer file.Close()

	var urls []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if url := strings.TrimSpace(scanner.Text()); url != "" {
			urls = append(urls, url)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading URLs: %w", err)
	}

	return urls, nil
}

func main() {
	// Configuration
	const (
		outputDir   = "./downloads"
		rateLimit   = 5.0 // downloads per second
		maxRetries  = 3
		maxParallel = 5
		urlFile     = "urls.txt"
	)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize downloader
	downloader, err := NewDownloader(outputDir, rateLimit, maxRetries)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize downloader: %v\n", err)
		os.Exit(1)
	}

	// Read URLs
	urls, err := readURLs(urlFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading URLs: %v\n", err)
		os.Exit(1)
	}

	if len(urls) == 0 {
		fmt.Println("No URLs to download")
		return
	}

	// Start downloads
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, maxParallel)
	downloader.progress.total = len(urls)

	for _, url := range urls {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			semaphore <- struct{}{} // Acquire
			defer func() { <-semaphore }() // Release

			result := downloader.downloadFile(ctx, url)
			downloader.results <- result

			downloader.progress.mu.Lock()
			downloader.progress.completed++
			fmt.Printf("\rProgress: %d/%d completed", downloader.progress.completed, downloader.progress.total)
			downloader.progress.mu.Unlock()
		}(url)
	}

	// Wait for completion in a separate goroutine
	go func() {
		wg.Wait()
		close(downloader.results)
	}()

	// Process results
	successful := 0
	failed := 0

	for result := range downloader.results {
		if result.Success {
			successful++
			fmt.Printf("\nSuccessfully downloaded: %s -> %s (SHA256: %s)", result.URL, result.Filename, result.Hash)
		} else {
			failed++
			fmt.Printf("\nFailed to download %s: %v", result.URL, result.Error)
		}
	}

	fmt.Printf("\n\nDownload summary:\n")
	fmt.Printf("Total: %d\n", len(urls))
	fmt.Printf("Successful: %d\n", successful)
	fmt.Printf("Failed: %d\n", failed)
}