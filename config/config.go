package config

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Prometheus      PrometheusConfig      `mapstructure:"prometheus"`
	Grafana         GrafanaConfig         `mapstructure:"grafana"`
	Collection      CollectionConfig      `mapstructure:"collection"`
	Teams           map[string]TeamConfig `mapstructure:"teams"`
	Recommendations RecommendationsConfig `mapstructure:"recommendations"`
	Server          ServerConfig          `mapstructure:"server"`
}

type PrometheusConfig struct {
	URL      string `mapstructure:"url"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

type GrafanaConfig struct {
	URL      string `mapstructure:"url"`
	APIToken string `mapstructure:"api_token"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

type CollectionConfig struct {
	Interval      time.Duration `mapstructure:"interval"`
	RetentionDays int           `mapstructure:"retention_days"`
}

type TeamConfig struct {
	MetricsPatterns []string `mapstructure:"metrics_patterns"`
}

type RecommendationsConfig struct {
	HighCardinalityThreshold int `mapstructure:"high_cardinality_threshold"`
	UnusedDaysThreshold      int `mapstructure:"unused_days_threshold"`
	MinCardinalityImpact     int `mapstructure:"min_cardinality_impact"`
}

type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Host string `mapstructure:"host"`
}

func Load(path string) (*Config, error) {
	v := viper.New()
	setDefaults(v)

	if path != "" {
		v.SetConfigFile(path)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
	}

	v.SetEnvPrefix("METRICCOST")
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return defaultConfig(), nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("prometheus.url", "http://localhost:9090")
	v.SetDefault("grafana.url", "http://localhost:3000")
	v.SetDefault("collection.interval", "24h")
	v.SetDefault("collection.retention_days", 90)
	v.SetDefault("recommendations.high_cardinality_threshold", 10000)
	v.SetDefault("recommendations.unused_days_threshold", 30)
	v.SetDefault("recommendations.min_cardinality_impact", 1000)
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.host", "0.0.0.0")
}

func defaultConfig() *Config {
	return &Config{
		Prometheus: PrometheusConfig{
			URL: "http://localhost:9090",
		},
		Grafana: GrafanaConfig{
			URL: "http://localhost:3000",
		},
		Collection: CollectionConfig{
			Interval:      24 * time.Hour,
			RetentionDays: 90,
		},
		Teams: make(map[string]TeamConfig),
		Recommendations: RecommendationsConfig{
			HighCardinalityThreshold: 10000,
			UnusedDaysThreshold:      30,
			MinCardinalityImpact:     1000,
		},
		Server: ServerConfig{
			Port: 8080,
			Host: "0.0.0.0",
		},
	}
}

func (c *Config) Validate() error {
	if c.Prometheus.URL == "" {
		return fmt.Errorf("prometheus.url is required")
	}
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535")
	}
	return nil
}

func (c *Config) RetentionDuration() time.Duration {
	return time.Duration(c.Collection.RetentionDays) * 24 * time.Hour
}

func ConfigFileExists(path string) bool {
	if path == "" {
		path = "config.yaml"
	}
	_, err := os.Stat(path)
	return err == nil
}
