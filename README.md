# Yad - Yet Another Downloader

A lightweight, web-based application for downloading files and torrents with real-time progress tracking.

![Yad - Yet Another Downloader](./image.png)

## Features

- **Multi-format Downloads**: Support for regular file downloads, torrent files, and magnet links
- **Real-time Progress Tracking**: Live updates via WebSockets
- **Concurrent Downloads**: Process multiple downloads simultaneously
- **Custom Output Locations**: Specify where your files should be saved
- **Simple Web Interface**: Easy-to-use UI accessible from any browser

## Installation

### Prerequisites

- Go 1.18 or higher
- Git
- Patience

### Installing from Source

1. Clone the repository:

```bash
git clone https://github.com/yourusername/yad.git
cd yad
```

2. Install dependencies:

```bash
go mod tidy
```

3. Run the application:

```bash
go run main.go
```

4. Open your web browser and navigate to:

```
http://localhost:8080
```

## Usage
1. Open your web browser and navigate to: `http://localhost:8080`

2. Enter URLs in the text area (one per line) and click "Add Download"

3. Monitor download progress in real-time

4. Access your downloaded files in the `downloads` directory or your specified output directory

## Technical Details

### API Endpoints

- `POST /api/download` - Add new downloads
- `GET /api/status` - Get current download status
- `WS /api/ws` - WebSocket endpoint for real-time updates

### Configuration

Default settings are defined in the source code:
- Downloads folder: `./downloads`
- Number of concurrent workers: 5
- Server port: 8080

## Accessing Downloaded Files

Downloaded files are stored in the `downloads` directory by default. You can:

1. Navigate to this directory using your file explorer
2. Specify a custom output directory in the web interface
3. Access the files directly from your file system

## Security Considerations

This application is designed for personal or internal use:
- No built-in authentication
- No encryption for stored files
- CORS restrictions are disabled

## License

[MIT License](LICENSE)

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.