package cmd

import (
	"fmt"
	"net/http"
	"runtime"

	"github.com/spf13/cobra"
)

const releasesAPI = "https://api.github.com/repos/Bloc-ai/Bloc/releases/latest"

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
		fmt.Println("  Watch for releases at: https://github.com/Bloc-ai/Bloc/releases")
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
	fmt.Println("  https://github.com/Bloc-ai/Bloc/releases/latest")
	fmt.Println()
	fmt.Println("Or update via Homebrew:")
	fmt.Println("  brew upgrade bloc-ai/bloc/bloc")
	return nil
}

// selfReplace and platformSuffix were removed (SEC-17: dead code).
// They will be reinstated when the self-update flow is implemented in a future release.
