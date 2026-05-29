package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/bloc-org/bloc/internal/config"
	"github.com/bloc-org/bloc/internal/downloader"
	"github.com/spf13/cobra"
)

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "Manage locally cached model weights",
	Long:  `List or clear locally cached GGUF model files.`,
	RunE:  runModels,
}

var modelsClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Delete all cached model files",
	RunE:  runModelsClear,
}

func init() {
	modelsCmd.AddCommand(modelsClearCmd)
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

	entries, err := dm.ListCached()
	if err != nil {
		return fmt.Errorf("cannot read cache: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No models cached. Run 'bloc deploy <recipe>' to download one.")
		return nil
	}

	fmt.Printf("\n%-50s %8s  %s\n", "MODEL FILE", "SIZE", "CACHED AT")
	fmt.Println(strings.Repeat("─", 80))
	var totalGB float64
	for _, e := range entries {
		sizeGB := float64(e.SizeBytes) / 1e9
		totalGB += sizeGB
		fmt.Printf("%-50s %6.1f GB  %s\n",
			e.FriendlyName,
			sizeGB,
			e.CachedAt.Format("2006-01-02"),
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

	entries, err := dm.ListCached()
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		fmt.Println("Cache is already empty.")
		return nil
	}

	var totalGB float64
	for _, e := range entries {
		totalGB += float64(e.SizeBytes) / 1e9
	}

	fmt.Printf("This will delete %.1f GB of cached model files.\n", totalGB)
	if !confirm("Continue? [y/N]: ") {
		fmt.Println("Cancelled.")
		return nil
	}

	if err := dm.ClearCache(); err != nil {
		return fmt.Errorf("cache clear failed: %w", err)
	}

	fmt.Fprintf(os.Stdout, "✓ Cache cleared.\n")
	return nil
}
