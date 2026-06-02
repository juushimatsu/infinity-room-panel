package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// RoomConfig stores persistent room settings.
type RoomConfig struct {
	ID        string `json:"id"`
	Service   string `json:"service"`
	RoomInput string `json:"room_input"`
	BotCount  int    `json:"bot_count"`
	FileID    string `json:"file_id"`
	Loop      bool   `json:"loop"`
}

// RoomConfigStore persists room configurations to a JSON file.
type RoomConfigStore struct {
	mu       sync.RWMutex
	configs  map[string]RoomConfig
	filePath string
}

// NewRoomConfigStore creates a store backed by dataDir/room_configs.json.
func NewRoomConfigStore(dataDir string) (*RoomConfigStore, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	store := &RoomConfigStore{
		configs:  make(map[string]RoomConfig),
		filePath: filepath.Join(dataDir, "room_configs.json"),
	}
	if err := store.load(); err != nil {
		return nil, fmt.Errorf("load: %w", err)
	}
	return store, nil
}

func (s *RoomConfigStore) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var configs []RoomConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		return err
	}
	for _, c := range configs {
		s.configs[c.ID] = c
	}
	return nil
}

func (s *RoomConfigStore) save() error {
	s.mu.RLock()
	configs := make([]RoomConfig, 0, len(s.configs))
	for _, c := range s.configs {
		configs = append(configs, c)
	}
	s.mu.RUnlock()

	data, err := json.MarshalIndent(configs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

// Save writes or overwrites a room config.
func (s *RoomConfigStore) Save(config RoomConfig) error {
	s.mu.Lock()
	s.configs[config.ID] = config
	s.mu.Unlock()
	return s.save()
}

// Delete removes a room config.
func (s *RoomConfigStore) Delete(id string) error {
	s.mu.Lock()
	delete(s.configs, id)
	s.mu.Unlock()
	return s.save()
}

// Get returns a single config by ID.
func (s *RoomConfigStore) Get(id string) (RoomConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.configs[id]
	return c, ok
}

// GetAll returns all stored configs.
func (s *RoomConfigStore) GetAll() []RoomConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	configs := make([]RoomConfig, 0, len(s.configs))
	for _, c := range s.configs {
		configs = append(configs, c)
	}
	return configs
}
