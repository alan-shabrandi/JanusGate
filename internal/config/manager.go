package config

import (
	"fmt"
	"log/slog"
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

	newCfg, v, err := parseConfig(m.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to reload config: %w", err)
	}

	m.v = v
	slog.Debug("Config file re-parsed and validated successfully")
	return newCfg, nil
}
