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

const maxWorkers = 5

func download(url, outputDir string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("error downloading %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error downloading %s: received status code %d", url, resp.StatusCode)
	}

	filePath := filepath.Join(outputDir, filepath.Base(url))
	out, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}
	defer out.Close()

	if _, err = io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("error saving file: %w", err)
	}
	return nil
}

func worker(urls <-chan string, outputDir string, wg *sync.WaitGroup) {
	defer wg.Done()
	for url := range urls {
		if err := download(url, outputDir); err != nil {
			log.Printf("Failed to download %s: %v\n", url, err)
		} else {
			log.Printf("Downloaded %s successfully\n", url)
		}
	}
}

func main() {
	outputDir := flag.String("o", "", "Output directory")
	urlsFile := flag.String("u", "", "File containing URLs (one per line)")
	flag.Parse()

	// Check required flags.
	if *outputDir == "" || *urlsFile == "" {
		log.Println("Both -o and -u flags are required.")
		flag.Usage()
		os.Exit(1)
	}

	// Create the output directory if it doesn't exist.
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("Error creating output directory: %v\n", err)
	}

	// Open the file containing URLs.
	file, err := os.Open(*urlsFile)
	if err != nil {
		log.Fatalf("Error opening URLs file: %v\n", err)
	}
	defer file.Close()

	// Read the file line by line to build the URL list.
	scanner := bufio.NewScanner(file)
	var urls []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			urls = append(urls, line)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading URLs file: %v\n", err)
	}

	// Create a channel to distribute URLs to workers.
	urlChan := make(chan string, len(urls))
	log.Println("Starting workers...")
	var wg sync.WaitGroup

	// Start worker goroutines.
	for range maxWorkers {
		wg.Add(1)
		go worker(urlChan, *outputDir, &wg)
	}

	// Send URLs to the channel.
	for _, url := range urls {
		urlChan <- url
	}
	close(urlChan)

	// Wait for all workers to finish.
	wg.Wait()
	log.Println("All downloads completed.")
}
