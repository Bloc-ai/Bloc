package cmd

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

const releasesAPI = "https://api.github.com/repos/Bloc-ai/Bloc/releases/latest"

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update the bloc CLI to the latest version",
	Long: `Downloads the latest release from GitHub, verifies the SHA256 checksum,
and atomically replaces the current binary. If installed via Homebrew, upgrades using brew.`,
	RunE: runUpdate,
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name        string `json:"name"`
		DownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func runUpdate(cmd *cobra.Command, args []string) error {
	fmt.Printf("Current version: bloc %s\n", Version)
	fmt.Print("Checking for updates...")

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
	} else if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status checking updates: %s", resp.Status)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("cannot parse latest release: %w", err)
	}

	fmt.Println()

	if !isNewer(release.TagName, Version) {
		fmt.Printf("You are already on the latest version (%s).\n", Version)
		return nil
	}

	fmt.Printf("A new version is available: %s\n", release.TagName)

	if isHomebrewInstall() {
		return upgradeViaHomebrew()
	}

	// Manual self-update flow
	archiveName := getArchiveName()
	var downloadURL string
	var checksumURL string
	for _, asset := range release.Assets {
		switch asset.Name {
		case archiveName:
			downloadURL = asset.DownloadURL
		case "checksums.txt":
			checksumURL = asset.DownloadURL
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("could not find release binary for platform %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	// SEC-04: Checksum verification is mandatory.
	// The update must never proceed with an unverified binary — a MITM or
	// corrupted GitHub release would silently replace the running binary.
	// Fail hard if checksums.txt is absent or cannot be fetched.
	var expectedSHA256 string
	if checksumURL == "" {
		return fmt.Errorf(
			"update aborted: checksums.txt was not found in the release assets\n" +
				"  This may indicate a broken release. Please check https://github.com/bloc-org/bloc/releases\n" +
				"  or install manually.")
	}
	var checksumErr error
	expectedSHA256, checksumErr = fetchExpectedChecksum(checksumURL, archiveName)
	if checksumErr != nil {
		return fmt.Errorf(
			"update aborted: could not fetch checksums.txt: %w\n"+
				"  Cannot verify binary integrity without a checksum. Aborting for safety.\n"+
				"  Check your internet connection or install manually.",
			checksumErr)
	}

	isZip := strings.HasSuffix(archiveName, ".zip")
	tmpBinaryPath, err := downloadAndExtract(downloadURL, isZip, expectedSHA256, archiveName)
	if err != nil {
		return fmt.Errorf("update failed: %w", err)
	}
	defer os.Remove(tmpBinaryPath)

	if err := selfReplace(tmpBinaryPath); err != nil {
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	fmt.Println("Successfully updated bloc to the latest version!")
	return nil
}

func parseVersion(v string) (major, minor, patch int) {
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")
	if len(parts) > 0 {
		fmt.Sscanf(parts[0], "%d", &major)
	}
	if len(parts) > 1 {
		fmt.Sscanf(parts[1], "%d", &minor)
	}
	if len(parts) > 2 {
		pStr := parts[2]
		if idx := strings.IndexAny(pStr, "-+"); idx != -1 {
			pStr = pStr[:idx]
		}
		fmt.Sscanf(pStr, "%d", &patch)
	}
	return
}

func isNewer(latest, current string) bool {
	if current == "dev" {
		return true // Allow upgrading dev versions for testing/development
	}
	lMaj, lMin, lPat := parseVersion(latest)
	cMaj, cMin, cPat := parseVersion(current)
	if lMaj != cMaj {
		return lMaj > cMaj
	}
	if lMin != cMin {
		return lMin > cMin
	}
	return lPat > cPat
}

func isHomebrewInstall() bool {
	execPath, err := os.Executable()
	if err != nil {
		return false
	}
	resolved, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		resolved = execPath
	}
	return strings.Contains(resolved, "Cellar") || strings.Contains(resolved, "homebrew")
}

func upgradeViaHomebrew() error {
	fmt.Println("Detected Homebrew installation.")

	brewPath, err := exec.LookPath("brew")
	if err != nil {
		return fmt.Errorf("brew not found in PATH")
	}

	// SEC-08 (H-4): Validate brew path to prevent PATH hijack attacks during update.
	if !strings.HasPrefix(brewPath, "/opt/homebrew/") && !strings.HasPrefix(brewPath, "/usr/local/") && !strings.HasPrefix(brewPath, "/home/linuxbrew/") {
		return fmt.Errorf("brew binary at unexpected path %q — refusing to execute", brewPath)
	}

	fmt.Println("Running: brew upgrade bloc-ai/bloc/bloc")
	c := exec.Command(brewPath, "upgrade", "bloc-ai/bloc/bloc")
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func getArchiveName() string {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("bloc_%s_%s%s", goos, goarch, ext)
}

// downloadAndExtract fetches the archive, verifies its SHA256 (if expectedSHA256 is non-empty),
// then extracts the bloc binary into a temp file. Returns the temp file path.
//
// FIX-2: SHA256 verification is performed on the raw archive bytes before any
// extraction occurs, preventing a corrupted or tampered download from replacing
// the live binary.
func downloadAndExtract(url string, isZip bool, expectedSHA256, archiveName string) (string, error) {
	tmpArchive, err := os.CreateTemp("", "bloc-archive-*")
	if err != nil {
		return "", err
	}
	tmpArchiveName := tmpArchive.Name()
	defer os.Remove(tmpArchiveName)

	fmt.Printf("Downloading from: %s\n", url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		tmpArchive.Close()
		return "", err
	}
	req.Header.Set("User-Agent", "bloc-cli/"+Version)
	resp, err := apiClient.Do(req)
	if err != nil {
		tmpArchive.Close()
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tmpArchive.Close()
		return "", fmt.Errorf("bad status downloading archive: %s", resp.Status)
	}

	_, err = io.Copy(tmpArchive, resp.Body)
	tmpArchive.Close() // Close immediately to release file lock on Windows
	if err != nil {
		return "", err
	}

	// FIX-2: Verify SHA256 before extraction.
	if expectedSHA256 != "" {
		fmt.Printf("  Verifying SHA256 checksum...\n")
		if err := verifyArchiveChecksum(tmpArchiveName, expectedSHA256); err != nil {
			return "", err
		}
		fmt.Printf("  \033[32m✓\033[0m  Checksum verified.\n")
	}

	tmpBinary, err := os.CreateTemp("", "bloc-binary-*")
	if err != nil {
		return "", err
	}
	tmpBinaryName := tmpBinary.Name()
	tmpBinary.Close() // Close handle immediately so it can be safely written to by extractors

	var extractErr error
	if isZip {
		extractErr = extractZip(tmpArchiveName, tmpBinaryName)
	} else {
		archiveFile, err := os.Open(tmpArchiveName)
		if err != nil {
			os.Remove(tmpBinaryName)
			return "", err
		}
		extractErr = extractTarGz(archiveFile, tmpBinaryName)
		archiveFile.Close()
	}

	if extractErr != nil {
		os.Remove(tmpBinaryName)
		return "", extractErr
	}

	if err := os.Chmod(tmpBinaryName, 0755); err != nil {
		os.Remove(tmpBinaryName)
		return "", err
	}

	return tmpBinaryName, nil
}

// verifyArchiveChecksum hashes the file at archivePath with SHA256 and
// compares it against expectedSHA256 (hex string, case-insensitive).
// Returns a descriptive error on mismatch that instructs the user to retry.
func verifyArchiveChecksum(archivePath, expectedSHA256 string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("cannot open archive for checksum: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("cannot hash archive: %w", err)
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, expectedSHA256) {
		return fmt.Errorf(
			"checksum mismatch — download may be corrupted or tampered\n"+
				"  expected: %s\n"+
				"  got:      %s\n"+
				"  Delete any cached files and try again.",
			expectedSHA256, got,
		)
	}
	return nil
}

// fetchExpectedChecksum downloads checksums.txt and returns the SHA256 for archiveName.
// checksums.txt format: "<sha256>  <filename>" (one per line, as produced by sha256sum).
func fetchExpectedChecksum(checksumURL, archiveName string) (string, error) {
	req, err := http.NewRequest("GET", checksumURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "bloc-cli/"+Version)
	resp, err := apiClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %s fetching checksums.txt", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024)) // 64 KB max
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == archiveName {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("checksum for %q not found in checksums.txt", archiveName)
}

func extractTarGz(r io.Reader, destPath string) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		name := filepath.Base(header.Name)
		if header.Typeflag == tar.TypeReg && (name == "bloc" || name == "bloc.exe") {
			destFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return err
			}
			// SEC-13 (L-1): Mitigate tar bomb attacks by limiting extracted file size
			const maxBinarySize = 500 * 1024 * 1024 // 500MB
			limitReader := io.LimitReader(tr, maxBinarySize+1)
			n, err := io.Copy(destFile, limitReader)
			destFile.Close()
			if err != nil {
				return err
			}
			if n > maxBinarySize {
				return fmt.Errorf("downloaded binary exceeds maximum expected size (500 MB); aborting update")
			}
			info, err := os.Stat(destPath)
			if err != nil || info.Size() == 0 {
				return fmt.Errorf("downloaded binary is empty or unreadable; aborting update")
			}
			return nil
		}
	}
	return fmt.Errorf("binary not found in tar.gz archive")
}

func extractZip(zipPath, destPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		name := filepath.Base(f.Name)
		if name == "bloc" || name == "bloc.exe" {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			destFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				rc.Close()
				return err
			}
			// SEC-13 (L-1): Mitigate zip bomb attacks by limiting extracted file size
			const maxBinarySize = 500 * 1024 * 1024 // 500MB
			limitReader := io.LimitReader(rc, maxBinarySize+1)
			n, err := io.Copy(destFile, limitReader)
			rc.Close()
			destFile.Close()
			if err != nil {
				return err
			}
			if n > maxBinarySize {
				return fmt.Errorf("downloaded binary exceeds maximum expected size (500 MB); aborting update")
			}
			info, err := os.Stat(destPath)
			if err != nil || info.Size() == 0 {
				return fmt.Errorf("downloaded binary is empty or unreadable; aborting update")
			}
			return nil
		}
	}
	return fmt.Errorf("binary not found in zip archive")
}

func selfReplace(newBinaryPath string) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine current executable path: %w", err)
	}

	resolvedPath, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		resolvedPath = execPath
	}

	dir := filepath.Dir(resolvedPath)
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

	if err := os.Rename(tmpPath, resolvedPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("cannot replace binary: %w", err)
	}

	return nil
}
