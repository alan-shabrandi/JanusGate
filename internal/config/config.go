package config

import (
	"errors"
	"fmt"
	"strings"
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

	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./configs")
	}

	v.SetEnvPrefix("JANUS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFoundError) {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to decode config into struct: %w", err)
	}

	if err := validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", "5s")
	v.SetDefault("server.write_timeout", "10s")
	v.SetDefault("server.idle_timeout", "120s")
}

func validateConfig(cfg *Config) error {
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", cfg.Server.Port)
	}

	if len(cfg.Routes) == 0 {
		return errors.New("at least one route must be defined")
	}

	for _, route := range cfg.Routes {
		if route.Path == "" {
			return fmt.Errorf("route %s: path cannot be empty", route.ID)
		}
		if len(route.Upstreams) == 0 {
			return fmt.Errorf("route %s: must have at least one upstream", route.ID)
		}
		for _, upstream := range route.Upstreams {
			if upstream.URL == "" {
				return fmt.Errorf("upstream %s in route %s: URL cannot be empty", upstream.ID, route.ID)
			}
		}
	}

	return nil
}
