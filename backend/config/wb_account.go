package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// WBAccountConfig stores WB Stream account credentials for the keeper bot.
type WBAccountConfig struct {
	Enabled         bool   `json:"enabled"`
	Cookies         string `json:"cookies"`
	AccessToken     string `json:"access_token"`
	UserAgent       string `json:"user_agent"`
	DisplayName     string `json:"display_name"`      // custom name; random if empty
	IntervalSec     int    `json:"interval_sec"`      // seconds between full cycles
	StayDurationSec int    `json:"stay_duration_sec"` // seconds to stay in each room
}

// AccountStore persists the WB account config to disk.
type AccountStore struct {
	mu       sync.RWMutex
	cfg      WBAccountConfig
	filePath string
}

// NewAccountStore creates or loads the account config.
func NewAccountStore(dataDir string) (*AccountStore, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	s := &AccountStore{
		filePath: filepath.Join(dataDir, "wb_account.json"),
	}
	if err := s.load(); err != nil {
		return nil, fmt.Errorf("load: %w", err)
	}
	return s, nil
}

func (s *AccountStore) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &s.cfg)
}

func (s *AccountStore) save() error {
	s.mu.RLock()
	data, err := json.MarshalIndent(s.cfg, "", "  ")
	s.mu.RUnlock()
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

// Set updates and persists the config.
func (s *AccountStore) Set(cfg WBAccountConfig) error {
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
	return s.save()
}

// Get returns the current config.
func (s *AccountStore) Get() WBAccountConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}
