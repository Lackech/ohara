package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Ohara",
	Long:  "Authenticate using an API key. Stores the token locally for subsequent commands.",
	RunE: func(cmd *cobra.Command, args []string) error {
		token, _ := cmd.Flags().GetString("token")

		if token == "" {
			fmt.Print("Enter your API key (ohara_...): ")
			fmt.Scanln(&token)
		}

		if token == "" {
			return fmt.Errorf("API key is required")
		}

		// Store token
		configDir, err := getConfigDir()
		if err != nil {
			return fmt.Errorf("failed to get config directory: %w", err)
		}

		if err := os.MkdirAll(configDir, 0700); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}

		config := map[string]string{
			"token": token,
		}

		data, _ := json.MarshalIndent(config, "", "  ")
		configFile := filepath.Join(configDir, "config.json")

		if err := os.WriteFile(configFile, data, 0600); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Println("✓ Authenticated successfully")
		fmt.Printf("  Token saved to %s\n", configFile)

		return nil
	},
}

func getConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ohara"), nil
}

func loadToken() (string, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(filepath.Join(configDir, "config.json"))
	if err != nil {
		return "", fmt.Errorf("not logged in. Run 'ohara login' first")
	}

	var config map[string]string
	if err := json.Unmarshal(data, &config); err != nil {
		return "", fmt.Errorf("invalid config file")
	}

	token, ok := config["token"]
	if !ok || token == "" {
		return "", fmt.Errorf("no token found. Run 'ohara login' first")
	}

	return token, nil
}

func init() {
	loginCmd.Flags().StringP("token", "t", "", "API key to authenticate with")
	rootCmd.AddCommand(loginCmd)
}
