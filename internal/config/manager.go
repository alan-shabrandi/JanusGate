package config

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

type Manager struct {
	mu         sync.Mutex
	v          *viper.Viper
	configPath string
}

func (m *Manager) Reload() (*Config, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	v := viper.New()
	if m.configPath != "" {
		v.SetConfigFile(m.configPath)
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
		return nil, fmt.Errorf("failed to re-read config file: %w", err)
	}

	var newCfg Config
	if err := v.Unmarshal(&newCfg); err != nil {
		return nil, fmt.Errorf("failed to decode reloaded config: %w", err)
	}

	applyDynamicDefaults(&newCfg)

	if err := validateConfig(&newCfg); err != nil {
		return nil, fmt.Errorf("reloaded config validation failed: %w", err)
	}

	m.v = v
	slog.Debug("Config file re-parsed and validated successfully")
	return &newCfg, nil
}
