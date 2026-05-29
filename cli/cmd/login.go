package cmd

import (
	"fmt"

	"github.com/bloc-org/bloc/internal/config"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with bloc-hub.com via GitHub OAuth",
	Long: `Opens a browser window to authenticate with GitHub OAuth.
Your credentials are saved locally in ~/.config/bloc/auth.json.`,
	RunE: runLogin,
}

func runLogin(cmd *cobra.Command, args []string) error {
	// Check if already logged in
	existing, err := config.LoadAuth()
	if err == nil && existing != nil {
		fmt.Printf("Already logged in as \033[32m%s\033[0m.\n", existing.Username)
		fmt.Println("Run 'bloc logout' first to switch accounts.")
		return nil
	}

	// TODO: Implement device flow OAuth
	// 1. POST https://bloc-hub.com/api/auth/device → returns device_code, user_code, verification_uri
	// 2. Print: "Open https://bloc-hub.com/auth/device and enter code: ABCD-1234"
	// 3. Poll /api/auth/device/token every 5s until granted or expired
	// 4. On success: save JWT + username to ~/.config/bloc/auth.json

	fmt.Println("🔐 Bloc Login")
	fmt.Println()
	fmt.Println("Opening bloc-hub.com for authentication...")
	fmt.Println()
	fmt.Println("  \033[33m[Coming soon]\033[0m Device flow OAuth is under development.")
	fmt.Println("  For now, you can browse and deploy recipes without logging in.")
	fmt.Println("  To publish recipes, open a PR to the bloc-product repository.")
	fmt.Println()
	fmt.Println("  Follow progress: https://github.com/bloc-org/bloc-product/issues")
	return nil
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear saved authentication credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.DeleteAuth(); err != nil {
			return fmt.Errorf("logout failed: %w", err)
		}
		fmt.Println("✓ Logged out. Credentials cleared from ~/.config/bloc/auth.json")
		return nil
	},
}
