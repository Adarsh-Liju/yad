package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
)

// downloadFile downloads a file from the given URL and saves it with the given filename
func downloadFile(url string, wg *sync.WaitGroup) {
	defer wg.Done()

	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("Failed to download %s: %v\n", url, err)
		return
	}
	defer resp.Body.Close()

	// Extract filename from URL
	filename := path.Base(url)
	if filename == "" || filename == "/" || filename == "." {
		filename = "downloaded_file"
	}

	// Create the file
	out, err := os.Create(filename)
	if err != nil {
		fmt.Printf("Failed to create file %s: %v\n", filename, err)
		return
	}
	defer out.Close()

	// Write the file content
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		fmt.Printf("Failed to save file %s: %v\n", filename, err)
		return
	}

	fmt.Printf("Downloaded: %s -> %s\n", url, filename)
}

// readURLs reads URLs from a text file and returns a slice of URLs
func readURLs(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var urls []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		url := strings.TrimSpace(scanner.Text())
		if url != "" {
			urls = append(urls, url)
		}
	}
	return urls, scanner.Err()
}

func main() {
	urls, err := readURLs("urls.txt")
	if err != nil {
		fmt.Println("Error reading URLs:", err)
		return
	}

	var wg sync.WaitGroup
	for _, url := range urls {
		wg.Add(1)
		go downloadFile(url, &wg)
	}

	wg.Wait()
	fmt.Println("All downloads complete!")
}
