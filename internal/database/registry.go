package database

import (
	"fmt"
	"sync"

	"github.com/bonheur/db-studio/internal/model"
)

type Factory func(config model.ConnectionConfig) (Engine, error)

var global = &Registry{
	factories: make(map[string]Factory),
}

type Registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

func Register(name string, factory Factory) {
	global.mu.Lock()
	defer global.mu.Unlock()
	if _, ok := global.factories[name]; ok {
		panic(fmt.Sprintf("database engine %q already registered", name))
	}
	global.factories[name] = factory
}

func Create(name string, config model.ConnectionConfig) (Engine, error) {
	global.mu.RLock()
	factory, ok := global.factories[name]
	global.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unsupported database engine: %s", name)
	}
	return factory(config)
}

func Available() []string {
	global.mu.RLock()
	defer global.mu.RUnlock()
	var names []string
	for n := range global.factories {
		names = append(names, n)
	}
	return names
}
