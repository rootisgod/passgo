// mcp_install.go - Auto-detect and download multipass-mcp binary
package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	mcpGitHubRepo = "rootisgod/multipass-mcp"
	mcpBinaryName = "multipass-mcp"
)

// findMCPBinary locates the multipass-mcp binary.
// Search order: configured path, ~/.passgo/, $PATH.
// Validates that the file is actually executable, not a stale archive.
func findMCPBinary(configPath string) string {
	// 1. Configured path
	if configPath != "" {
		if isExecutableBinary(configPath) {
			return configPath
		}
	}

	// 2. ~/.passgo/multipass-mcp
	home, err := os.UserHomeDir()
	if err == nil {
		name := mcpBinaryName
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		localPath := filepath.Join(home, ".passgo", name)
		if isExecutableBinary(localPath) {
			return localPath
		}
	}

	// 3. $PATH
	if path, err := exec.LookPath(mcpBinaryName); err == nil {
		return path
	}

	return ""
}

// isExecutableBinary checks that a file exists and is not a gzip/zip archive
// (which would indicate a failed extraction from a previous download).
func isExecutableBinary(path string) bool {
	f, err := os.Open(path) // #nosec G304 -- path from user config or known location
	if err != nil {
		return false
	}
	defer f.Close()

	// Read magic bytes
	magic := make([]byte, 4)
	n, err := f.Read(magic)
	if err != nil || n < 2 {
		return false
	}

	// Reject gzip (1f 8b) and zip (50 4b) archives — these are unextracted downloads
	if magic[0] == 0x1f && magic[1] == 0x8b {
		return false
	}
	if magic[0] == 0x50 && magic[1] == 0x4b {
		return false
	}

	return true
}

// githubRelease represents a GitHub release.
type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

// githubAsset represents a release asset.
type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// downloadMCPBinary downloads the multipass-mcp binary from GitHub releases.
// Returns the path to the downloaded binary. Calls progressFn with status updates.
func downloadMCPBinary(progressFn func(string)) (string, error) {
	if progressFn == nil {
		progressFn = func(string) {}
	}

	progressFn("Checking for multipass-mcp releases...")

	// Get latest release
	client := &http.Client{Timeout: 30 * time.Second}
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", mcpGitHubRepo)
	resp, err := client.Get(url) // #nosec G107 -- URL from constant
	if err != nil {
		return "", fmt.Errorf("failed to check releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to parse release: %w", err)
	}

	// Match platform/arch
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// Find matching asset (skip checksums/sigs)
	var downloadURL, assetName string
	for _, asset := range release.Assets {
		name := strings.ToLower(asset.Name)
		if !strings.HasSuffix(name, ".tar.gz") && !strings.HasSuffix(name, ".zip") {
			continue
		}
		if strings.Contains(name, goos) && strings.Contains(name, goarch) {
			downloadURL = asset.BrowserDownloadURL
			assetName = asset.Name
			break
		}
	}

	if downloadURL == "" {
		return "", fmt.Errorf("no multipass-mcp binary found for %s/%s in release %s", goos, goarch, release.TagName)
	}

	progressFn(fmt.Sprintf("Downloading multipass-mcp %s...", release.TagName))

	// Download to temp file
	dlResp, err := client.Get(downloadURL) // #nosec G107 -- URL from GitHub releases API
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned HTTP %d", dlResp.StatusCode)
	}

	// Write archive to temp file
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".passgo")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}

	tmpFile, err := os.CreateTemp(dir, "mcp-download-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmpFile, dlResp.Body); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("download write failed: %w", err)
	}
	tmpFile.Close()

	progressFn("Extracting multipass-mcp...")

	// Determine binary name in archive
	binaryName := mcpBinaryName
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}

	destPath := filepath.Join(dir, binaryName)

	// Extract based on archive type
	if strings.HasSuffix(strings.ToLower(assetName), ".tar.gz") {
		if err := extractFromTarGz(tmpPath, binaryName, destPath); err != nil {
			return "", fmt.Errorf("extract tar.gz: %w", err)
		}
	} else if strings.HasSuffix(strings.ToLower(assetName), ".zip") {
		if err := extractFromZip(tmpPath, binaryName, destPath); err != nil {
			return "", fmt.Errorf("extract zip: %w", err)
		}
	}

	// Make executable
	if err := os.Chmod(destPath, 0o750); err != nil {
		return "", fmt.Errorf("chmod: %w", err)
	}

	progressFn("multipass-mcp installed successfully")
	return destPath, nil
}

// extractFromTarGz extracts a named file from a .tar.gz archive.
func extractFromTarGz(archivePath, targetName, destPath string) error {
	f, err := os.Open(archivePath) // #nosec G304 -- temp file we just created
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read: %w", err)
		}

		// Match by basename (archive may have just the filename or a directory prefix)
		if filepath.Base(hdr.Name) == targetName && hdr.Typeflag == tar.TypeReg {
			out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o750) // #nosec G304 -- dest from UserHomeDir
			if err != nil {
				return err
			}
			// Limit copy to prevent decompression bombs (100MB should be more than enough)
			if _, err := io.Copy(out, io.LimitReader(tr, 100*1024*1024)); err != nil {
				out.Close()
				return err
			}
			return out.Close()
		}
	}

	return fmt.Errorf("%s not found in archive", targetName)
}

// extractFromZip extracts a named file from a .zip archive.
func extractFromZip(archivePath, targetName, destPath string) error {
	r, err := zip.OpenReader(archivePath) // #nosec G304 -- temp file we just created
	if err != nil {
		return err
	}
	defer r.Close()

	for _, zf := range r.File {
		if filepath.Base(zf.Name) == targetName {
			rc, err := zf.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o750) // #nosec G304 -- dest from UserHomeDir
			if err != nil {
				return err
			}
			// Limit copy to prevent decompression bombs
			if _, err := io.Copy(out, io.LimitReader(rc, 100*1024*1024)); err != nil {
				out.Close()
				return err
			}
			return out.Close()
		}
	}

	return fmt.Errorf("%s not found in archive", targetName)
}
