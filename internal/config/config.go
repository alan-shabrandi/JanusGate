package config

import "time"

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

func LoadConfig() (*Config, error) {
	return &Config{
		Server: ServerConfig{
			Port:         8080,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
		Routes: []RouteConfig{
			{
				ID:          "user-service-route",
				Path:        "/users",
				Methods:     []string{"GET", "POST"},
				StripPrefix: true,
				Upstreams: []UpstreamNode{
					{
						ID:     "user-service-1",
						URL:    "http://localhost:8081",
						Weight: 1,
					},
				},
			},
		},
	}, nil
}
