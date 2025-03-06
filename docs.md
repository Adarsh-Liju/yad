# Yad - Backend Implementation

## Core Functionality

The application provides a web interface that enables users to:

1. Submit URLs for downloading (both regular files and torrents)
2. Track download progress in real-time
3. Specify custom output directories
4. Download multiple files concurrently

## Technical Components

### Main Server Components

- Uses the **Gorilla Mux** router for handling HTTP requests
- Implements **WebSocket** connections for real-time updates
- Supports both regular HTTP downloads and torrents (via the anacrolix/torrent library)
- Runs on port 8080 by default

### API Endpoints

- `/api/download` - POST endpoint to add new downloads
- `/api/status` - GET endpoint to retrieve current download status
- `/api/ws` - WebSocket endpoint for real-time updates
- `/` - Serves the main HTML interface

### Download Processing

- Uses a worker pool (5 concurrent workers by default) to process downloads
- Automatically detects if a URL is a regular file, magnet link, or torrent file
- Downloads are tracked in memory with statuses: queued, downloading, completed, or failed
- Progress is calculated and broadcast to all connected clients

### Data Storage

- Downloaded files are saved to the `./downloads` directory by default
- Users can specify alternative directories through the web interface
- The application automatically creates directories if they don't exist

### Real-time Updates

- Uses WebSockets to push download status updates to all connected clients
- Falls back to polling if WebSockets aren't available

## Technical Details

### Download Handling

- Regular file downloads track progress by counting bytes and comparing against Content-Length
- Torrent downloads leverage the anacrolix/torrent library and track piece completion
- Both methods provide real-time progress updates

### Concurrency

- Uses Go's goroutines and channels for concurrent processing
- Implements mutex locks to protect shared state
- Manages WebSocket connections in a thread-safe manner

### Error Handling

- Provides detailed error reporting
- Handles network failures, file system errors, and invalid URLs
- Has a 24-hour timeout for torrent downloads

## Implementation Notes

- The application is completely self-contained and doesn't require a database
- Download status is stored in memory and will be lost on server restart
- Completed downloads remain in the UI until the server restarts
- The application doesn't implement user authentication or download limits

This web application provides a straightforward way for users to download files and torrents through a browser interface, with all downloads managed and tracked centrally.
