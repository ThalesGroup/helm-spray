package cmd

import (
	"fmt"
	"github.com/gemalto/helm-spray/v4/internal/web"
	"github.com/spf13/cobra"
)

var (
	webAddr      string
	webChartDir  string
	webNamespace string
)

var webCmd = &cobra.Command{
	Use:   "web",
	Short: "Start the Helm Spray web GUI",
	Long:  `Start a web-based GUI for browsing charts and executing spray operations.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		server := web.NewServer(webAddr, webChartDir, webNamespace)
		fmt.Printf("Starting Helm Spray Web GUI on %s...\n", webAddr)
		return server.Start()
	},
}

func AddWebCommand(rootCmd *cobra.Command) {
	webCmd.Flags().StringVar(&webAddr, "addr", ":8080", "Address to listen on")
	webCmd.Flags().StringVar(&webChartDir, "chart-dir", ".", "Directory containing charts")
	webCmd.Flags().StringVar(&webNamespace, "namespace", "default", "Kubernetes namespace")
	rootCmd.AddCommand(webCmd)
}
