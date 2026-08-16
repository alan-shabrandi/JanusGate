package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

type Config struct {
	Server ServerConfig  `mapstructure:"server" json:"server" yaml:"server"`
	Redis  RedisConfig   `mapstructure:"redis" json:"redis" yaml:"redis"`
	Auth   AuthConfig    `mapstructure:"auth" json:"auth" yaml:"auth"`
	Routes []RouteConfig `mapstructure:"routes" json:"routes" yaml:"routes"`
}

type ServerConfig struct {
	Port         int           `mapstructure:"port" json:"port" yaml:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout" json:"read_timeout" yaml:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout" json:"write_timeout" yaml:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout" json:"idle_timeout" yaml:"idle_timeout"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr" json:"addr" yaml:"addr"`
	Password string `mapstructure:"password" json:"password" yaml:"password"`
	DB       int    `mapstructure:"db" json:"db" yaml:"db"`
}

type AuthConfig struct {
	JWTSecret   string        `mapstructure:"jwt_secret" json:"jwt_secret" yaml:"jwt_secret"`
	TokenExpiry time.Duration `mapstructure:"token_expiry" json:"token_expiry" yaml:"token_expiry"`
}

type RouteConfig struct {
	ID           string           `mapstructure:"id" json:"id" yaml:"id"`
	PathPrefix   string           `mapstructure:"path_prefix" json:"path_prefix" yaml:"path_prefix"`
	MatchType    string           `mapstructure:"match_type" json:"match_type" yaml:"match_type"`
	Methods      []string         `mapstructure:"methods" json:"methods" yaml:"methods"`
	StripPrefix  bool             `mapstructure:"strip_prefix" json:"strip_prefix" yaml:"strip_prefix"`
	RequiresAuth bool             `mapstructure:"requires_auth" json:"requires_auth" yaml:"requires_auth"`
	Timeout      time.Duration    `mapstructure:"timeout" json:"timeout" yaml:"timeout"`
	LBStrategy   string           `mapstructure:"lb_strategy" json:"lb_strategy" yaml:"lb_strategy"`
	Retry        RetryConfig      `mapstructure:"retry" json:"retry" yaml:"retry"`
	Upstreams    []UpstreamConfig `mapstructure:"upstreams" json:"upstreams" yaml:"upstreams"`
}

type UpstreamConfig struct {
	ID     string `mapstructure:"id" json:"id" yaml:"id"`
	URL    string `mapstructure:"url" json:"url" yaml:"url"`
	Weight int    `mapstructure:"weight" json:"weight" yaml:"weight"`
}

type RetryConfig struct {
	Attempts        int           `mapstructure:"attempts" json:"attempts" yaml:"attempts"`
	InitialInterval time.Duration `mapstructure:"initial_interval" json:"initial_interval" yaml:"initial_interval"`
	MaxInterval     time.Duration `mapstructure:"max_interval" json:"max_interval" yaml:"max_interval"`
}

type Manager struct {
	v *viper.Viper
}

func Load(configPath string) (*Config, *Manager, error) {
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

	setStaticDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFoundError) {
			return nil, nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, nil, fmt.Errorf("unable to decode config into struct: %w", err)
	}

	applyDynamicDefaults(&cfg)

	if err := validateConfig(&cfg); err != nil {
		return nil, nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, &Manager{v: v}, nil
}

func (m *Manager) Watch(onChange func(cfg *Config)) {
	m.v.OnConfigChange(func(e fsnotify.Event) {
		slog.Info("Configuration file change detected", "file", e.Name)

		var newCfg Config
		if err := m.v.Unmarshal(&newCfg); err != nil {
			slog.Error("Failed to unmarshal new config during hot-reload", "error", err)
			return
		}

		applyDynamicDefaults(&newCfg)

		if err := validateConfig(&newCfg); err != nil {
			slog.Error("Invalid configuration detected during hot-reload. Changes ignored.", "error", err)
			return
		}

		slog.Info("Configuration successfully reloaded")
		onChange(&newCfg)
	})
	m.v.WatchConfig()
}

func setStaticDefaults(v *viper.Viper) {
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", 5*time.Second)
	v.SetDefault("server.write_timeout", 10*time.Second)
	v.SetDefault("server.idle_timeout", 120*time.Second)
	v.SetDefault("redis.addr", "localhost:6379")
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)
	v.SetDefault("auth.jwt_secret", "janusgate-default-secret-key-change-in-production")
	v.SetDefault("auth.token_expiry", 15*time.Minute)
}

func applyDynamicDefaults(cfg *Config) {
	for i := range cfg.Routes {
		if cfg.Routes[i].LBStrategy == "" {
			cfg.Routes[i].LBStrategy = "round_robin"
		}
		if cfg.Routes[i].Timeout <= 0 {
			cfg.Routes[i].Timeout = 5 * time.Second
		}
		if cfg.Routes[i].Retry.Attempts > 0 {
			if cfg.Routes[i].Retry.InitialInterval <= 0 {
				cfg.Routes[i].Retry.InitialInterval = 100 * time.Millisecond
			}
			if cfg.Routes[i].Retry.MaxInterval <= 0 {
				cfg.Routes[i].Retry.MaxInterval = 2 * time.Second
			}
		}
		for j := range cfg.Routes[i].Upstreams {
			if cfg.Routes[i].Upstreams[j].Weight <= 0 {
				cfg.Routes[i].Upstreams[j].Weight = 1
			}
		}
	}
}

func validateConfig(cfg *Config) error {
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", cfg.Server.Port)
	}

	if len(cfg.Routes) == 0 {
		return errors.New("at least one route must be defined")
	}

	for i, route := range cfg.Routes {
		if route.PathPrefix == "" {
			return fmt.Errorf("route [%d] (%s): path_prefix cannot be empty", i, route.ID)
		}

		if route.MatchType != "" && route.MatchType != "exact" && route.MatchType != "prefix" {
			return fmt.Errorf("route [%d] (%s): invalid match_type '%s', must be 'exact' or 'prefix'", i, route.ID, route.MatchType)
		}

		if len(route.Upstreams) == 0 {
			return fmt.Errorf("route [%d] (%s): must have at least one upstream", i, route.ID)
		}

		for j, upstream := range route.Upstreams {
			if upstream.URL == "" {
				return fmt.Errorf("upstream [%d] in route (%s): URL cannot be empty", j, route.ID)
			}
			if _, err := url.ParseRequestURI(upstream.URL); err != nil {
				return fmt.Errorf("upstream [%d] in route (%s): invalid URL format '%s'", j, route.ID, upstream.URL)
			}
		}
	}

	return nil
}
