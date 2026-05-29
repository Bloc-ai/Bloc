package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Version is set at build time via ldflags: -X github.com/bloc-org/bloc/cmd.Version=0.1.0
var Version = "dev"
var BuildCommit = "unknown"
var BuildDate = "unknown"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the bloc CLI version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("bloc %s (%s) built %s\n", Version, BuildCommit, BuildDate)
		fmt.Printf("Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	},
}
