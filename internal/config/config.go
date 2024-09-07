package config

import (
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"go.uber.org/zap"
)

type Config struct {
	MetadataUpdateInterval time.Duration `toml:"metadata_update_interval"`
	CompactionInterval     time.Duration `toml:"compaction_interval"`
	BroadcastBufferDepth   int           `toml:"broadcast_buffer_depth"`
	ListenAddr             string        `toml:"listen_addr"`
	WatchDir               string        `toml:"watch_dir"`
}

var (
	globalConfig *Config
	once         sync.Once
)

var DEFAULT_CONFIG = Config{
	MetadataUpdateInterval: 500 * time.Millisecond,
	CompactionInterval:     1 * time.Minute,
	BroadcastBufferDepth:   1000,
	ListenAddr:             "localhost:16000",
	WatchDir:               "/tmp/fsd",
}

// InitConfig initializes the global config
func InitConfig() {
	once.Do(func() {
		globalConfig = loadConfig()
	})
}

// GetConfig returns the global config instance
func GetConfig() *Config {
	if globalConfig == nil {
		zap.L().Fatal("Config not initialized. Call InitConfig() first.")
	}
	return globalConfig
}

// loadConfig reads the configuration file and returns a Config struct.
// If the config file doesn't exist, it creates one with the default configuration.
// If the config file can't be parsed, it returns the default configuration.
func loadConfig() *Config {
	// Get the current user to determine the home directory
	currentUser, err := user.Current()
	if err != nil {
		zap.L().Fatal("failed to get current user", zap.Error(err))
	}

	// Construct the path to the config file
	configFilePath := filepath.Join(currentUser.HomeDir, ".fsd", "config.toml")

	// Check if the config file exists
	if _, err := os.Stat(configFilePath); errors.Is(err, os.ErrNotExist) {
		// If it doesn't exist, create the directory for the config file
		configDir := filepath.Dir(configFilePath)
		if err = os.MkdirAll(configDir, 0700); err != nil {
			zap.L().Fatal("failed to create config directory", zap.Error(err))
		}

		// Create the config file with default configuration
		file, err := os.Create(configFilePath)
		if err != nil {
			zap.L().Fatal("failed to create config file", zap.Error(err))
		}
		defer file.Close()

		encoder := toml.NewEncoder(file)
		if err := encoder.Encode(DEFAULT_CONFIG); err != nil {
			zap.L().Fatal("failed to write default config to file", zap.Error(err))
		}

		zap.L().Info("created default config file", zap.String("path", configFilePath))
		return &DEFAULT_CONFIG
	}

	zap.L().Info("using existing config file", zap.String("path", configFilePath))

	// Attempt to decode the existing config file
	var config Config
	if _, err := toml.DecodeFile(configFilePath, &config); err != nil {
		// If decoding fails, log a warning and return the default configuration
		zap.L().Warn("failed to decode config file, using default config", zap.Error(err))
		return &DEFAULT_CONFIG
	}

	// Return the successfully loaded configuration
	return &config
}

func GetDBPath() string {
	currentUser, err := user.Current()
	if err != nil {
		zap.L().Fatal("failed to get current user", zap.Error(err))
	}

	return filepath.Join(currentUser.HomeDir, ".fsd", "fsd.db")
}
