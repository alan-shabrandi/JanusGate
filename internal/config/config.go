package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server ServerConfig  `mapstructure:"server" json:"server" yaml:"server"`
	Routes []RouteConfig `mapstructure:"routes" json:"routes" yaml:"routes"`
}
type ServerConfig struct {
	Port         int           `mapstructure:"port" json:"port" yaml:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout" json:"read_timeout" yaml:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout" json:"write_timeout" yaml:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout" json:"idle_timeout" yaml:"idle_timeout"`
}
type RouteConfig struct {
	ID          string         `mapstructure:"id" json:"id" yaml:"id"`
	Path        string         `mapstructure:"path" json:"path" yaml:"path"`
	Methods     []string       `mapstructure:"methods" json:"methods" yaml:"methods"`
	StripPrefix bool           `mapstructure:"strip_prefix" json:"strip_prefix" yaml:"strip_prefix"`
	Upstreams   []UpstreamNode `mapstructure:"upstreams" json:"upstreams" yaml:"upstreams"`
}
type UpstreamNode struct {
	ID     string `mapstructure:"id" json:"id" yaml:"id"`
	URL    string `mapstructure:"url" json:"url" yaml:"url"`
	Weight int    `mapstructure:"weight" json:"weight" yaml:"weight"`
}

func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()

	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", "5s")
	v.SetDefault("server.write_timeout", "10s")
	v.SetDefault("server.idle_timeout", "120s")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}
