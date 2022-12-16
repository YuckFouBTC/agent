package nodedata

import (
	"sync"
	"time"

	entities "github.com/bolt-observer/agent/entities"
)

type GlobalSettings struct {
	mutex sync.RWMutex
	data  map[string]Settings
}

func NewGlobalSettings() *GlobalSettings {
	return &GlobalSettings{
		mutex: sync.RWMutex{},
		data:  make(map[string]Settings),
	}
}

func (s *GlobalSettings) GetKeys() []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}

	return keys
}

func (s *GlobalSettings) Get(key string) Settings {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.data[key]
}

func (s *GlobalSettings) Set(key string, value Settings) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.data[key] = value
}

func (s *GlobalSettings) Delete(key string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.data, key)
}

type Settings struct {
	callback       entities.BalanceReportCallback
	getApi         entities.NewApiCall
	hash           uint64
	identifier     entities.NodeIdentifier
	interval       entities.Interval // Is this needed, it is not being set?
	lastCheck      time.Time
	lastGraphCheck time.Time
	lastReport     time.Time
	private        bool
	settings       entities.ReportingSettings
}
