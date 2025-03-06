package main

import (
	"encoding/json"
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
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

const (
	workers        = 5
	downloadFolder = "./downloads" // Default download location
)

type DownloadRequest struct {
	URLs      []string `json:"urls"`
	OutputDir string   `json:"outputDir"`
}

type DownloadStatus struct {
	URL       string  `json:"url"`
	Progress  float64 `json:"progress"`
	Status    string  `json:"status"`
	FileName  string  `json:"fileName"`
	Completed bool    `json:"completed"`
	Error     string  `json:"error,omitempty"`
}

var (
	activeDownloads = make(map[string]*DownloadStatus)
	downloadsMutex  sync.Mutex
	upgrader        = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for this example
		},
	}
	clients    = make(map[*websocket.Conn]bool)
	clientsMux sync.Mutex
)

func main() {
	// Create downloads directory if it doesn't exist
	if err := os.MkdirAll(downloadFolder, os.ModePerm); err != nil {
		log.Fatalf("Failed to create download directory: %v", err)
	}

	// Create router
	r := mux.NewRouter()

	// Serve static files
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	// API endpoints
	r.HandleFunc("/api/download", handleDownloadRequest).Methods("POST")
	r.HandleFunc("/api/status", handleGetAllStatus).Methods("GET")
	r.HandleFunc("/api/ws", handleWebSocket)

	// Serve index.html for the root path
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/index.html")
	})

	// Start server
	port := "8080"
	log.Printf("Starting server on port %s...", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func handleDownloadRequest(w http.ResponseWriter, r *http.Request) {
	var req DownloadRequest

	// Parse request
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate request
	if len(req.URLs) == 0 {
		http.Error(w, "No URLs provided", http.StatusBadRequest)
		return
	}

	// Use provided output directory or default
	outputDir := req.OutputDir
	if outputDir == "" {
		outputDir = downloadFolder
	}

	// Ensure directory exists
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		http.Error(w, fmt.Sprintf("Failed to create output directory: %v", err), http.StatusInternalServerError)
		return
	}

	// Filter empty URLs
	var validURLs []string
	for _, url := range req.URLs {
		url = strings.TrimSpace(url)
		if url != "" {
			validURLs = append(validURLs, url)
		}
	}

	// Start download process in background
	go processURLs(validURLs, outputDir)

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

func handleGetAllStatus(w http.ResponseWriter, r *http.Request) {
	downloadsMutex.Lock()
	defer downloadsMutex.Unlock()

	// Return all download statuses
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(activeDownloads)
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade the HTTP connection to a WebSocket connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade to WebSocket: %v", err)
		return
	}

	// Register client
	clientsMux.Lock()
	clients[conn] = true
	clientsMux.Unlock()

	// Send current status to new client
	downloadsMutex.Lock()
	statusJSON, _ := json.Marshal(activeDownloads)
	downloadsMutex.Unlock()
	conn.WriteMessage(websocket.TextMessage, statusJSON)

	// Listen for close events
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			clientsMux.Lock()
			delete(clients, conn)
			clientsMux.Unlock()
			break
		}
	}
}

func broadcastStatus() {
	downloadsMutex.Lock()
	statusJSON, _ := json.Marshal(activeDownloads)
	downloadsMutex.Unlock()

	clientsMux.Lock()
	for client := range clients {
		err := client.WriteMessage(websocket.TextMessage, statusJSON)
		if err != nil {
			client.Close()
			delete(clients, client)
		}
	}
	clientsMux.Unlock()
}

func processURLs(urls []string, outputDir string) {
	var wg sync.WaitGroup
	urlChan := make(chan string, len(urls))

	// Initialize download status for each URL
	for _, url := range urls {
		fileName := filepath.Base(url)
		if fileName == "" || fileName == "." || fileName == "/" {
			fileName = "downloaded_file"
		}

		downloadsMutex.Lock()
		activeDownloads[url] = &DownloadStatus{
			URL:       url,
			Progress:  0,
			Status:    "queued",
			FileName:  fileName,
			Completed: false,
		}
		downloadsMutex.Unlock()
	}

	// Broadcast initial status
	broadcastStatus()

	// Start worker goroutines
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for url := range urlChan {
				// Update status to "downloading"
				updateDownloadStatus(url, "downloading", 0, false, "")

				var err error
				if strings.HasPrefix(url, "magnet:") {
					// Handle torrent download
					err = downloadTorrent(url, outputDir)
				} else {
					// Handle HTTP download
					err = downloadFile(url, outputDir)
				}

				// Update final status
				if err != nil {
					log.Printf("Failed to download %s: %v", url, err)
					updateDownloadStatus(url, "failed", 0, true, err.Error())
				} else {
					log.Printf("Downloaded: %s", url)
					updateDownloadStatus(url, "completed", 100, true, "")
				}
			}
		}()
	}

	// Send URLs to workers
	for _, url := range urls {
		urlChan <- url
	}
	close(urlChan)

	// Wait for all downloads to complete
	wg.Wait()
}

func updateDownloadStatus(url, status string, progress float64, completed bool, errorMsg string) {
	downloadsMutex.Lock()
	if download, exists := activeDownloads[url]; exists {
		download.Status = status
		download.Progress = progress
		download.Completed = completed
		download.Error = errorMsg
	}
	downloadsMutex.Unlock()

	// Broadcast status update to all clients
	broadcastStatus()
}

func downloadFile(url string, outputDir string) error {
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

	// Set up progress tracking
	var downloaded int64
	progressChan := make(chan int64)

	// Start a goroutine to update progress
	go func() {
		for bytesDownloaded := range progressChan {
			downloaded = bytesDownloaded
			var progress float64
			if fileSize > 0 {
				progress = float64(downloaded) / float64(fileSize) * 100
			} else {
				progress = -1 // Unknown progress for unknown file size
			}
			updateDownloadStatus(url, "downloading", progress, false, "")
			time.Sleep(500 * time.Millisecond)
		}
	}()

	// Create a reader that reports progress
	reader := &progressReader{
		Reader:       resp.Body,
		BytesRead:    0,
		ProgressChan: progressChan,
	}

	// Download the file
	_, err = io.Copy(file, reader)
	close(progressChan)
	if err != nil {
		return fmt.Errorf("failed to save file: %v", err)
	}

	return nil
}

func downloadTorrent(magnetLink string, outputDir string) error {
	// Configure the torrent client
	clientConfig := torrent.NewDefaultClientConfig()
	clientConfig.DataDir = outputDir

	// Create a new torrent client
	client, err := torrent.NewClient(clientConfig)
	if err != nil {
		return fmt.Errorf("failed to create torrent client: %v", err)
	}
	defer client.Close()

	// Add the magnet link
	t, err := client.AddMagnet(magnetLink)
	if err != nil {
		return fmt.Errorf("failed to add magnet link: %v", err)
	}

	// Wait for metadata
	<-t.GotInfo()

	// Start downloading
	t.DownloadAll()

	// Monitor progress
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				// Calculate progress
				info := t.Info()
				if info != nil {
					totalLength := float64(info.TotalLength())
					if totalLength > 0 {
						progress := float64(t.BytesCompleted()) / totalLength * 100
						updateDownloadStatus(magnetLink, "downloading", progress, false, "")
					}
				}

				// Check if download is complete
				if t.Info() != nil && t.BytesCompleted() == t.Info().TotalLength() {
					close(done)
					return
				}

				time.Sleep(1 * time.Second)
			}
		}
	}()

	// Wait for completion or timeout
	select {
	case <-done:
		return nil
	case <-time.After(24 * time.Hour): // 24h timeout
		return fmt.Errorf("download timed out")
	}
}

// progressReader reports download progress
type progressReader struct {
	Reader       io.Reader
	BytesRead    int64
	ProgressChan chan int64
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.Reader.Read(p)
	pr.BytesRead += int64(n)
	pr.ProgressChan <- pr.BytesRead
	return n, err
}