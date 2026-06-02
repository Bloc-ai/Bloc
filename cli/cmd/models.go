package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bloc-org/bloc/internal/config"
	"github.com/bloc-org/bloc/internal/downloader"
	"github.com/spf13/cobra"
)

type CachedModel struct {
	Name         string    // e.g. "qwen2.5-0.5b-instruct.gguf" or "qwen/qwen1.5-1.8b-chat (main)"
	Path         string    // absolute path to file or directory
	IsRepo       bool      // true if full HF repo, false if single GGUF
	SizeBytes    int64     // size in bytes
	CachedAt     time.Time // when first downloaded
	LastUsedAt   time.Time // last deployed/accessed
	HFRepo       string    // empty if not repo
	Revision     string    // empty if not repo
	FriendlyName string    // for index deletion
}

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "Manage locally cached model weights",
	Long:  `List or clear locally cached GGUF model files and HuggingFace repositories.`,
	RunE:  runModels,
}

var modelsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all downloaded models, sizes, and last-used timestamps",
	RunE:  runModels,
}

var modelsClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Delete all cached model files",
	RunE:  runModelsClear,
}

var modelsPruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Interactive: select models to delete with size previews",
	RunE:  runModelsPrune,
}

func init() {
	modelsCmd.AddCommand(modelsListCmd)
	modelsCmd.AddCommand(modelsClearCmd)
	modelsCmd.AddCommand(modelsPruneCmd)
}

func getCachedModels(cacheDir string, dm *downloader.Manager) ([]CachedModel, error) {
	var models []CachedModel

	// 1. Single-file GGUF models from index
	entries, err := dm.ListCached()
	if err == nil {
		for _, e := range entries {
			mPath := dm.ModelPath(e.FriendlyName)
			lastUsed := e.CachedAt
			if info, statErr := os.Stat(mPath); statErr == nil {
				lastUsed = info.ModTime()
			}
			models = append(models, CachedModel{
				Name:         e.FriendlyName,
				Path:         mPath,
				IsRepo:       false,
				SizeBytes:    e.SizeBytes,
				CachedAt:     e.CachedAt,
				LastUsedAt:   lastUsed,
				FriendlyName: e.FriendlyName,
			})
		}
	}

	// 2. Full HF Repos from `repos/` dir
	reposDir := filepath.Join(cacheDir, "repos")
	orgDirs, err := os.ReadDir(reposDir)
	if err == nil {
		for _, orgDir := range orgDirs {
			if !orgDir.IsDir() {
				continue
			}
			orgNameEscaped := orgDir.Name()
			orgPath := filepath.Join(reposDir, orgNameEscaped)

			revDirs, err := os.ReadDir(orgPath)
			if err != nil {
				continue
			}
			for _, revDir := range revDirs {
				if !revDir.IsDir() {
					continue
				}
				revName := revDir.Name()
				revPath := filepath.Join(orgPath, revName)

				// Compute size and latest modification time (as last used)
				var sizeBytes int64
				var maxMtime time.Time
				_ = filepath.WalkDir(revPath, func(path string, d os.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if !d.IsDir() {
						info, err := d.Info()
						if err == nil {
							sizeBytes += info.Size()
							if info.ModTime().After(maxMtime) {
								maxMtime = info.ModTime()
							}
						}
					}
					return nil
				})

				if sizeBytes == 0 {
					continue
				}

				dirInfo, err := os.Stat(revPath)
				if err != nil {
					continue
				}
				if maxMtime.IsZero() {
					maxMtime = dirInfo.ModTime()
				}

				hfRepo := strings.ReplaceAll(orgNameEscaped, "--", "/")
				models = append(models, CachedModel{
					Name:       fmt.Sprintf("%s (%s)", hfRepo, revName),
					Path:       revPath,
					IsRepo:     true,
					SizeBytes:  sizeBytes,
					CachedAt:   dirInfo.ModTime(),
					LastUsedAt: maxMtime,
					HFRepo:     hfRepo,
					Revision:   revName,
				})
			}
		}
	}

	return models, nil
}

func runModels(cmd *cobra.Command, args []string) error {
	cacheDir, err := config.CacheDir()
	if err != nil {
		return err
	}

	dm, err := downloader.NewManager(cacheDir)
	if err != nil {
		return err
	}

	models, err := getCachedModels(cacheDir, dm)
	if err != nil {
		return fmt.Errorf("cannot read cache: %w", err)
	}

	if len(models) == 0 {
		fmt.Println("No models cached. Run 'bloc run <recipe>' to download one.")
		return nil
	}

	fmt.Printf("\n%-50s %-8s %8s  %s\n", "MODEL / REPOSITORY", "TYPE", "SIZE", "LAST USED")
	fmt.Println(strings.Repeat("─", 84))
	var totalGB float64
	for _, m := range models {
		sizeGB := float64(m.SizeBytes) / 1e9
		totalGB += sizeGB
		typeName := "File"
		if m.IsRepo {
			typeName = "Repo"
		}
		
		// Truncate name if too long for display
		dispName := m.Name
		if len(dispName) > 48 {
			dispName = dispName[:45] + "..."
		}
		fmt.Printf("%-50s %-8s %6.1f GB  %s\n",
			dispName,
			typeName,
			sizeGB,
			m.LastUsedAt.Format("2006-01-02"),
		)
	}
	fmt.Printf("\nTotal: %.1f GB in %s\n", totalGB, cacheDir)
	return nil
}

func runModelsClear(cmd *cobra.Command, args []string) error {
	cacheDir, err := config.CacheDir()
	if err != nil {
		return err
	}

	dm, err := downloader.NewManager(cacheDir)
	if err != nil {
		return err
	}

	models, err := getCachedModels(cacheDir, dm)
	if err != nil {
		return err
	}

	if len(models) == 0 {
		fmt.Println("Cache is already empty.")
		return nil
	}

	var totalGB float64
	for _, m := range models {
		totalGB += float64(m.SizeBytes) / 1e9
	}

	fmt.Printf("This will delete %.1f GB of cached model files and directories.\n", totalGB)
	if !confirm("Continue? [y/N]: ") {
		fmt.Println("Cancelled.")
		return nil
	}

	if err := dm.ClearCache(); err != nil {
		return fmt.Errorf("cache clear failed: %w", err)
	}

	// Also clear repos directory
	reposDir := filepath.Join(cacheDir, "repos")
	_ = os.RemoveAll(reposDir)
	_ = os.MkdirAll(reposDir, 0755)

	fmt.Fprintf(os.Stdout, "✓ Cache cleared.\n")
	return nil
}

func runModelsPrune(cmd *cobra.Command, args []string) error {
	cacheDir, err := config.CacheDir()
	if err != nil {
		return err
	}

	dm, err := downloader.NewManager(cacheDir)
	if err != nil {
		return err
	}

	models, err := getCachedModels(cacheDir, dm)
	if err != nil {
		return err
	}

	if len(models) == 0 {
		fmt.Println("No cached models to prune.")
		return nil
	}

	fmt.Println("\nCached Models:")
	for i, m := range models {
		typeName := "File"
		if m.IsRepo {
			typeName = "Repo"
		}
		sizeGB := float64(m.SizeBytes) / 1e9
		fmt.Printf("  [%d] %s (%s, %.1f GB, last used: %s)\n",
			i+1,
			m.Name,
			typeName,
			sizeGB,
			m.LastUsedAt.Format("2006-01-02"),
		)
	}

	fmt.Println()
	fmt.Print("Enter numbers to delete (e.g. 1,3 or 1-3, or 'all', 'q' to cancel): ")
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return nil
	}
	input := strings.TrimSpace(scanner.Text())
	if input == "" || strings.ToLower(input) == "q" {
		fmt.Println("Cancelled.")
		return nil
	}

	var selected []int
	if strings.ToLower(input) == "all" {
		for i := range models {
			selected = append(selected, i)
		}
	} else {
		// Parse comma separated values and ranges like 1-3
		parts := strings.Split(input, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if strings.Contains(part, "-") {
				rangeParts := strings.Split(part, "-")
				if len(rangeParts) == 2 {
					start, err1 := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
					end, err2 := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
					if err1 == nil && err2 == nil && start <= end {
						for idx := start; idx <= end; idx++ {
							selected = append(selected, idx-1)
						}
					}
				}
			} else {
				idx, err := strconv.Atoi(part)
				if err == nil {
					selected = append(selected, idx-1)
				}
			}
		}
	}

	// Validate indices and remove duplicates
	uniqueSelected := make(map[int]bool)
	var validSelected []int
	for _, idx := range selected {
		if idx >= 0 && idx < len(models) {
			if !uniqueSelected[idx] {
				uniqueSelected[idx] = true
				validSelected = append(validSelected, idx)
			}
		}
	}

	if len(validSelected) == 0 {
		fmt.Println("Invalid selection. No models selected to delete.")
		return nil
	}

	var totalGBToDelete float64
	fmt.Println("\nSelected models to delete:")
	for _, idx := range validSelected {
		m := models[idx]
		sizeGB := float64(m.SizeBytes) / 1e9
		totalGBToDelete += sizeGB
		fmt.Printf("  - %s (%.1f GB)\n", m.Name, sizeGB)
	}

	fmt.Printf("\nThis will free %.1f GB of space.\n", totalGBToDelete)
	if !confirm("Are you sure you want to delete these? [y/N]: ") {
		fmt.Println("Cancelled.")
		return nil
	}

	for _, idx := range validSelected {
		m := models[idx]
		if m.IsRepo {
			err = dm.DeleteCachedRepo(m.HFRepo, m.Revision)
			if err != nil {
				fmt.Printf("  \033[31m✗\033[0m  Failed to delete repo %s: %v\n", m.Name, err)
			} else {
				fmt.Printf("  \033[32m✓\033[0m  Deleted %s\n", m.Name)
			}
		} else {
			err = dm.DeleteCachedModel(m.FriendlyName)
			if err != nil {
				fmt.Printf("  \033[31m✗\033[0m  Failed to delete model %s: %v\n", m.Name, err)
			} else {
				fmt.Printf("  \033[32m✓\033[0m  Deleted %s\n", m.Name)
			}
		}
	}

	fmt.Println("✓ Selection pruned.")
	return nil
}
