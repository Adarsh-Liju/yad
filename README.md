# YAD

## Overview

The `yad` is a Go program that enables efficient downloading of multiple files simultaneously with built-in rate limiting, retry mechanisms, and progress tracking. This documentation covers the program's architecture, components, and usage.

## Installation

To install and run yad:

1. Install directly using Go:

```bash
go install github.com/Adarsh-Liju/yad@latest
```

2. Create a text file containing URLs to download (one per line), for example `urls.txt`:

```
https://example.com/file1.pdf
https://example.com/file2.pdf
```

3. Run the program:

```bash
yad -u urls.txt -o output_directory
```

Where:
- `-u`: Path to the file containing URLs
- `-o`: Directory where downloaded files will be saved

The program will create the output directory if it doesn't exist and begin downloading the files in parallel using 5 concurrent workers.

## Implementation

The program works by:

1. Reading the URLs from the input file
2. Creating a pool of worker goroutines
3. Distributing the URLs among the workers
4. Downloading the files in parallel
5. Saving the files to the output directory

## Future Enhancements

The following features are planned for future releases:

1. Progress bars for large downloads
   - Visual download progress tracking
   - Estimated time remaining
   - Download speed indicators

2. Enhanced parallelization
   - Configurable number of concurrent downloads
   - Improved resource utilization
   - Better handling of system limitations

3. Customization options
   - Configurable retry attempts and timeouts
   - Custom rate limiting thresholds
   - Output filename templates
   - Download filters and rules

4. Documentation improvements
   - API documentation
   - Configuration guide
   - Best practices
   - Performance tuning tips

Contributions and feature requests are welcome! Please feel free to open issues or submit pull requests on GitHub.
