package config

import (
	"fmt"
	"log/slog"
)

func (m *Manager) Reload(configPath string) (*Config, error) {
	if err := m.v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to re-read config file: %w", err)
	}

	var newCfg Config
	if err := m.v.Unmarshal(&newCfg); err != nil {
		return nil, fmt.Errorf("failed to decode reloaded config: %w", err)
	}

	applyDynamicDefaults(&newCfg)

	if err := validateConfig(&newCfg); err != nil {
		return nil, fmt.Errorf("reloaded config validation failed: %w", err)
	}

	slog.Debug("Config file re-parsed and validated successfully")
	return &newCfg, nil
}
