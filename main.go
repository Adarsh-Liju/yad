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
)

const workers int = 5

func download(url, outputDir string) error {
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
				if err := download(url, outputDir); err != nil {
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
