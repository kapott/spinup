package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Configure providers and generate .env",
	Long: `Initialize continueplz configuration.

This interactive setup wizard will guide you through:
- Selecting which cloud GPU providers to configure
- Entering and validating API keys
- Generating WireGuard keys
- Setting default preferences

The configuration will be saved to a .env file in the current directory.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("continueplz init - Configuration wizard")
		fmt.Println("")
		fmt.Println("Interactive setup coming soon!")
		fmt.Println("")
		fmt.Println("This will help you configure:")
		fmt.Println("  - Provider API keys (Vast.ai, Lambda Labs, RunPod, etc.)")
		fmt.Println("  - WireGuard keys for secure tunneling")
		fmt.Println("  - Default preferences (model tier, region, etc.)")
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
