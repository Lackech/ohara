package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ohara",
	Short: "Ohara — Agent-optimized documentation CLI",
	Long:  "Ohara CLI for managing Diataxis-structured documentation projects.\nCreate, validate, preview, and sync documentation with the Ohara platform.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringP("api-url", "", "https://api.ohara.dev", "Ohara API URL")
	rootCmd.PersistentFlags().StringP("token", "t", "", "API token for authentication")
}
