package cmd

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/Lackech/ohara/apps/cli/web"
	"github.com/spf13/cobra"
)

var viewCmd = &cobra.Command{
	Use:     "view",
	Aliases: []string{"dev:view"},
	Short:   "Start the local documentation viewer",
	Long: `Opens a web-based documentation viewer for your hub.
Renders all docs with search, navigation, and coverage dashboard.
Reads directly from hub files — no cloud needed.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		hubRoot, err := FindHubRoot(".")
		if err != nil {
			return fmt.Errorf("no ohara hub found. Run 'ohara init' first")
		}

		port, _ := cmd.Flags().GetInt("port")
		noBrowser, _ := cmd.Flags().GetBool("no-browser")

		if !noBrowser {
			url := fmt.Sprintf("http://localhost:%d", port)
			go openBrowser(url)
		}

		return web.Serve(hubRoot, port)
	},
}

func openBrowser(url string) {
	switch runtime.GOOS {
	case "darwin":
		exec.Command("open", url).Start()
	case "linux":
		exec.Command("xdg-open", url).Start()
	case "windows":
		exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	}
}

func init() {
	viewCmd.Flags().IntP("port", "p", 4000, "Port to serve on")
	viewCmd.Flags().Bool("no-browser", false, "Don't open browser automatically")
	rootCmd.AddCommand(viewCmd)
}
