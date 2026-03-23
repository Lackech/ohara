package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show project sync status",
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := loadToken()
		if err != nil {
			return err
		}

		apiURL, _ := cmd.Flags().GetString("api-url")

		req, _ := http.NewRequest("GET", apiURL+"/api/v1/projects", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to connect to API: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return fmt.Errorf("API returned status %d", resp.StatusCode)
		}

		var result struct {
			Data []struct {
				Name         string  `json:"name"`
				Slug         string  `json:"slug"`
				LastSyncedAt *string `json:"lastSyncedAt"`
			} `json:"data"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		if len(result.Data) == 0 {
			fmt.Println("No projects found.")
			return nil
		}

		fmt.Println("Projects:")
		for _, p := range result.Data {
			synced := "never"
			if p.LastSyncedAt != nil {
				synced = *p.LastSyncedAt
			}
			fmt.Printf("  %-20s /%s  (last sync: %s)\n", p.Name, p.Slug, synced)
		}

		return nil
	},
}

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search documentation",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := loadToken()
		if err != nil {
			return err
		}

		project, _ := cmd.Flags().GetString("project")
		if project == "" {
			return fmt.Errorf("--project flag required")
		}

		apiURL, _ := cmd.Flags().GetString("api-url")
		query := args[0]

		req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/search?query=%s&project=%s", apiURL, query, project), nil)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to connect to API: %w", err)
		}
		defer resp.Body.Close()

		var result struct {
			Data struct {
				Results []struct {
					Title       string `json:"title"`
					Path        string `json:"path"`
					DiataxisType string `json:"diataxisType"`
					Snippet     string `json:"snippet"`
				} `json:"results"`
			} `json:"data"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		if len(result.Data.Results) == 0 {
			fmt.Println("No results found.")
			return nil
		}

		for i, r := range result.Data.Results {
			fmt.Printf("%d. %s [%s]\n   %s\n\n", i+1, r.Title, r.DiataxisType, r.Path)
		}

		return nil
	},
}

func init() {
	searchCmd.Flags().StringP("project", "p", "", "Project slug")
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(searchCmd)
}
