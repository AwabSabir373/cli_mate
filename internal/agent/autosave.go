package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"cli_mate/internal/providers"
)

// AutoSaveConfig configures the auto-save behavior.
type AutoSaveConfig struct {
	Enabled  bool
	Interval time.Duration
	MaxSaves int
	Dir      string
}

// DefaultAutoSaveConfig returns the default auto-save configuration.
func DefaultAutoSaveConfig(dir string) AutoSaveConfig {
	return AutoSaveConfig{
		Enabled:  true,
		Interval: 5 * time.Minute,
		MaxSaves: 10,
		Dir:      dir,
	}
}

// AutoSaver automatically saves conversation state.
type AutoSaver struct {
	config   AutoSaveConfig
	messages []providers.Message
	mu       sync.Mutex
	stopCh   chan struct{}
	running  bool
}

// NewAutoSaver creates a new auto-saver.
func NewAutoSaver(config AutoSaveConfig) *AutoSaver {
	return &AutoSaver{
		config: config,
		stopCh: make(chan struct{}),
	}
}

// Start begins the auto-save loop.
func (as *AutoSaver) Start() {
	if !as.config.Enabled || as.running {
		return
	}

	as.running = true
	go as.saveLoop()
}

// Stop stops the auto-save loop.
func (as *AutoSaver) Stop() {
	if !as.running {
		return
	}

	as.running = false
	close(as.stopCh)
}

// UpdateMessages updates the messages to be auto-saved.
func (as *AutoSaver) UpdateMessages(messages []providers.Message) {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.messages = messages
}

// SaveNow performs an immediate save.
func (as *AutoSaver) SaveNow() error {
	as.mu.Lock()
	messages := as.messages
	as.mu.Unlock()

	if len(messages) == 0 {
		return nil
	}

	return as.save(messages)
}

func (as *AutoSaver) saveLoop() {
	ticker := time.NewTicker(as.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-as.stopCh:
			return
		case <-ticker.C:
			as.SaveNow()
		}
	}
}

func (as *AutoSaver) save(messages []providers.Message) error {
	if err := os.MkdirAll(as.config.Dir, 0700); err != nil {
		return err
	}

	// Create save data
	type SaveData struct {
		Messages []providers.Message `json:"messages"`
		SavedAt  time.Time           `json:"savedAt"`
	}

	data := SaveData{
		Messages: messages,
		SavedAt:  time.Now(),
	}

	// Generate filename with timestamp
	filename := "autosave_" + time.Now().Format("20060102_150405") + ".json"
	path := filepath.Join(as.config.Dir, filename)

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, jsonData, 0600); err != nil {
		return err
	}

	// Clean old saves
	as.cleanOldSaves()

	return nil
}

func (as *AutoSaver) cleanOldSaves() {
	entries, err := os.ReadDir(as.config.Dir)
	if err != nil {
		return
	}

	var saves []os.DirEntry
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			saves = append(saves, entry)
		}
	}

	// Remove oldest saves if we exceed MaxSaves
	if len(saves) > as.config.MaxSaves {
		toRemove := saves[:len(saves)-as.config.MaxSaves]
		for _, entry := range toRemove {
			os.Remove(filepath.Join(as.config.Dir, entry.Name()))
		}
	}
}

// LoadAutoSave loads the most recent auto-save.
func LoadAutoSave(dir string) ([]providers.Message, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var latestFile string
	var latestTime time.Time

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
			latestFile = entry.Name()
		}
	}

	if latestFile == "" {
		return nil, nil
	}

	data, err := os.ReadFile(filepath.Join(dir, latestFile))
	if err != nil {
		return nil, err
	}

	type SaveData struct {
		Messages []providers.Message `json:"messages"`
	}

	var save SaveData
	if err := json.Unmarshal(data, &save); err != nil {
		return nil, err
	}

	return save.Messages, nil
}
