package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	cfgFile string
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "metriccost",
	Short: "Observability metrics analyzer",
	Long: `metriccost is a tool for analyzing Prometheus/VictoriaMetrics metrics
and Grafana dashboards to identify optimization opportunities.

It helps you understand:
- Which metrics consume the most storage (high cardinality)
- Which metrics are unused
- Which Grafana dashboards are stale
- Team-level metrics breakdown`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is ./config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("metriccost v0.1.0")
	},
}
