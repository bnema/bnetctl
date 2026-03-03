package ports

// DownloadProgress reports download progress
type DownloadProgress struct {
	BytesDownloaded int64
	TotalBytes      int64
	Percent         float64
}

// DownloaderPort defines operations for downloading files
type DownloaderPort interface {
	// Download fetches a URL to a local file path, reporting progress via callback
	Download(url string, destPath string, progressFn func(DownloadProgress)) error
}
