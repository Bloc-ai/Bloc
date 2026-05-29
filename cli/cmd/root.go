package cmd

import (
	"fmt"
	"os"

	"github.com/bloc-org/bloc/internal/telemetry"
	"github.com/spf13/cobra"
)

func init() {
	// Propagate build-time version to telemetry package (avoids circular import)
	telemetry.CLIVersion = Version
}

var rootCmd = &cobra.Command{
	Use:   "bloc",
	Short: "Bloc — run AI models locally with community-crafted recipes",
	Long: `
██████╗ ██╗      ██████╗  ██████╗ 
██╔══██╗██║     ██╔═══██╗██╔════╝ 
██████╔╝██║     ██║   ██║██║      
██╔══██╗██║     ██║   ██║██║      
██████╔╝███████╗╚██████╔╝╚██████╗ 
╚═════╝ ╚══════╝ ╚═════╝  ╚═════╝ 

Bloc is a CLI tool for discovering, deploying, and running
local AI models using community-curated recipes from bloc-theta.vercel.app.

Run 'bloc help' to see all available commands.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(modelsCmd)
	rootCmd.AddCommand(telemetryCmd)
	rootCmd.AddCommand(updateCmd)
}
