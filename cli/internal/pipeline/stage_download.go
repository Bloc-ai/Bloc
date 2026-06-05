package pipeline

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/bloc-org/bloc/internal/config"
	"github.com/bloc-org/bloc/internal/downloader"
)

// DownloadModelStage downloads the model weights if not already cached.
// Skipped (with a simulated path) in --dry-run mode.
//
// Supports two model specs:
//   - HFRepo: full Hugging Face repository (all files).
//   - File + DownloadURL + SHA256: single file download.
//
// Sets: state.ModelPath
type DownloadModelStage struct{}

func (s *DownloadModelStage) Name() string { return "Checking model cache" }

func (s *DownloadModelStage) Run(ctx context.Context, state *RunState) error {
	dm, err := downloader.NewManager(state.CacheDir)
	if err != nil {
		return err
	}

	// Inject HuggingFace auth token if available.
	if hfCreds, hfErr := config.LoadHFAuth(); hfErr == nil && hfCreds != nil {
		dm.SetHFToken(hfCreds.Token)
	}

	r := state.Recipe

	if r.Model.HFRepo != "" {
		return s.ensureHFRepo(ctx, state, dm)
	}
	return s.ensureFile(ctx, state, dm)
}

func (s *DownloadModelStage) ensureHFRepo(ctx context.Context, state *RunState, dm *downloader.Manager) error {
	r := state.Recipe

	if dm.IsRepoCached(r.Model.HFRepo, "") {
		state.ModelPath = dm.RepoPath(r.Model.HFRepo, "main")
		fmt.Fprintf(os.Stderr, "  \033[32m✓\033[0m  Already cached: %s\n", state.ModelPath)
		return nil
	}

	if state.IsDryRun {
		state.ModelPath = dm.RepoPath(r.Model.HFRepo, "main")
		fmt.Fprintf(os.Stderr, "  \033[32m✓\033[0m  [Dry Run] Simulated model cache path: %s\n", state.ModelPath)
		return nil
	}

	if err := checkDiskSpace(state.CacheDir, r.Model.SizeGB); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "  Downloading HF repo %s (%.1f GB)...\n", r.Model.HFRepo, r.Model.SizeGB)
	bw := bufio.NewWriterSize(os.Stdout, 1024)
	modelPath, err := dm.EnsureRepoDownloaded(
		ctx,
		r.Model.HFRepo,
		"", // default revision "main"
		func(downloaded, total int64, speedMBs float64) {
			pct := float64(0)
			if total > 0 {
				pct = float64(downloaded) / float64(total) * 100
			}
			bar := progressBar(int(pct), 30)
			fmt.Fprintf(bw, "\r  %s %.1f/%.1f GB  [%s] %.0f%% @ %.1f MB/s",
				r.Model.HFRepo,
				float64(downloaded)/1e9,
				float64(total)/1e9,
				bar, pct, speedMBs,
			)
			bw.Flush()
		},
	)
	fmt.Fprintln(os.Stdout) // newline after progress bar
	if err != nil {
		return fmt.Errorf("repo download failed: %w", err)
	}

	state.ModelPath = modelPath
	fmt.Fprintf(os.Stderr, "  \033[32m✓\033[0m  Saved to %s\n", modelPath)
	return nil
}

func (s *DownloadModelStage) ensureFile(ctx context.Context, state *RunState, dm *downloader.Manager) error {
	r := state.Recipe

	cached, _ := dm.IsAlreadyCached(r.Model.File, r.Model.SHA256)
	if cached {
		state.ModelPath = dm.ModelPath(r.Model.File)
		fmt.Fprintf(os.Stderr, "  \033[32m✓\033[0m  Already cached: %s\n", state.ModelPath)
		return nil
	}

	if state.IsDryRun {
		state.ModelPath = dm.ModelPath(r.Model.File)
		fmt.Fprintf(os.Stderr, "  \033[32m✓\033[0m  [Dry Run] Simulated model cache path: %s\n", state.ModelPath)
		return nil
	}

	if err := checkDiskSpace(state.CacheDir, r.Model.SizeGB); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "  Downloading %s (%.1f GB)...\n", r.Model.File, r.Model.SizeGB)
	bw := bufio.NewWriterSize(os.Stdout, 1024)
	modelPath, err := dm.EnsureDownloaded(
		ctx,
		r.Model.File,
		r.Model.DownloadURL,
		r.Model.SHA256,
		r.Model.SizeGB,
		func(downloaded, total int64, speedMBs float64) {
			pct := float64(downloaded) / float64(total) * 100
			bar := progressBar(int(pct), 30)
			fmt.Fprintf(bw, "\r  %s %.1f/%.1f GB  [%s] %.0f%% @ %.1f MB/s",
				r.Model.File,
				float64(downloaded)/1e9,
				float64(total)/1e9,
				bar, pct, speedMBs,
			)
		},
	)
	fmt.Fprintln(os.Stdout)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	state.ModelPath = modelPath
	fmt.Fprintf(os.Stderr, "  \033[32m✓\033[0m  Saved to %s\n", modelPath)
	return nil
}

// ─── Shared display helpers ───────────────────────────────────────────────────

// progressBar returns a string of '=' chars padded with ' ' up to width.
func progressBar(pct, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := pct * width / 100
	bar := make([]byte, width)
	for i := range bar {
		if i < filled {
			bar[i] = '='
		} else {
			bar[i] = ' '
		}
	}
	return string(bar)
}
