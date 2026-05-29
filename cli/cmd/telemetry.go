package cmd

import (
	"fmt"

	"github.com/bloc-org/bloc/internal/config"
	"github.com/spf13/cobra"
)

var telemetryCmd = &cobra.Command{
	Use:   "telemetry [on|off|show]",
	Short: "Manage anonymous telemetry settings",
	Long: `Control whether bloc sends anonymous usage data to bloc-theta.vercel.app.

  bloc telemetry on     Enable telemetry
  bloc telemetry off    Disable telemetry
  bloc telemetry show   Show current status

Telemetry data includes: CLI version, OS, recipe success/failure, tokens/sec.
It never includes: file paths, model content, hostnames, or IP addresses.

You can also set BLOC_NO_TELEMETRY=1 to disable permanently via environment.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTelemetry,
}

func runTelemetry(cmd *cobra.Command, args []string) error {
	t, err := config.LoadTelemetry()
	if err != nil {
		return err
	}

	if len(args) == 0 || args[0] == "show" {
		if !t.ConsentGiven {
			fmt.Println("Telemetry: not yet configured (will prompt on next deploy)")
		} else if t.Enabled {
			fmt.Println("Telemetry: \033[32menabled\033[0m")
		} else {
			fmt.Println("Telemetry: \033[31mdisabled\033[0m")
		}
		return nil
	}

	switch args[0] {
	case "on":
		t.Enabled = true
		t.ConsentGiven = true
		if err := config.SaveTelemetry(t); err != nil {
			return err
		}
		fmt.Println("✓ Telemetry enabled. Thank you for helping improve Bloc!")
	case "off":
		t.Enabled = false
		t.ConsentGiven = true
		if err := config.SaveTelemetry(t); err != nil {
			return err
		}
		fmt.Println("✓ Telemetry disabled.")
	default:
		return fmt.Errorf("unknown argument %q — use: on, off, or show", args[0])
	}
	return nil
}
