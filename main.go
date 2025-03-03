package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"github.com/anacrolix/torrent"
	"time"
)

const workers int = 5

func downloadTorrent(url, outputDir string) error {
	// Configure the torrent client with reasonable defaults
	config := torrent.NewDefaultClientConfig()
	config.DataDir = outputDir
	config.Seed = false // Don't seed after download completes
	config.NoUpload = true // Don't upload while downloading

	client, err := torrent.NewClient(config)
	if err != nil {
		return fmt.Errorf("failed to create torrent client: %w", err)
	}
	defer client.Close()

	t, err := client.AddMagnet(url)
	if err != nil {
		return fmt.Errorf("failed to add magnet URL: %w", err)
	}

	// Set a reasonable timeout for getting torrent info
	infoTimeout := 1 * time.Minute
	select {
	case <-t.GotInfo():
	case <-time.After(infoTimeout):
		return fmt.Errorf("timeout waiting for torrent info after %v", infoTimeout)
	}

	t.DownloadAll()

	// Add progress reporting and better completion check
	downloadStart := time.Now()
	for !t.Complete().Bool() {
		stats := t.Stats()
		progress := float64(stats.BytesRead.Int64()) / float64(t.Length()) * 100
		speed := float64(stats.BytesRead.Int64()) / time.Since(downloadStart).Seconds() / 1024 / 1024 // MB/s

		log.Printf("Downloading torrent: %.1f%% complete (%.2f MB/s)", progress, speed)

		// Check if download is stuck
		if stats.ActivePeers == 0 {
			return fmt.Errorf("download stalled - no active peers")
		}

		time.Sleep(2 * time.Second)
	}

	log.Printf("Torrent download completed in %v", time.Since(downloadStart))
	return nil
}

func normalDownload(url, outputDir string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received status code %d for %s", resp.StatusCode, url)
	}

	outPath := filepath.Join(outputDir, filepath.Base(url))
	out, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", outPath, err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func processURLs(urls []string, outputDir string) {
	var wg sync.WaitGroup
	urlChan := make(chan string, len(urls))

	// Start workers
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for url := range urlChan {
				// if url is torrent magnet send it to other function
				if strings.HasPrefix(url, "magnet:") {
					if err := downloadTorrent(url, outputDir); err != nil {
						log.Printf("Error downloading torrent: %v", err)
					}
					continue
				}
				if err := normalDownload(url, outputDir); err != nil {
					log.Printf("Error: %v", err)
				} else {
					log.Printf("Successfully downloaded %s", url)
				}
			}
		}()
	}

	// Feed URLs to workers
	for _, url := range urls {
		urlChan <- url
	}
	close(urlChan)

	wg.Wait()
}

func main() {
	outputDir := flag.String("o", "", "Output directory")
	urlsFile := flag.String("u", "", "File containing URLs (one per line)")
	flag.Parse()

	if *outputDir == "" || *urlsFile == "" {
		log.Fatal("Both -o and -u flags are required")
	}

	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	file, err := os.Open(*urlsFile)
	if err != nil {
		log.Fatalf("Failed to open URLs file: %v", err)
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
		log.Fatalf("Failed to read URLs file: %v", err)
	}

	log.Println("Starting download...")
	processURLs(urls, *outputDir)
	log.Println("All downloads completed")
}
