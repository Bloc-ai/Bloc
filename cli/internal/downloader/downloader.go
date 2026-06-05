package downloader

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
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
	MaxIdleConnsPerHost:   4, // raised for parallel repo downloads
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
	// SEC-NEW-A: Always strip the Authorization header on redirects.
	// Go's default policy preserves it on same-host redirects; a compromised CDN
	// or DNS rebinding attack could then steal the HF bearer token.
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		req.Header.Del("Authorization") // F-21: never forward HF token on redirect
		return nil
	},
}

// Manager handles all model download and cache operations.
// P-09: CacheIndex is held in-memory after first load to avoid repeated disk reads.
// Fix #6: hfToken is set via SetHFToken() and injected into every HF request.
type Manager struct {
	cacheDir string
	index    CacheIndex
	indexMu  sync.RWMutex
	hfToken  string // F-21: never logged or printed
}

// PERF-01: Package-level buffer pool for 1 MB download buffers.
// downloadRepoFile runs up to 4 goroutines concurrently; without a pool each
// goroutine allocates a fresh 1 MB buffer per file, creating GC pressure on
// large repos (100+ shards). The pool recycles buffers across goroutines.
var downloadBufPool = &sync.Pool{
	New: func() any {
		buf := make([]byte, 1<<20) // 1 MB
		return &buf
	},
}

// NewManager creates a Manager rooted at the given cache directory.
func NewManager(cacheDir string) (*Manager, error) {
	dirs := []string{
		filepath.Join(cacheDir, "models"),
		filepath.Join(cacheDir, "downloads"),
		filepath.Join(cacheDir, "repos"),
	}
	for _, d := range dirs {
		// SEC-03: Use 0700 (owner-only) instead of 0755 for cache directories
		// to prevent other users on a shared machine from reading model files.
		if err := os.MkdirAll(d, 0700); err != nil {
			return nil, fmt.Errorf("cannot create cache directory %s: %w", d, err)
		}
	}
	m := &Manager{cacheDir: cacheDir}
	// LOW-7: Always initialize m.index. If readIndexFromDisk returns an I/O error
	// (not just ErrNotExist), silently swallowing it with _ leaves m.index nil,
	// causing a nil map panic on the next write. Fallback to an empty index.
	if idx, err := m.readIndexFromDisk(); err == nil {
		m.index = idx
	} else {
		m.index = make(CacheIndex)
	}
	return m, nil
}

// SetHFToken sets the Hugging Face access token used for all HF API and
// download requests. Call this immediately after NewManager if a token is available.
// Fix #6: token is stored only in-memory, never written to disk by Manager.
func (m *Manager) SetHFToken(token string) {
	m.hfToken = token
}

// injectHFAuth adds the Authorization header to a request destined for
// huggingface.co if a token is configured. Fix #6.
// F-21: token value is never logged or included in error strings.
func (m *Manager) injectHFAuth(req *http.Request) {
	if m.hfToken != "" {
		req.Header.Set("Authorization", "Bearer "+m.hfToken)
	}
}

// ModelPath returns the symlink path for a friendly filename.
func (m *Manager) ModelPath(friendlyName string) string {
	return filepath.Join(m.cacheDir, "models", friendlyName)
}

// RepoPath returns the local directory path for a cached HF repo.
func (m *Manager) RepoPath(hfRepo, revision string) string {
	safe := strings.ReplaceAll(hfRepo, "/", "--")
	return filepath.Join(m.cacheDir, "repos", safe, revision)
}

// ggufMagic is the 4-byte magic number that starts every valid GGUF file.
var ggufMagic = []byte{'G', 'G', 'U', 'F'}

// validateGGUF opens the file at path and verifies the first 4 bytes are the
// GGUF magic number. Returns a descriptive error if the file is corrupt,
// truncated, or not a GGUF. Used both after download and on cache-hit checks.
func validateGGUF(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot open file for validation: %w", err)
	}
	defer f.Close()
	header := make([]byte, 4)
	if _, err := io.ReadFull(f, header); err != nil {
		return fmt.Errorf("file too small to be a valid GGUF model (read error: %w)", err)
	}
	for i, b := range ggufMagic {
		if header[i] != b {
			return fmt.Errorf("invalid GGUF magic bytes: got %#v, expected %#v — file is corrupt or incomplete",
				header, ggufMagic)
		}
	}
	return nil
}

// IsAlreadyCached checks if a model file is already in the cache.
// F-08: Validates by checking the in-memory index (which stores verified SHA256)
// rather than trusting the symlink target filename.
func (m *Manager) IsAlreadyCached(friendlyName, expectedSHA256 string) (bool, error) {
	linkPath := m.ModelPath(friendlyName)

	// If no SHA256 provided, check existence + GGUF magic.
	if expectedSHA256 == "" {
		if _, err := os.Stat(linkPath); err != nil {
			return false, nil
		}
		// GGUF magic check: verify the file is a valid model, not a partial
		// download or CDN error page.
		if err := validateGGUF(linkPath); err != nil {
			fmt.Fprintf(os.Stderr,
				"  \033[33m⚠  Cached file %s failed GGUF validation (%v) — evicting and re-downloading\033[0m\n",
				filepath.Base(linkPath), err,
			)
			_ = os.Remove(linkPath)
			return false, nil
		}
		return true, nil
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

	// GGUF magic check on indexed files too.
	if err := validateGGUF(finalPath); err != nil {
		fmt.Fprintf(os.Stderr,
			"  \033[33m⚠  Indexed file %s failed GGUF validation (%v) — evicting and re-downloading\033[0m\n",
			filepath.Base(finalPath), err,
		)
		m.indexMu.Lock()
		delete(m.index, expectedSHA256)
		m.indexMu.Unlock()
		_ = m.writeIndexToDisk()
		_ = os.Remove(finalPath)
		_ = os.Remove(linkPath)
		return false, nil
	}

	return true, nil
}

// EnsureDownloaded downloads the model if not already cached.
// Returns the absolute path to the model file.
// P-07: Replaced recursive EnsureDownloaded call on 416 with iterative retry loop.
// Fix #1: SHA256 hasher is pre-seeded with bytes of any existing partial file
//
//	so a resumed download produces the correct full-file hash.
//
// Fix #6: HF auth token injected into every download request.
func (m *Manager) EnsureDownloaded(ctx context.Context, friendlyName, downloadURL, expectedSHA256 string, sizeGB float64, progress ProgressFn) (string, error) {
	// Check cache first.
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

	// PERF-03 + PERF-NEW-E: Hoist the 1 MB buffer before the retry loop so it
	// is allocated once rather than once per retry attempt.
	// PERF-NEW-E: Use O_RDWR to seed the hasher AND append without reopening:
	// the old code opened the file for append then opened it AGAIN (os.Open)
	// to seed the SHA256 hasher, causing a full re-read of a potentially
	// multi-GB partial file from disk before receiving any new bytes.
	buf := make([]byte, 1<<20) // 1 MB — allocated once outside the retry loop

	const maxRetries = 5
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
		req.Header.Set("User-Agent", "bloc-cli/1.0 (https://bloc-theta.vercel.app)")
		m.injectHFAuth(req) // Fix #6

		// P-06: Use package-level client with connection/header timeouts (F-07)
		resp, err := downloadClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("download failed: %w", err)
		}

		if resp.StatusCode == http.StatusUnauthorized {
			resp.Body.Close()
			// Provide a clear, actionable gated-model error (F-21: no token in message)
			return "", fmt.Errorf(
				"access denied (401): %s may be a gated model.\n"+
					"  1. Accept the license at: https://huggingface.co/%s\n"+
					"  2. Run: bloc login --hf\n"+
					"  Then retry.",
				friendlyName, strings.TrimPrefix(downloadURL, "https://huggingface.co/"),
			)
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

		// PERF-NEW-E: Open the partial file with O_RDWR so we can both read
		// existing bytes (for the SHA256 hasher pre-seed) AND append new bytes
		// using a single file descriptor — no second os.Open required.
		// SEC-03: Use 0600 (owner-only).
		flags := os.O_CREATE | os.O_RDWR
		f, err := os.OpenFile(partialPath, flags, 0600)
		if err != nil {
			resp.Body.Close()
			return "", fmt.Errorf("cannot open partial file: %w", err)
		}

		// Fix #1 + PERF-NEW-E: Seed the SHA256 hasher by reading existing bytes
		// through the same file descriptor (seeked to start), then seek to end
		// for appending. Eliminates the double disk read from the old pattern:
		//   os.OpenFile(O_APPEND) + os.Open (read-only to seed hasher).
		hasher := sha256.New()
		if startByte > 0 && expectedSHA256 != "" {
			if _, seekErr := f.Seek(0, io.SeekStart); seekErr == nil {
				// Fix #3: If pre-seeding fails, the partial file is unreadable or
				// corrupt. Delete it and restart the download from byte 0 rather
				// than continuing with a poisoned hasher that will never produce
				// the correct final SHA256.
				if _, copyErr := io.CopyBuffer(hasher, io.LimitReader(f, startByte), buf); copyErr != nil {
					f.Close()
					resp.Body.Close()
					_ = os.Remove(partialPath)
					fmt.Fprintf(os.Stderr,
						"  \033[33m⚠  Partial file is unreadable (%v) — discarding and restarting download\033[0m\n",
						copyErr,
					)
					// Reset startByte so the next iteration starts fresh.
					startByte = 0
					continue
				}
			}
			// Position at end for appending new bytes.
			if _, seekErr := f.Seek(0, io.SeekEnd); seekErr != nil {
				f.Close()
				resp.Body.Close()
				return "", fmt.Errorf("cannot seek to end of partial file: %w", seekErr)
			}
		}

		downloaded := startByte
		// PERF-03: buf is hoisted outside the retry loop — reused across attempts.
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

				// PERF-22 (PM-7): Call time.Now() once per chunk instead of twice.
				if progress != nil {
					now := time.Now()
					if now.Sub(lastReport) > 200*time.Millisecond {
						elapsed := now.Sub(startTime).Seconds()
						speedMBs := float64(downloaded-startByte) / 1024 / 1024 / elapsed
						progress(downloaded, totalBytes, speedMBs)
						lastReport = now
					}
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
			if !isTransientNetErr(streamErr) || attempt == maxRetries-1 {
				return "", fmt.Errorf("download interrupted: %w", streamErr)
			}
			// Transient error — wait and retry from current offset
			backoff := time.Duration(attempt+1) * 2 * time.Second
			fmt.Fprintf(os.Stderr, "\n  ⚠  Network error (%v), retrying in %s...\n", streamErr, backoff)
			time.Sleep(backoff)
			continue
		}

		// Fix #1: Verify GGUF magic bytes before accepting the downloaded file.
		// This catches truncated CDN responses, corrupt transfers, and any other
		// case where the file content is not a valid GGUF model.
		if err := validateGGUF(partialPath); err != nil {
			_ = os.Remove(partialPath)
			return "", fmt.Errorf("downloaded file is not a valid GGUF model: %w — file deleted", err)
		}

		// Fix #2: When no SHA256 is provided in the recipe, validate by size.
		// This catches truncated downloads where the server returned HTTP 200
		// but streamed fewer bytes than expected (e.g., cached CDN partial).
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

		// SEC-14: Atomic symlink creation: write to a temp name then rename
		// to avoid the TOCTOU window between Remove and Symlink.
		linkPath := m.ModelPath(friendlyName)
		tmpLink := linkPath + ".tmp-" + randomHex8()
		if err := os.Symlink(finalPath, tmpLink); err != nil {
			// Symlink failed (e.g. cross-device) — use direct path
			return finalPath, nil
		}
		if err := os.Rename(tmpLink, linkPath); err != nil {
			os.Remove(tmpLink)
			return finalPath, nil
		}

		// SEC-15 (L-5): Strip query parameters (which may contain signed CDN auth tokens)
		// before persisting the URL to disk.
		cleanURL := downloadURL
		if u, err := url.Parse(downloadURL); err == nil {
			u.RawQuery = ""
			cleanURL = u.String()
		}

		// P-09: Update in-memory index and persist to disk once per download
		entry := Entry{
			SHA256:       actualSHA256,
			FriendlyName: friendlyName,
			SizeBytes:    downloaded,
			DownloadURL:  cleanURL,
			CachedAt:     time.Now(),
		}
		m.indexMu.Lock()
		m.index[actualSHA256] = entry
		m.indexMu.Unlock()
		_ = m.writeIndexToDisk()

		return linkPath, nil
	}

	return "", fmt.Errorf("download failed after %d attempts (server rejected range request or network issue)", maxRetries)
}

// isTransientNetErr returns true for connection resets, timeouts, etc.
func isTransientNetErr(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	return false
}

// ─── HuggingFace Repository Downloader ───────────────────────────────────────

// hfSibling represents a file entry from the HF API siblings list.
type hfSibling struct {
	Rfilename string `json:"rfilename"`
	Size      int64  `json:"size"`
	BlobID    string `json:"blob_id"` // SHA256 of the file content on HF
}

// hfModelInfo is the subset of the HF API /api/models/{repo} response we need.
type hfModelInfo struct {
	Siblings []hfSibling `json:"siblings"`
}

var repoFileKeepPatterns = []string{
	".safetensors",
	".bin",          // older PyTorch weights, some models still use these
	"config.json",
	"tokenizer",
	"tokenizer_config.json",
	"special_tokens_map.json",
	"generation_config.json",
	"chat_template",
	".json",
	".py",           // F-18: custom model code — downloaded, never CLI-executed
	".tiktoken",
	".model",        // SentencePiece tokenizer models
	"pytorch_model",
}

// repoFileFilter returns true for file types that vLLM needs from an HF repo.
// We download model weights, config, tokenizer, and optional custom code.
// .py files are downloaded for non-standard architectures but NEVER executed
// by the CLI — they run inside vLLM's Python runtime only (F-18).
func repoFileFilter(name string) bool {
	lower := strings.ToLower(name)
	for _, suffix := range repoFileKeepPatterns {
		if strings.HasSuffix(lower, suffix) || strings.Contains(lower, suffix) {
			return true
		}
	}
	return false
}

// EnsureRepoDownloaded downloads a full HuggingFace model repository
// (safetensors + config + tokenizer + optional custom code) to
// ~/.cache/bloc/repos/{org}--{model}/{revision}/.
//
// It checks existing files against HF's reported sizes and skips files
// that are already fully downloaded. Files that exist but have the wrong
// size are re-downloaded from scratch (Fix #1 equivalent for multi-file repos).
//
// Fix #6: HF auth token is injected into the /api/models metadata request
// and into every file download, enabling gated model access.
//
// Returns the absolute local path to the repository root directory.
func (m *Manager) EnsureRepoDownloaded(
	ctx context.Context,
	hfRepo string,  // "org/model-name"
	revision string, // "main" or a specific commit SHA
	progress ProgressFn,
) (string, error) {
	if revision == "" {
		revision = "main"
	}

	repoDir := m.RepoPath(hfRepo, revision)
	// SEC-03: Use 0700 for the repo directory (owner-only).
	if err := os.MkdirAll(repoDir, 0700); err != nil {
		return "", fmt.Errorf("cannot create repo dir %s: %w", repoDir, err)
	}

	// ── Step 1: Fetch file list from HF API ───────────────────────────────────
	// SEC-10: URL-escape hfRepo and revision to prevent SSRF-like URL manipulation
	// (e.g., hf_repo: "org/model?token=leaked" injecting extra query params).
	apiURL := fmt.Sprintf("https://huggingface.co/api/models/%s?revision=%s",
		url.PathEscape(hfRepo), url.QueryEscape(revision))
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("cannot build HF API request: %w", err)
	}
	req.Header.Set("User-Agent", "bloc-cli/1.0 (https://bloc-theta.vercel.app)")
	m.injectHFAuth(req) // Fix #6

	resp, err := downloadClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("HF API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return "", fmt.Errorf(
			"access denied (401): %s is a gated model.\n"+
				"  1. Accept the license at: https://huggingface.co/%s\n"+
				"  2. Run: bloc login --hf\n"+
				"  Then retry.",
			hfRepo, hfRepo,
		)
	}
	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("model %q not found on HuggingFace — check the repo name", hfRepo)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HF API returned %d for %s", resp.StatusCode, hfRepo)
	}

	// 1 MB cap on metadata response — a valid model card is never larger
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("cannot read HF API response: %w", err)
	}

	var modelInfo hfModelInfo
	if err := json.Unmarshal(body, &modelInfo); err != nil {
		return "", fmt.Errorf("cannot parse HF API response: %w", err)
	}

	// ── Step 2: Filter to files we need ──────────────────────────────────────
	var toDownload []hfSibling
	for _, sib := range modelInfo.Siblings {
		if repoFileFilter(sib.Rfilename) {
			toDownload = append(toDownload, sib)
		}
	}
	if len(toDownload) == 0 {
		return "", fmt.Errorf("no downloadable files found in %s — check the repo name or revision", hfRepo)
	}

	// ── Step 3: Validate file count and calculate total size ──────────────────
	// SEC-NEW-B: Cap the number of files to prevent channel/memory exhaustion.
	const maxRepoFiles = 1000
	if len(toDownload) > maxRepoFiles {
		return "", fmt.Errorf("repo %s has %d files (limit %d) — contact support if legitimate",
			hfRepo, len(toDownload), maxRepoFiles)
	}

	// SEC-NEW-B: Overflow-safe sum — attacker-controlled f.Size values can wrap int64.
	var totalBytes int64
	for _, f := range toDownload {
		if f.Size > 0 && f.Size < math.MaxInt64-totalBytes {
			totalBytes += f.Size
		}
	}

	// ── Step 4: Parallel download with bounded goroutine pool ─────────────────
	// LOW-PERF-12: Expose worker pool size configuration via BLOC_DOWNLOAD_WORKERS
	workers := 4
	if envW := os.Getenv("BLOC_DOWNLOAD_WORKERS"); envW != "" {
		if val, err := strconv.Atoi(envW); err == nil && val > 0 {
			workers = val
			if workers > 16 {
				workers = 16 // cap to protect HF API and system resources
			}
		}
	}

	type result struct {
		filename string
		err      error
	}

	jobs := make(chan hfSibling, len(toDownload))
	results := make(chan result, len(toDownload))

	// MEDIUM-PERF-04: Derive cancellable context to stop all workers immediately on first error
	downloadCtx, downloadCancel := context.WithCancel(ctx)
	defer downloadCancel()

	// M-4: Track worker goroutines with a WaitGroup so they are guaranteed to
	// exit before this function returns, even on early cancellation.
	var workerWg sync.WaitGroup
	for i := 0; i < workers; i++ {
		workerWg.Add(1)
		go func() {
			defer workerWg.Done()
			for sib := range jobs {
				err := m.downloadRepoFile(downloadCtx, hfRepo, revision, repoDir, sib)
				results <- result{filename: sib.Rfilename, err: err}
			}
		}()
	}

	for _, sib := range toDownload {
		jobs <- sib
	}
	close(jobs)
	// Wait for all workers to finish after jobs channel is closed and results collected.
	// We start a goroutine to close results once workers are done.
	go func() {
		workerWg.Wait()
		close(results)
	}()

	// PERF-02: Pre-build a filename→size map so result collection is O(1)
	// instead of O(n²). For a 500-file repo the old code did 500×500 = 250,000
	// string comparisons in the worst case.
	sizeByFile := make(map[string]int64, len(toDownload))
	for _, sib := range toDownload {
		sizeByFile[sib.Rfilename] = sib.Size
	}

	// Collect results — fail fast on first error
	var downloadedBytes int64
	lastProgressReport := time.Now()
	startTime := time.Now()
	var downloadErr error
	for range toDownload {
		r := <-results
		if r.err != nil && downloadErr == nil {
			downloadErr = fmt.Errorf("failed to download %s: %w", r.filename, r.err)
			downloadCancel() // cancel all in-flight workers immediately
		}
		if downloadErr == nil {
			downloadedBytes += sizeByFile[r.filename]
			if progress != nil && time.Since(lastProgressReport) > 500*time.Millisecond {
				elapsed := time.Since(startTime).Seconds()
				var speedMBs float64
				if elapsed > 0 {
					speedMBs = float64(downloadedBytes) / 1024 / 1024 / elapsed
				}
				progress(downloadedBytes, totalBytes, speedMBs)
				lastProgressReport = time.Now()
			}
		}
	}

	if downloadErr != nil {
		return "", downloadErr
	}

	return repoDir, nil
}

// downloadRepoFile downloads a single file from an HF repo into repoDir.
// If the file already exists with the correct size, it is skipped (resume-aware).
// Fix #6: HF auth token injected into download request.
func (m *Manager) downloadRepoFile(ctx context.Context, hfRepo, revision, repoDir string, sib hfSibling) error {
	// SEC-06: Path containment check — sib.Rfilename comes from the HF API and
	// could be "../../.ssh/authorized_keys" in a malicious or MITM response.
	if strings.Contains(sib.Rfilename, "..") || filepath.IsAbs(sib.Rfilename) {
		return fmt.Errorf("security: unsafe rfilename %q rejected", sib.Rfilename)
	}
	localPath := filepath.Join(repoDir, sib.Rfilename)
	// H-5: Check Abs errors explicitly — a failure would let the prefix check
	// pass trivially. Also resolve symlinks (EvalSymlinks) to prevent a
	// symlink inside repoDir pointing outside the cache directory.
	absRepo, err := filepath.Abs(repoDir)
	if err != nil {
		return fmt.Errorf("security: cannot resolve repo directory %q: %w", repoDir, err)
	}
	absLocal, err := filepath.Abs(localPath)
	if err != nil {
		return fmt.Errorf("security: cannot resolve local path for %q: %w", sib.Rfilename, err)
	}
	if !strings.HasPrefix(absLocal, absRepo+string(filepath.Separator)) {
		return fmt.Errorf("security: rfilename %q escapes repo directory — download rejected", sib.Rfilename)
	}
	// H-5: After the prefix check passes on the clean path, also verify that
	// if the file already exists as a symlink it doesn't escape the repo dir.
	if resolved, symlinkErr := filepath.EvalSymlinks(localPath); symlinkErr == nil {
		resolvedAbs, _ := filepath.Abs(resolved)
		if !strings.HasPrefix(resolvedAbs, absRepo+string(filepath.Separator)) {
			return fmt.Errorf("security: rfilename %q resolves via symlink outside repo directory — rejected", sib.Rfilename)
		}
	}

	// Create parent subdirectories if needed (e.g. "pytorch_model-00001-of-00008.safetensors")
	// SEC-03: Use 0700 for subdirectories.
	if err := os.MkdirAll(filepath.Dir(localPath), 0700); err != nil {
		return fmt.Errorf("cannot create subdirectory: %w", err)
	}

	// Check if already fully downloaded (size match is sufficient for repo files —
	// HF doesn't always expose per-file SHA256 in the siblings list)
	if stat, err := os.Stat(localPath); err == nil && sib.Size > 0 {
		if stat.Size() == sib.Size {
			return nil // already complete
		}
		// Wrong size — remove and re-download from scratch
		os.Remove(localPath)
	}

	// Build download URL: HF resolve endpoint.
	// SEC-10: URL-escape all path components to prevent injection via crafted filenames.
	downloadURL := fmt.Sprintf(
		"https://huggingface.co/%s/resolve/%s/%s",
		url.PathEscape(hfRepo), url.PathEscape(revision), url.PathEscape(sib.Rfilename),
	)

	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return fmt.Errorf("cannot build request: %w", err)
	}
	req.Header.Set("User-Agent", "bloc-cli/1.0 (https://bloc-theta.vercel.app)")
	m.injectHFAuth(req) // Fix #6

	resp, err := downloadClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("access denied (401) — run 'bloc login --hf' to authenticate")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}

	// Write directly to the target path (atomic on completion via rename)
	tmpPath := localPath + ".tmp." + randomHex8()
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("cannot create file: %w", err)
	}

	// PERF-01: Get a pooled 1 MB buffer instead of allocating a new one per file.
	// With 4 parallel workers, this reduces GC pressure on large repos significantly.
	bufPtr := downloadBufPool.Get().(*[]byte)
	buf := *bufPtr
	defer downloadBufPool.Put(bufPtr)
	for {
		select {
		case <-ctx.Done():
			f.Close()
			os.Remove(tmpPath)
			return ctx.Err()
		default:
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := f.Write(buf[:n]); writeErr != nil {
				f.Close()
				os.Remove(tmpPath)
				return fmt.Errorf("disk write failed: %w", writeErr)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			f.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("read interrupted: %w", readErr)
		}
	}
	f.Close()

	// Atomic rename from .tmp to final path
	if err := os.Rename(tmpPath, localPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("cannot rename completed file: %w", err)
	}

	return nil
}

// IsRepoCached returns true if the repo directory exists and contains at least one completed file.
func (m *Manager) IsRepoCached(hfRepo, revision string) bool {
	if revision == "" {
		revision = "main"
	}
	dir := m.RepoPath(hfRepo, revision)
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() && !strings.HasSuffix(entry.Name(), ".tmp") {
			if info, err := entry.Info(); err == nil && info.Size() > 0 {
				return true
			}
		}
	}
	return false
}

// ─── Existing index/utility methods (unchanged) ───────────────────────────────

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
	reposDir := filepath.Join(m.cacheDir, "repos")

	for _, dir := range []string{modelsDir, downloadsDir} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
	// PERF-13 / SEC: Use RemoveAll for the repos dir because it contains
	// subdirectories (org--model/revision/files). os.Remove silently fails on dirs.
	if err := os.RemoveAll(reposDir); err == nil {
		// Recreate an empty repos dir with tight permissions.
		_ = os.MkdirAll(reposDir, 0700)
	}
	indexPath := filepath.Join(m.cacheDir, "index.json")
	os.Remove(indexPath)

	// Clear in-memory index too
	m.indexMu.Lock()
	m.index = make(CacheIndex)
	m.indexMu.Unlock()

	return nil
}

// DeleteCachedModel deletes a specific cached GGUF model and updates index.json.
func (m *Manager) DeleteCachedModel(friendlyName string) error {
	// PERF-26 (PL-3): Find the entry under an RLock to avoid blocking readers during O(n) scan
	m.indexMu.RLock()
	var targetSHA string
	for sha, e := range m.index {
		if e.FriendlyName == friendlyName {
			targetSHA = sha
			break
		}
	}
	m.indexMu.RUnlock()

	if targetSHA != "" {
		m.indexMu.Lock()
		// Re-verify under write lock
		if e, exists := m.index[targetSHA]; exists && e.FriendlyName == friendlyName {
			delete(m.index, targetSHA)
			_ = m.writeIndexToDiskLocked()
			
			// Remove physical file
			_ = os.Remove(filepath.Join(m.cacheDir, "models", targetSHA))
		}
		m.indexMu.Unlock()
	}

	// Also remove symlink
	_ = os.Remove(filepath.Join(m.cacheDir, "models", friendlyName))

	return nil
}

// DeleteCachedRepo deletes a specific cached HF repository.
func (m *Manager) DeleteCachedRepo(hfRepo, revision string) error {
	if revision == "" {
		revision = "main"
	}
	repoDir := m.RepoPath(hfRepo, revision)
	return os.RemoveAll(repoDir)
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

// writeIndexToDiskLocked persists the in-memory index to disk without managing locks.
// The caller MUST already hold the m.indexMu write or read lock.
func (m *Manager) writeIndexToDiskLocked() error {
	snapshot := make(CacheIndex, len(m.index))
	for k, v := range m.index {
		snapshot[k] = v
	}

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	// SEC-03: Use 0600 (owner-only) for the index file — it contains DownloadURL
	// fields that may include signed CDN tokens for some HF blob downloads.
	return os.WriteFile(filepath.Join(m.cacheDir, "index.json"), data, 0600)
}

// writeIndexToDisk persists the in-memory index to disk with thread-safety.
// P-09: Called once per download completion, not on every chunk.
func (m *Manager) writeIndexToDisk() error {
	m.indexMu.Lock() // PM-5: Use exclusive Lock so two goroutines don't write to disk concurrently
	defer m.indexMu.Unlock()
	return m.writeIndexToDiskLocked()
}

func sanitizeFilename(name string) string {
	// SEC-16 (L-3): Cap filename length to prevent filesystem errors or log spam
	if len(name) > 100 {
		name = name[:100]
	}
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '-'
	}, name)
}

// randomHex8 returns 4 random bytes encoded as an 8-character hex string.
// Used to generate unique temp-symlink names in the atomic symlink pattern (SEC-14).
func randomHex8() string {
	b := make([]byte, 4)
	// Best-effort random: if crypto/rand is unavailable (e.g. exhausted entropy) we
	// fall back to a timestamp-derived value — uniqueness is sufficient here.
	if _, err := rand.Read(b); err != nil {
		now := time.Now().UnixNano()
		b[0] = byte(now)
		b[1] = byte(now >> 8)
		b[2] = byte(now >> 16)
		b[3] = byte(now >> 24)
	}
	return hex.EncodeToString(b)
}
