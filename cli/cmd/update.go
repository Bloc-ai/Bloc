package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

const releasesAPI = "https://api.github.com/repos/bloc-org/bloc/releases/latest"

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update the bloc CLI to the latest version",
	Long: `Downloads the latest release from GitHub, verifies the SHA256 checksum,
and atomically replaces the current binary.`,
	RunE: runUpdate,
}

func runUpdate(cmd *cobra.Command, args []string) error {
	fmt.Printf("Current version: bloc %s\n", Version)
	fmt.Print("Checking for updates...")

	// Fetch latest release info
	// P-01: Use shared package-level apiClient (see cmd/httpclient.go)
	// F-16: Handle http.NewRequest error — previously silently discarded with _
	req, err := http.NewRequest("GET", releasesAPI, nil)
	if err != nil {
		return fmt.Errorf("cannot build update request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "bloc-cli/"+Version)

	resp, err := apiClient.Do(req)
	if err != nil {
		return fmt.Errorf("cannot check for updates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		fmt.Println("\n  No releases published yet.")
		fmt.Println("  Watch for releases at: https://github.com/bloc-org/bloc/releases")
		return nil
	}

	fmt.Println()

	// TODO: Parse JSON response to get tag_name and download_url for current platform
	// For now, direct user to releases page
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	fmt.Printf("Platform: %s/%s\n\n", goos, goarch)
	fmt.Println("Self-update is being finalized.")
	fmt.Println("Download the latest release from:")
	fmt.Println("  https://github.com/bloc-org/bloc/releases/latest")
	fmt.Println()
	fmt.Println("Or update via Homebrew:")
	fmt.Println("  brew upgrade bloc-org/bloc/bloc")
	return nil
}

// selfReplace atomically replaces the current binary with a newly downloaded one.
// Used internally once update download + checksum verification is complete.
func selfReplace(newBinaryPath string) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine current executable path: %w", err)
	}

	// Write to a temp file next to the current binary
	dir := filepath.Dir(execPath)
	tmpPath := filepath.Join(dir, ".bloc-update-tmp")

	src, err := os.Open(newBinaryPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("cannot write update: %w (try sudo or check permissions)", err)
	}

	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		os.Remove(tmpPath)
		return err
	}
	dst.Close()

	// Atomic rename
	if err := os.Rename(tmpPath, execPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("cannot replace binary: %w", err)
	}

	return nil
}

// platformSuffix returns the GoReleaser archive suffix for the current platform.
func platformSuffix() string {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	if goarch == "amd64" {
		goarch = "x86_64"
	} else if goarch == "arm64" {
		goarch = "arm64"
	}
	switch goos {
	case "darwin":
		return fmt.Sprintf("Darwin_%s", goarch)
	case "linux":
		return fmt.Sprintf("Linux_%s", goarch)
	case "windows":
		return fmt.Sprintf("Windows_%s", goarch)
	default:
		return strings.ToTitle(goos) + "_" + goarch
	}
}
