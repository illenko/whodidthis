package config

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Prometheus PrometheusConfig `mapstructure:"prometheus"`
	Discovery  DiscoveryConfig  `mapstructure:"discovery"`
	Scan       ScanConfig       `mapstructure:"scan"`
	Storage    StorageConfig    `mapstructure:"storage"`
	Server     ServerConfig     `mapstructure:"server"`
	Log        LogConfig        `mapstructure:"log"`
}

type PrometheusConfig struct {
	URL            string  `mapstructure:"url"`
	Username       string  `mapstructure:"username"`
	Password       string  `mapstructure:"password"`
	RateLimit      float64 `mapstructure:"rate_limit"`
	RateLimitBurst int     `mapstructure:"rate_limit_burst"`
}

type DiscoveryConfig struct {
	ServiceLabel string `mapstructure:"service_label"`
}

type ScanConfig struct {
	Interval          time.Duration `mapstructure:"interval"`
	SampleValuesLimit int           `mapstructure:"sample_values_limit"`
}

type StorageConfig struct {
	Path          string `mapstructure:"path"`
	RetentionDays int    `mapstructure:"retention_days"`
}

type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Host string `mapstructure:"host"`
}

type LogConfig struct {
	Level string `mapstructure:"level"` // debug, info, warn, error
}

func Load(path string) (*Config, error) {
	v := viper.New()

	if path != "" {
		v.SetConfigFile(path)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
	}

	v.SetEnvPrefix("WDT")
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		fmt.Println("Error reading config:", err)
		fmt.Println("Building config from env")
		return envConfig(v), nil
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

func envConfig(v *viper.Viper) *Config {
	return &Config{
		Prometheus: PrometheusConfig{
			URL:            v.GetString("prometheus_url"),
			RateLimit:      v.GetFloat64("prometheus_rate_limit"),
			RateLimitBurst: v.GetInt("prometheus_rate_limit_burst"),
		},
		Discovery: DiscoveryConfig{
			ServiceLabel: v.GetString("discovery_service_label"),
		},
		Scan: ScanConfig{
			Interval:          v.GetDuration("scan_interval"),
			SampleValuesLimit: v.GetInt("scan_sample_values_limit"),
		},
		Storage: StorageConfig{
			Path:          v.GetString("storage_path"),
			RetentionDays: v.GetInt("storage_retention_days"),
		},
		Server: ServerConfig{
			Port: v.GetInt("server_port"),
			Host: v.GetString("server_host"),
		},
		Log: LogConfig{
			Level: v.GetString("log_level"),
		},
	}
}

func (c *Config) Validate() error {
	if c.Prometheus.URL == "" {
		return fmt.Errorf("prometheus.url is required")
	}
	if c.Discovery.ServiceLabel == "" {
		return fmt.Errorf("discovery.service_label is required")
	}
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535")
	}
	return nil
}

func (c *Config) RetentionDuration() time.Duration {
	return time.Duration(c.Storage.RetentionDays) * 24 * time.Hour
}

func (c *Config) LogLevel() slog.Level {
	switch c.Log.Level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func ConfigFileExists(path string) bool {
	if path == "" {
		path = "config.yaml"
	}
	_, err := os.Stat(path)
	return err == nil
}
