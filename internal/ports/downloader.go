package ports

import "github.com/bnema/bnetctl/internal/domain"

// DownloaderPort defines operations for downloading files
type DownloaderPort interface {
	// Download fetches a URL to a local file path, reporting progress via callback
	Download(url string, destPath string, progressFn func(domain.DownloadProgress)) error
}
