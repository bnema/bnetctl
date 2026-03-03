package http

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bnema/bnetctl/internal/ports"
)

// Downloader implements ports.DownloaderPort using net/http
type Downloader struct {
	client *http.Client
}

var _ ports.DownloaderPort = (*Downloader)(nil)

// NewDownloader creates a new HTTP downloader
func NewDownloader() *Downloader {
	return &Downloader{
		client: &http.Client{},
	}
}

// Download fetches a URL to a local file, reporting progress
func (d *Downloader) Download(url string, destPath string, progressFn func(ports.DownloadProgress)) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("create download dir: %w", err)
	}

	resp, err := d.client.Get(url)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer out.Close()

	totalBytes := resp.ContentLength

	if progressFn == nil {
		_, err = io.Copy(out, resp.Body)
		return err
	}

	// Copy with progress reporting
	buf := make([]byte, 32*1024)
	var downloaded int64

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			_, writeErr := out.Write(buf[:n])
			if writeErr != nil {
				return fmt.Errorf("write file: %w", writeErr)
			}
			downloaded += int64(n)

			var percent float64
			if totalBytes > 0 {
				percent = float64(downloaded) / float64(totalBytes) * 100
			}
			progressFn(ports.DownloadProgress{
				BytesDownloaded: downloaded,
				TotalBytes:      totalBytes,
				Percent:         percent,
			})
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return fmt.Errorf("read response: %w", readErr)
		}
	}

	return nil
}
