package pipeline

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/bloc-org/bloc/internal/hardware"
	"golang.org/x/term"
)

// ─── TTY confirmation helpers ──────────────────────────────────────────────────

// confirmPrompt shows prompt and returns true if the user answers y/yes/Enter.
//
// FIX-1 non-TTY safety:
//   - If isYes is true (--yes flag), returns true immediately (CI-safe opt-in).
//   - If stdin is not a terminal, returns false and prints a clear message.
//     Old behaviour was silent EOF→true, auto-approving dangerous prompts in CI.
//   - If stdin IS a terminal and user hits Ctrl+D (EOF), returns false.
func confirmPrompt(prompt string, isYes bool) bool {
	if isYes {
		fmt.Fprintf(os.Stderr, "%s [auto-confirmed via --yes]\n", prompt)
		return true
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintln(os.Stderr, "  [non-interactive] Prompt skipped — use --yes to auto-confirm in CI")
		return false
	}
	fmt.Fprint(os.Stderr, prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		ans := strings.ToLower(strings.TrimSpace(scanner.Text()))
		return ans == "y" || ans == "yes" || ans == ""
	}
	return false // EOF on a real TTY = Ctrl+D = treat as "no"
}

// confirmYesExplicit is like confirmPrompt but requires an explicit "y" or "yes".
// Enter alone is treated as "no". Used for high-risk gates (trust_remote_code).
func confirmYesExplicit(prompt string, isYes bool) bool {
	if isYes {
		fmt.Fprintf(os.Stderr, "%s [auto-confirmed via --yes]\n", prompt)
		return true
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintln(os.Stderr, "  [non-interactive] Security prompt skipped — use --yes to auto-confirm in CI")
		return false
	}
	fmt.Fprint(os.Stderr, prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		ans := strings.ToLower(strings.TrimSpace(scanner.Text()))
		return ans == "y" || ans == "yes"
	}
	return false
}

// ─── Disk space helpers ────────────────────────────────────────────────────────

// checkDiskSpace verifies there is enough free space in dir to download a file
// of sizeGB gigabytes. Returns an error with a clear message if not.
// Silent success if sizeGB is 0 (unknown size).
func checkDiskSpace(dir string, sizeGB float64) error {
	if sizeGB <= 0 {
		return nil
	}
	freeBytes, err := hardware.FreeSpaceBytes(dir)
	if err != nil {
		return nil // ignore stat errors — don't block the download
	}
	freeGB := float64(freeBytes) / 1e9
	requiredGB := sizeGB * 1.1 // 10% headroom
	if freeGB < requiredGB {
		fmt.Println()
		fmt.Printf("  \033[33m⚠  Warning:\033[0m This model is ~%.1f GB. You have %.1f GB free.\n", sizeGB, freeGB)
		fmt.Println("     Run 'bloc models prune' to free space.")
		return fmt.Errorf("cancelled due to low disk space: need %.1f GB, only %.1f GB free", sizeGB, freeGB)
	}
	return nil
}

// ─── Engine log file helpers ──────────────────────────────────────────────────

// openEngineLogFile creates a named log file under cacheDir/logs/.
// FIX-4: Replaces os.CreateTemp("", "bloc-engine-*.log") which put logs in
// an ephemeral /tmp directory that is cleared on reboot.
func openEngineLogFile(cacheDir, recipeName string) (*os.File, error) {
	logDir := logDirPath(cacheDir)
	if err := os.MkdirAll(logDir, 0750); err != nil {
		return nil, fmt.Errorf("cannot create log dir: %w", err)
	}
	slug := sanitizeLogSlug(recipeName)
	logName := fmt.Sprintf("engine-%s-%s.log", slug, time.Now().Format("20060102-150405"))
	logPath := logDirPath(cacheDir) + "/" + logName
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0640)
	if err != nil {
		return nil, fmt.Errorf("cannot create log file: %w", err)
	}
	return f, nil
}

// pruneEngineLogs removes all but the `keep` most-recent engine-*.log files
// from logDir. Silent on errors — log rotation failure is never fatal.
// Fix #5: Sort by modification time (not filename) so that logs for recipes
// whose name sorts early alphabetically (e.g. "engine-gemma-...") are not
// pruned before older logs for recipes that sort later (e.g. "engine-llama-...").
func pruneEngineLogs(logDir string, keep int) error {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return err
	}
	type logEntry struct {
		path  string
		mtime time.Time
	}
	var logs []logEntry
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "engine-") && strings.HasSuffix(e.Name(), ".log") {
			info, infoErr := e.Info()
			if infoErr != nil {
				continue
			}
			logs = append(logs, logEntry{
				path:  filepath.Join(logDir, e.Name()),
				mtime: info.ModTime(),
			})
		}
	}
	// Sort ascending by modification time — oldest first, newest last.
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].mtime.Before(logs[j].mtime)
	})
	// Remove oldest entries until we are within the keep limit.
	for len(logs) > keep {
		_ = os.Remove(logs[0].path)
		logs = logs[1:]
	}
	return nil
}

// logDirPath returns the engine log directory path for the given cache dir.
func logDirPath(cacheDir string) string {
	return cacheDir + "/logs"
}

// sanitizeLogSlug converts a recipe name into a safe filename component.
var logSlugRe = regexp.MustCompile(`[^a-zA-Z0-9\-.]+`)

func sanitizeLogSlug(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "unknown"
	}
	slug := logSlugRe.ReplaceAllString(name, "-")
	if len(slug) > 48 {
		slug = slug[:48]
	}
	return slug
}
