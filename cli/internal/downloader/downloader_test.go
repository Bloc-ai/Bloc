package downloader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDeleteCachedModel_NoDeadlock(t *testing.T) {
	// Create a temporary cache directory
	tmpDir, err := os.MkdirTemp("", "bloc-test-cache-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create subdirectories
	err = os.MkdirAll(filepath.Join(tmpDir, "models"), 0700)
	if err != nil {
		t.Fatalf("failed to create models dir: %v", err)
	}

	// Initialize downloader manager
	m, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Add dummy model file to disk
	dummySHA := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	dummyFile := filepath.Join(tmpDir, "models", dummySHA)
	err = os.WriteFile(dummyFile, []byte("dummy weight bytes"), 0600)
	if err != nil {
		t.Fatalf("failed to write dummy file: %v", err)
	}

	// Set up dummy index entry
	m.indexMu.Lock()
	m.index[dummySHA] = Entry{
		SHA256:       dummySHA,
		FriendlyName: "dummy-model.gguf",
		SizeBytes:    18,
		DownloadURL:  "https://huggingface.co/org/model/resolve/main/dummy-model.gguf",
		CachedAt:     time.Now(),
	}
	m.indexMu.Unlock()

	// Persist initial index to disk
	err = m.writeIndexToDisk()
	if err != nil {
		t.Fatalf("failed to write index to disk: %v", err)
	}

	// Call DeleteCachedModel — this would crash with deadlock on the old code
	done := make(chan bool)
	go func() {
		err := m.DeleteCachedModel("dummy-model.gguf")
		if err != nil {
			t.Errorf("DeleteCachedModel returned error: %v", err)
		}
		done <- true
	}()

	select {
	case <-done:
		// Success! Completed without deadlock
	case <-time.After(2 * time.Second):
		t.Fatal("DEADLOCK DETECTED! DeleteCachedModel took more than 2 seconds to return.")
	}

	// Verify model is deleted from memory
	m.indexMu.RLock()
	_, found := m.index[dummySHA]
	m.indexMu.RUnlock()
	if found {
		t.Error("expected dummy entry to be deleted from in-memory index map")
	}

	// Verify index on disk is updated and exists
	updatedIndex, err := m.readIndexFromDisk()
	if err != nil {
		t.Fatalf("failed to read updated index: %v", err)
	}
	if _, foundOnDisk := updatedIndex[dummySHA]; foundOnDisk {
		t.Error("expected dummy entry to be deleted from index.json on disk")
	}

	// Verify physical file was removed
	if _, err := os.Stat(dummyFile); !os.IsNotExist(err) {
		t.Error("expected dummy model file to be removed from disk cache")
	}
}

// TestEnsureDownloaded_TransientNetworkRetry verifies that the downloader can
// recover from a transient network connection interruption mid-stream and
// complete the download using a Range request (D4 fix).
func TestEnsureDownloaded_TransientNetworkRetry(t *testing.T) {
	// 1. Setup mock server that drops connection on the first request and returns remaining bytes on the second.
	calledCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledCount++
		rangeHeader := r.Header.Get("Range")

		if rangeHeader == "" {
			// First request: Write partial bytes (must contain GGUF magic bytes start) and close abruptly
			w.Header().Set("Content-Length", "10")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("GGUF")) // 4 bytes of GGUF magic
			_, _ = w.Write([]byte("a"))    // 5th byte

			// Flush the buffer to the client before hijacking
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}

			// Hijack the connection and close it abruptly to simulate network reset/failure
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Error("http.ResponseWriter does not support http.Hijacker")
				return
			}
			conn, _, err := hj.Hijack()
			if err == nil {
				conn.Close()
			}
			return
		}

		// Second request: Expect range bytes=5-
		if rangeHeader == "bytes=5-" {
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte("bcdef")) // remaining 5 bytes
			return
		}

		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	// 2. Setup temporary cache directory
	tmpDir, err := os.MkdirTemp("", "bloc-test-cache-d4-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	m, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Calculate SHA256 of the complete expected payload ("GGUFabcdef")
	payload := []byte("GGUFabcdef")
	hasher := sha256.New()
	hasher.Write(payload)
	expectedSHA256 := hex.EncodeToString(hasher.Sum(nil))

	// 3. Call EnsureDownloaded, which should trigger the retry loop
	ctx := context.Background()
	linkPath, err := m.EnsureDownloaded(ctx, "test-model.gguf", srv.URL, expectedSHA256, 0.0, nil)
	if err != nil {
		t.Fatalf("EnsureDownloaded failed: %v", err)
	}

	// 4. Verify file content matches expected complete payload
	data, err := os.ReadFile(linkPath)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}

	if string(data) != string(payload) {
		t.Errorf("downloaded file content mismatch: got %q, want %q", string(data), string(payload))
	}

	if calledCount < 2 {
		t.Errorf("expected at least 2 server calls, got %d", calledCount)
	}
}
