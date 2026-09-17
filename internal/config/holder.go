package config

import (
	"sync/atomic"
)

type Holder struct {
	ptr atomic.Pointer[Config]
}

func NewHolder(initial *Config) *Holder {
	h := &Holder{}
	if initial != nil {
		h.ptr.Store(initial)
	} else {
		h.ptr.Store(&Config{})
	}
	return h
}

func (h *Holder) Get() *Config {
	cfg := h.ptr.Load()
	if cfg == nil {
		return &Config{}
	}
	return cfg
}

func (h *Holder) Update(newCfg *Config) {
	if newCfg != nil {
		h.ptr.Store(newCfg)
	}
}
