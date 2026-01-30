package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new config file",
	Long:  `Creates a new config.yaml file with default settings in the current directory.`,
	RunE:  runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	configPath := "config.yaml"

	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("config.yaml already exists, remove it first or use a different directory")
	}

	defaultConfig := `# metriccost configuration

prometheus:
  url: http://localhost:9090
  # Optional basic auth
  # username: ""
  # password: ""

grafana:
  url: http://localhost:3000
  api_token: ""  # Grafana service account token

collection:
  interval: 24h   # How often to scan
  retention: 90d  # How long to keep history in SQLite

size_model:
  bytes_per_sample: 2        # Prometheus TSDB average
  default_retention_days: 30
  scrape_interval: 15s       # Used to calculate samples/day

teams:
  # Example team configuration
  # backend-core:
  #   metrics_patterns:
  #     - "jvm_.*"
  #     - "http_server_.*"
  # integrations:
  #   metrics_patterns:
  #     - "integration_.*"
  #     - "payment_.*"

recommendations:
  high_cardinality_threshold: 10000
  unused_days_threshold: 30
  min_size_impact_mb: 100  # Don't show recommendations under 100MB impact

server:
  port: 8080
  host: 0.0.0.0
`

	if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Println("Created config.yaml")
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Edit config.yaml with your Prometheus/Grafana URLs")
	fmt.Println("  2. Run: metriccost scan")
	fmt.Println("  3. Run: metriccost serve")

	return nil
}
