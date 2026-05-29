package downloader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Entry is a record in ~/.cache/bloc/index.json
type Entry struct {
	SHA256       string    `json:"sha256"`
	FriendlyName string    `json:"friendly_name"`
	SizeBytes    int64     `json:"size_bytes"`
	DownloadURL  string    `json:"download_url"`
	CachedAt     time.Time `json:"cached_at"`
}

// CacheIndex maps SHA256 → Entry
type CacheIndex map[string]Entry

// ProgressFn is called periodically with bytes downloaded and total bytes.
type ProgressFn func(downloaded, total int64, speed float64)

// P-06: Package-level download transport with large read/write buffers and
// no global timeout (downloads can take minutes for large GGUF files).
// P-07: TLS handshake and response header timeouts are still enforced via
// the Transport to prevent stall attacks (F-07).
var downloadTransport = &http.Transport{
	// F-07: TLS handshake timeout prevents stalls during connection setup
	TLSHandshakeTimeout: 30 * time.Second,
	// F-07: Response header timeout prevents slow-header DoS
	ResponseHeaderTimeout: 60 * time.Second,
	MaxIdleConnsPerHost:   2,
	// GGUF files are not HTTP-compressed — disable decompression overhead
	DisableCompression: true,
	// P-06: 1 MB buffers match the download chunk size
	WriteBufferSize: 1 << 20,
	ReadBufferSize:  1 << 20,
}

var downloadClient = &http.Client{
	// No overall Timeout — body streaming can take many minutes for multi-GB files.
	// Connection and header deadlines are enforced by the Transport above.
	Transport: downloadTransport,
}

// Manager handles all model download and cache operations.
// P-09: CacheIndex is held in-memory after first load to avoid repeated disk reads.
type Manager struct {
	cacheDir string
	index    CacheIndex
	indexMu  sync.RWMutex
}

// NewManager creates a Manager rooted at the given cache directory.
func NewManager(cacheDir string) (*Manager, error) {
	dirs := []string{
		filepath.Join(cacheDir, "models"),
		filepath.Join(cacheDir, "downloads"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return nil, fmt.Errorf("cannot create cache directory %s: %w", d, err)
		}
	}
	m := &Manager{cacheDir: cacheDir}
	// P-09: Pre-load index once
	m.index, _ = m.readIndexFromDisk()
	return m, nil
}

// ModelPath returns the symlink path for a friendly filename.
func (m *Manager) ModelPath(friendlyName string) string {
	return filepath.Join(m.cacheDir, "models", friendlyName)
}

// IsAlreadyCached checks if a model file is already in the cache.
// F-08: Validates by checking the in-memory index (which stores verified SHA256)
// rather than trusting the symlink target filename.
func (m *Manager) IsAlreadyCached(friendlyName, expectedSHA256 string) (bool, error) {
	linkPath := m.ModelPath(friendlyName)

	// If no SHA256 provided, just check if the symlink/file exists
	if expectedSHA256 == "" {
		_, err := os.Stat(linkPath)
		return err == nil, nil
	}

	// F-08: Check in-memory index for a verified entry with matching SHA256.
	m.indexMu.RLock()
	entry, found := m.index[expectedSHA256]
	m.indexMu.RUnlock()

	if !found {
		return false, nil
	}

	// Verify the actual file still exists on disk (not just the index entry)
	finalPath := filepath.Join(m.cacheDir, "models", entry.SHA256)
	if _, err := os.Stat(finalPath); err != nil {
		// File was deleted externally — remove stale index entry
		m.indexMu.Lock()
		delete(m.index, expectedSHA256)
		m.indexMu.Unlock()
		_ = m.writeIndexToDisk()
		return false, nil
	}

	return true, nil
}

// EnsureDownloaded downloads the model if not already cached.
// Returns the absolute path to the model file.
// P-07: Replaced recursive EnsureDownloaded call on 416 with iterative retry loop.
func (m *Manager) EnsureDownloaded(ctx context.Context, friendlyName, downloadURL, expectedSHA256 string, sizeGB float64, progress ProgressFn) (string, error) {
	// Check cache first
	cached, err := m.IsAlreadyCached(friendlyName, expectedSHA256)
	if err == nil && cached {
		return m.ModelPath(friendlyName), nil
	}

	// Determine partial download path
	partialName := expectedSHA256
	if partialName == "" {
		partialName = sanitizeFilename(friendlyName)
	}
	partialPath := filepath.Join(m.cacheDir, "downloads", partialName+".incomplete")

	const maxRetries = 2
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Get size of partial file for resume
		var startByte int64
		if stat, err := os.Stat(partialPath); err == nil {
			startByte = stat.Size()
		}

		// Create HTTP request with Range header for resume
		req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
		if err != nil {
			return "", fmt.Errorf("cannot create download request: %w", err)
		}
		if startByte > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", startByte))
		}
		// Hugging Face requires a User-Agent
		req.Header.Set("User-Agent", "bloc-cli/1.0 (https://bloc-theta.vercel.app)")

		// P-06: Use package-level client with connection/header timeouts (F-07)
		resp, err := downloadClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("download failed: %w", err)
		}

		if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
			// P-07: Server rejected range — delete partial and retry iteratively
			resp.Body.Close()
			os.Remove(partialPath)
			continue // retry from scratch (startByte will be 0 next iteration)
		}

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			resp.Body.Close()
			return "", fmt.Errorf("server returned %d for %s", resp.StatusCode, downloadURL)
		}

		totalBytes := resp.ContentLength + startByte
		if totalBytes <= 0 && sizeGB > 0 {
			totalBytes = int64(sizeGB * 1024 * 1024 * 1024)
		}

		// Open partial file for append (or create)
		flags := os.O_CREATE | os.O_WRONLY
		if startByte > 0 {
			flags |= os.O_APPEND
		}
		f, err := os.OpenFile(partialPath, flags, 0644)
		if err != nil {
			resp.Body.Close()
			return "", fmt.Errorf("cannot open partial file: %w", err)
		}

		// Stream with progress tracking
		hasher := sha256.New()
		downloaded := startByte
		buf := make([]byte, 1<<20) // 1 MB chunks
		lastReport := time.Now()
		startTime := time.Now()

		var streamErr error
		for {
			select {
			case <-ctx.Done():
				f.Close()
				resp.Body.Close()
				return "", ctx.Err()
			default:
			}

			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				// P-08: Propagate disk write errors immediately — don't silently corrupt
				if _, writeErr := f.Write(buf[:n]); writeErr != nil {
					f.Close()
					resp.Body.Close()
					return "", fmt.Errorf("disk write failed (disk full?): %w", writeErr)
				}
				hasher.Write(buf[:n])
				downloaded += int64(n)

				if progress != nil && time.Since(lastReport) > 200*time.Millisecond {
					elapsed := time.Since(startTime).Seconds()
					speedMBs := float64(downloaded-startByte) / 1024 / 1024 / elapsed
					progress(downloaded, totalBytes, speedMBs)
					lastReport = time.Now()
				}
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				streamErr = readErr
				break
			}
		}
		f.Close()
		resp.Body.Close()

		if streamErr != nil {
			return "", fmt.Errorf("download interrupted: %w", streamErr)
		}

		// Verify SHA256 if provided
		actualSHA256 := hex.EncodeToString(hasher.Sum(nil))
		if expectedSHA256 != "" && !strings.EqualFold(actualSHA256, expectedSHA256) {
			os.Remove(partialPath)
			return "", fmt.Errorf("SHA256 mismatch: expected %s, got %s — file deleted", expectedSHA256, actualSHA256)
		}

		// Determine final hash-addressed path
		hashName := actualSHA256
		if hashName == "" {
			hashName = sanitizeFilename(friendlyName) + "-" + fmt.Sprintf("%d", time.Now().Unix())
		}
		finalPath := filepath.Join(m.cacheDir, "models", hashName)

		// Atomic rename: partial → hash-addressed file
		if err := os.Rename(partialPath, finalPath); err != nil {
			return "", fmt.Errorf("cannot move completed download: %w", err)
		}

		// Create friendly symlink: models/filename.gguf → ./sha256hash
		linkPath := m.ModelPath(friendlyName)
		os.Remove(linkPath) // remove stale link if any
		if err := os.Symlink(finalPath, linkPath); err != nil {
			// Symlink failed (e.g. cross-device) — use direct path
			return finalPath, nil
		}

		// P-09: Update in-memory index and persist to disk once per download
		entry := Entry{
			SHA256:       actualSHA256,
			FriendlyName: friendlyName,
			SizeBytes:    downloaded,
			DownloadURL:  downloadURL,
			CachedAt:     time.Now(),
		}
		m.indexMu.Lock()
		m.index[actualSHA256] = entry
		m.indexMu.Unlock()
		_ = m.writeIndexToDisk()

		return linkPath, nil
	}

	return "", fmt.Errorf("download failed after %d attempts (server rejected range request)", maxRetries)
}

// ListCached returns all models in the cache index.
func (m *Manager) ListCached() ([]Entry, error) {
	m.indexMu.RLock()
	defer m.indexMu.RUnlock()
	entries := make([]Entry, 0, len(m.index))
	for _, e := range m.index {
		entries = append(entries, e)
	}
	return entries, nil
}

// ClearCache removes all cached model files.
func (m *Manager) ClearCache() error {
	modelsDir := filepath.Join(m.cacheDir, "models")
	downloadsDir := filepath.Join(m.cacheDir, "downloads")

	for _, dir := range []string{modelsDir, downloadsDir} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
	indexPath := filepath.Join(m.cacheDir, "index.json")
	os.Remove(indexPath)

	// Clear in-memory index too
	m.indexMu.Lock()
	m.index = make(CacheIndex)
	m.indexMu.Unlock()

	return nil
}

// readIndexFromDisk loads the cache index from disk.
func (m *Manager) readIndexFromDisk() (CacheIndex, error) {
	path := filepath.Join(m.cacheDir, "index.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return make(CacheIndex), nil
	}
	if err != nil {
		return nil, err
	}
	var index CacheIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return make(CacheIndex), nil
	}
	return index, nil
}

// writeIndexToDisk persists the in-memory index to disk.
// P-09: Called once per download completion, not on every chunk.
func (m *Manager) writeIndexToDisk() error {
	m.indexMu.RLock()
	data, err := json.MarshalIndent(m.index, "", "  ")
	m.indexMu.RUnlock()
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(m.cacheDir, "index.json"), data, 0644)
}

func sanitizeFilename(name string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '-'
	}, name)
}
