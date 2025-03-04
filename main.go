package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const workers int = 5

func main() {
	// Create a new Fyne application
	myApp := app.New()
	myWindow := myApp.NewWindow("Yad - Yet Another Downloader")

	// Create UI elements
	outputDirEntry := widget.NewEntry()
	outputDirEntry.SetPlaceHolder("Select output directory...")

	urlsEntry := widget.NewMultiLineEntry()
	urlsEntry.SetPlaceHolder("Enter URLs or Magnet Links (one per line)...")

	// Button to open folder selection dialog
	selectDirButton := widget.NewButtonWithIcon("Select Directory", theme.FolderOpenIcon(), func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil {
				log.Println("Error selecting directory:", err)
				return
			}
			if uri != nil {
				outputDirEntry.SetText(uri.Path()) // Set the selected directory path
			}
		}, myWindow)
	})

	startButton := widget.NewButtonWithIcon("Start Download", theme.DownloadIcon(), func() {
		outputDir := outputDirEntry.Text
		urls := strings.Split(urlsEntry.Text, "\n") // Split URLs by newline
		if outputDir == "" || len(urls) == 0 {
			dialog.ShowInformation("Error", "Please provide both output directory and at least one URL/magnet link", myWindow)
			return
		}
		go startDownload(urls, outputDir, myWindow)
	})

	clearButton := widget.NewButtonWithIcon("Clear", theme.DeleteIcon(), func() {
		outputDirEntry.SetText("")
		urlsEntry.SetText("")
	})

	// Create a status bar
	statusBar := widget.NewLabel("Ready")

	// Create a vertical box layout for the UI
	content := container.NewVBox(
		widget.NewLabelWithStyle("Yad - Yet Another Downloader", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Output Directory:"),
		container.NewBorder(nil, nil, nil, selectDirButton, outputDirEntry),
		widget.NewLabel("URLs or Magnet Links (one per line):"),
		urlsEntry,
		container.NewHBox(startButton, clearButton),
		statusBar,
	)

	// Set the window content and show it
	myWindow.SetContent(content)
	myWindow.Resize(fyne.NewSize(1200, 800)) // Larger window to accommodate progress bars
	myWindow.ShowAndRun()
}

func startDownload(urls []string, outputDir string, window fyne.Window) {
	// Filter out empty URLs
	var validURLs []string
	for _, url := range urls {
		url = strings.TrimSpace(url)
		if url != "" {
			validURLs = append(validURLs, url)
		}
	}

	if len(validURLs) == 0 {
		dialog.ShowInformation("Error", "No valid URLs/magnet links provided", window)
		return
	}

	log.Println("Starting download...")

	// Create a container to hold progress bars
	progressContainer := container.NewVBox()

	// Add progress bars to the window
	window.SetContent(container.NewVBox(
		widget.NewLabelWithStyle("Download Progress:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		progressContainer,
	))

	// Process URLs with progress bars
	go processURLs(validURLs, outputDir, progressContainer)
}

func processURLs(urls []string, outputDir string, progressContainer *fyne.Container) {
	var wg sync.WaitGroup
	urlChan := make(chan string, len(urls))

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for url := range urlChan {
				// Create a progress bar for this URL
				progressBar := widget.NewProgressBar()
				label := widget.NewLabel(url)
				progressContainer.Add(container.NewVBox(label, progressBar))

				// Check if the URL is a magnet link or HTTP URL
				if strings.HasPrefix(url, "magnet:") {
					// Handle magnet link (torrent download)
					err := downloadTorrent(url, outputDir, progressBar)
					if err != nil {
						log.Printf("Failed to download torrent %s: %v\n", url, err)
					} else {
						log.Printf("Downloaded torrent: %s\n", url)
					}
				} else {
					// Handle HTTP download
					err := downloadFile(url, outputDir, progressBar)
					if err != nil {
						log.Printf("Failed to download %s: %v\n", url, err)
					} else {
						log.Printf("Downloaded: %s\n", url)
					}
				}
			}
		}()
	}

	for _, url := range urls {
		urlChan <- url
	}
	close(urlChan)
	wg.Wait()
}

func downloadFile(url string, outputDir string, progressBar *widget.ProgressBar) error {
	// Create the output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	// Get the file name from the URL
	fileName := filepath.Base(url)
	if fileName == "" || fileName == "." || fileName == "/" {
		fileName = "downloaded_file"
	}

	// Create the output file
	outputPath := filepath.Join(outputDir, fileName)
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()

	// Start the HTTP request
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to start download: %v", err)
	}
	defer resp.Body.Close()

	// Check if the response is successful
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download: %s", resp.Status)
	}

	// Get the total file size
	fileSize := resp.ContentLength
	progressBar.Max = float64(fileSize)

	// Download the file and update the progress bar
	reader := io.TeeReader(resp.Body, &progressWriter{progressBar: progressBar})
	_, err = io.Copy(file, reader)
	if err != nil {
		return fmt.Errorf("failed to save file: %v", err)
	}

	return nil
}

func downloadTorrent(magnetLink string, outputDir string, progressBar *widget.ProgressBar) error {
	// Configure the torrent client with the output directory
	clientConfig := torrent.NewDefaultClientConfig()
	clientConfig.DataDir = outputDir // Set the download directory

	// Create a new torrent client
	client, err := torrent.NewClient(clientConfig)
	if err != nil {
		return fmt.Errorf("failed to create torrent client: %v", err)
	}
	defer client.Close()

	// Add the magnet link to the client
	torrent, err := client.AddMagnet(magnetLink)
	if err != nil {
		return fmt.Errorf("failed to add magnet link: %v", err)
	}

	// Wait for the torrent metadata to be available
	<-torrent.GotInfo()

	// Start downloading the torrent
	torrent.DownloadAll()

	// Monitor download progress
	for {
		progress := float64(torrent.BytesCompleted()) / float64(torrent.Info().TotalLength())
		progressBar.SetValue(progress)
		progressBar.Refresh()

		if torrent.BytesCompleted() == torrent.Info().TotalLength() {
			break // Download complete
		}

		time.Sleep(500 * time.Millisecond) // Update progress every 500ms
	}

	return nil
}

// progressWriter updates the progress bar as data is written
type progressWriter struct {
	progressBar *widget.ProgressBar
	written     int64
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.written += int64(n)
	pw.progressBar.SetValue(float64(pw.written))
	pw.progressBar.Refresh()
	return n, nil
}