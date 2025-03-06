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
	downloadFolder = "./downloads"
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
	clients         = make(map[*websocket.Conn]bool)
	clientsMux      sync.Mutex
	upgrader        = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

func main() {
	// Create downloads directory if it doesn't exist
	if err := os.MkdirAll(downloadFolder, os.ModePerm); err != nil {
		log.Fatalf("Failed to create download directory: %v", err)
	}

	// Create router
	r := mux.NewRouter()

	// API endpoints
	r.HandleFunc("/api/download", handleDownloadRequest).Methods("POST")
	r.HandleFunc("/api/status", handleGetAllStatus).Methods("GET")
	r.HandleFunc("/api/ws", handleWebSocket)

	// Serve static files
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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

	// Start download process in background
	go processURLs(req.URLs, outputDir)

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

func handleGetAllStatus(w http.ResponseWriter, r *http.Request) {
	downloadsMutex.Lock()
	defer downloadsMutex.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(activeDownloads)
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade to WebSocket: %v", err)
		return
	}

	clientsMux.Lock()
	clients[conn] = true
	clientsMux.Unlock()

	downloadsMutex.Lock()
	statusJSON, _ := json.Marshal(activeDownloads)
	downloadsMutex.Unlock()
	conn.WriteMessage(websocket.TextMessage, statusJSON)

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

	broadcastStatus()

	// Start worker goroutines
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for url := range urlChan {
				updateDownloadStatus(url, "downloading", 0, false, "")

				var err error
				// Check if the URL is a magnet link or torrent file
				if strings.HasPrefix(url, "magnet:") || strings.HasSuffix(url, ".torrent") {
					err = downloadTorrent(url, outputDir)
				} else {
					err = downloadFile(url, outputDir)
				}

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

	for _, url := range urls {
		urlChan <- url
	}
	close(urlChan)
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
	broadcastStatus()
}

func broadcastStatus() {
	downloadsMutex.Lock()
	statusJSON, _ := json.Marshal(activeDownloads)
	downloadsMutex.Unlock()

	clientsMux.Lock()
	for client := range clients {
		if err := client.WriteMessage(websocket.TextMessage, statusJSON); err != nil {
			client.Close()
			delete(clients, client)
		}
	}
	clientsMux.Unlock()
}

func downloadFile(url, outputDir string) error {
	fileName := filepath.Base(url)
	if fileName == "" || fileName == "." || fileName == "/" {
		fileName = "downloaded_file"
	}
	outputPath := filepath.Join(outputDir, fileName)
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to start download: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download: %s", resp.Status)
	}

	fileSize := resp.ContentLength
	var downloaded int64
	progressChan := make(chan int64)

	go func() {
		for bytesDownloaded := range progressChan {
			downloaded = bytesDownloaded
			var prog float64
			if fileSize > 0 {
				prog = float64(downloaded) / float64(fileSize) * 100
			} else {
				prog = -1
			}
			updateDownloadStatus(url, "downloading", prog, false, "")
			time.Sleep(500 * time.Millisecond)
		}
	}()

	reader := &progressReader{
		Reader:       resp.Body,
		BytesRead:    0,
		ProgressChan: progressChan,
	}

	_, err = io.Copy(file, reader)
	close(progressChan)
	if err != nil {
		return fmt.Errorf("failed to save file: %v", err)
	}
	return nil
}

func downloadTorrent(link, outputDir string) error {
	clientConfig := torrent.NewDefaultClientConfig()
	clientConfig.DataDir = outputDir
	client, err := torrent.NewClient(clientConfig)
	if err != nil {
		return fmt.Errorf("failed to create torrent client: %v", err)
	}
	defer client.Close()

	var t *torrent.Torrent
	if strings.HasPrefix(link, "magnet:") {
		t, err = client.AddMagnet(link)
		if err != nil {
			return fmt.Errorf("failed to add magnet link: %v", err)
		}
	} else if strings.HasSuffix(link, ".torrent") {
		// Download the torrent file to a temporary location
		tmpFile, err := os.CreateTemp("", "*.torrent")
		if err != nil {
			return fmt.Errorf("failed to create temporary torrent file: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		resp, err := http.Get(link)
		if err != nil {
			return fmt.Errorf("failed to download torrent file: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to download torrent file: %s", resp.Status)
		}
		if _, err := io.Copy(tmpFile, resp.Body); err != nil {
			return fmt.Errorf("failed to save torrent file: %v", err)
		}
		tmpFile.Close()
		t, err = client.AddTorrentFromFile(tmpFile.Name())
		if err != nil {
			return fmt.Errorf("failed to add torrent from file: %v", err)
		}
	} else {
		return fmt.Errorf("unsupported torrent link format")
	}

	<-t.GotInfo()
	t.DownloadAll()

	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				info := t.Info()
				if info != nil {
					totalLength := float64(info.TotalLength())
					if totalLength > 0 {
						prog := float64(t.BytesCompleted()) / totalLength * 100
						updateDownloadStatus(link, "downloading", prog, false, "")
					}
				}
				if info != nil && t.BytesCompleted() == info.TotalLength() {
					close(done)
					return
				}
				time.Sleep(1 * time.Second)
			}
		}
	}()
	select {
	case <-done:
		return nil
	case <-time.After(24 * time.Hour):
		return fmt.Errorf("download timed out")
	}
}

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
