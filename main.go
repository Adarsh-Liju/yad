package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"bufio"
	"strings"
	"github.com/cheggaaa/pb/v3"
)

func download(url, outputDir string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("error downloading %s: %w", url, err)
	}
	defer resp.Body.Close()

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


func main() {
	outputDir := flag.String("outputDir", "", "Output directory")
	urlsFile := flag.String("urls", "", "File containing URLs (one per line)")
	flag.Parse()

	// Check required flags.
	if *outputDir == "" || *urlsFile == "" {
		fmt.Println("Both -outputDir and -urls flags are required.")
		flag.Usage()
		os.Exit(1)
	}

	// Create the output directory if it doesn't exist.
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	// Open the file containing URLs.
	file, err := os.Open(*urlsFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening URLs file: %v\n", err)
		os.Exit(1)
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
		fmt.Fprintf(os.Stderr, "Error reading URLs file: %v\n", err)
		os.Exit(1)
	}

	// Download each URL with a progress bar.
	bar := pb.StartNew(len(urls))
	for _, u := range urls {
		bar.Increment()
		if err := download(u, *outputDir); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to download %s: %v\n", u, err)
		} else {
			fmt.Printf("Downloaded %s successfully\n", u)
		}
	}
	bar.Finish()
}
