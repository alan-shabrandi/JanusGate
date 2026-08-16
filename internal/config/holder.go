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
	}
	return h
}

func (h *Holder) Get() *Config {
	return h.ptr.Load()
}

func (h *Holder) Update(newCfg *Config) {
	if newCfg != nil {
		h.ptr.Store(newCfg)
	}
}
