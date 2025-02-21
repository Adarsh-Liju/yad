# YAD

## Overview

The Concurrent File Downloader is a Go program that enables efficient downloading of multiple files simultaneously with built-in rate limiting, retry mechanisms, and progress tracking. This documentation covers the program's architecture, components, and usage.

## Features

- Concurrent downloads with configurable parallelism
- Rate limiting to prevent server overload
- Automatic retries for failed downloads
- Progress tracking and reporting
- File integrity verification via SHA256 checksums
- Safe file name handling
- Detailed error reporting and statistics

## Core Components

### Downloader Struct

```go
type Downloader struct {
    client      *http.Client
    limiter     *rate.Limiter
    outputDir   string
    maxRetries  int
    results     chan DownloadResult
    progress    *Progress
}
```

The Downloader struct is the main component that handles download operations. It contains:

- `client`: HTTP client with configured timeout
- `limiter`: Rate limiter to control download frequency
- `outputDir`: Directory where downloaded files are saved
- `maxRetries`: Maximum number of retry attempts for failed downloads
- `results`: Channel for communicating download results
- `progress`: Tracks overall download progress

### DownloadResult Struct

```go
type DownloadResult struct {
    URL      string
    Filename string
    Success  bool
    Error    error
    Hash     string
}
```

DownloadResult stores the outcome of each download attempt:

- `URL`: Source URL of the file
- `Filename`: Local path where the file is saved
- `Success`: Boolean indicating download success
- `Error`: Error information if download failed
- `Hash`: SHA256 checksum of the downloaded file

## Key Functions

### NewDownloader

```go
func NewDownloader(outputDir string, rateLimit float64, maxRetries int) (*Downloader, error)
```

Creates a new Downloader instance with specified configuration:

- `outputDir`: Directory for downloaded files
- `rateLimit`: Maximum downloads per second
- `maxRetries`: Number of retry attempts for failed downloads

Returns an error if the output directory cannot be created.

### downloadFile

```go
func (d *Downloader) downloadFile(ctx context.Context, url string) DownloadResult
```

Downloads a single file with retry logic:

1. Generates safe filename from URL
2. Attempts download with configured retries
3. Verifies file integrity
4. Returns download result

### tryDownload

```go
func (d *Downloader) tryDownload(ctx context.Context, url, filename string) (string, error)
```

Performs a single download attempt:

1. Creates HTTP request with context
2. Downloads file content
3. Calculates SHA256 hash
4. Saves file to disk
5. Returns file hash or error

## Configuration

Default configuration in main():

```go
const (
    outputDir   = "./downloads"    // Output directory
    rateLimit   = 5.0             // Downloads per second
    maxRetries  = 3               // Maximum retry attempts
    maxParallel = 5              // Maximum concurrent downloads
    urlFile     = "urls.txt"      // Input file containing URLs
)
```

## Usage

1. Create a text file named `urls.txt` containing URLs to download (one per line)
2. Run the program:

   ```bash
   go run main.go
   ```

3. The program will:
   - Create output directory if it doesn't exist
   - Read URLs from urls.txt
   - Download files concurrently
   - Display progress and results
   - Show final summary

## Error Handling

The program handles various error scenarios:

- Invalid URLs
- Network failures
- File system errors
- Rate limiting
- Context cancellation

Each error is logged with relevant details and retry attempts are made when appropriate.

## Progress Tracking

Download progress is displayed in real-time:

- Current progress (completed/total)
- Success/failure for each download
- File checksums
- Final summary statistics

## Example Output

```sh
Progress: 3/5 completed
Successfully downloaded: https://example.com/file1.pdf -> downloads/file1.pdf (SHA256: abc123...)
Failed to download https://example.com/file2.pdf: connection timeout
Successfully downloaded: https://example.com/file3.pdf -> downloads/file3.pdf (SHA256: def456...)

Download summary:
Total: 5
Successful: 4
Failed: 1
```

## Best Practices

1. **Rate Limiting**: Adjust `rateLimit` based on server capabilities and requirements
2. **Concurrent Downloads**: Modify `maxParallel` based on available system resources
3. **Retry Attempts**: Configure `maxRetries` based on network reliability
4. **File Organization**: Use meaningful URL paths to generate clear filenames
5. **Error Handling**: Check error messages and logs for troubleshooting

## Limitations

- Only supports HTTP/HTTPS downloads
- No support for authentication
- No resume support for partial downloads
- Limited to file system storage

## Future Improvements

Potential enhancements:

1. Resume interrupted downloads
2. Support for authentication
3. Custom HTTP headers
4. Bandwidth limiting
5. Multiple storage backends
6. File type validation
7. Progress persistence
8. WebSocket status updates

## Safety Considerations

The program implements several safety measures:

- File name sanitization
- Checksum verification
- Partial file cleanup
- Rate limiting
- Context cancellation
- Resource cleanup

## Troubleshooting

Common issues and solutions:

1. **Permission Errors**: Ensure write access to output directory
2. **Memory Issues**: Reduce maxParallel value
3. **Timeout Errors**: Increase HTTP client timeout
4. **Rate Limiting**: Adjust rateLimit value
5. **Failed Downloads**: Check network connection and URL validity
