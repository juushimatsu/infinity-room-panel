package storage

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

const MaxFileSize = 50 * 1024 * 1024 // 50 MB

type FileInfo struct {
	ID           string    `json:"id"`
	OriginalName string    `json:"filename"`
	Size         int64     `json:"size"`
	UploadedAt   time.Time `json:"uploaded_at"`
}

type Storage struct {
	baseDir     string
	metadataPath string
	mu          sync.RWMutex
	files       map[string]FileInfo
}

func NewStorage(baseDir string) (*Storage, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("create audio dir: %w", err)
	}
	s := &Storage{
		baseDir:      baseDir,
		metadataPath: filepath.Join(baseDir, "metadata.json"),
		files:        make(map[string]FileInfo),
	}
	if err := s.loadMetadata(); err != nil {
		return nil, fmt.Errorf("load metadata: %w", err)
	}
	return s, nil
}

func (s *Storage) loadMetadata() error {
	data, err := os.ReadFile(s.metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &s.files)
}

func (s *Storage) saveMetadata() error {
	data, err := json.MarshalIndent(s.files, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.metadataPath, data, 0644)
}

func (s *Storage) UploadFile(filename string, reader io.Reader) (string, error) {
	fileID := uuid.New().String()
	filePath := filepath.Join(s.baseDir, fileID+".mp3")

	f, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	lr := io.LimitReader(reader, MaxFileSize+1)
	written, err := io.Copy(f, lr)
	if err != nil {
		os.Remove(filePath)
		return "", fmt.Errorf("write file: %w", err)
	}
	if written > MaxFileSize {
		os.Remove(filePath)
		return "", fmt.Errorf("file exceeds maximum size of %d bytes", MaxFileSize)
	}

	info := FileInfo{
		ID:           fileID,
		OriginalName: filename,
		Size:         written,
		UploadedAt:   time.Now(),
	}

	s.mu.Lock()
	s.files[fileID] = info
	err = s.saveMetadata()
	s.mu.Unlock()

	if err != nil {
		os.Remove(filePath)
		return "", fmt.Errorf("save metadata: %w", err)
	}

	return fileID, nil
}

func (s *Storage) GetFilePath(fileID string) (string, error) {
	s.mu.RLock()
	_, ok := s.files[fileID]
	s.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("file not found: %s", fileID)
	}

	filePath := filepath.Join(s.baseDir, fileID+".mp3")
	if _, err := os.Stat(filePath); err != nil {
		return "", fmt.Errorf("file not accessible: %w", err)
	}
	return filePath, nil
}

func (s *Storage) ListFiles() []FileInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]FileInfo, 0, len(s.files))
	for _, info := range s.files {
		result = append(result, info)
	}
	return result
}
