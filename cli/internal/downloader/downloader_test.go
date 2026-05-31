package downloader

import (
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
