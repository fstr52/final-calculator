package config

import (
	"fmt"
	"os"

	"github.com/fstr52/final-calculator/internal/logger"

	"gopkg.in/yaml.v3"
)

type Loader interface {
	Load(configType, configPath string) (*Config, error)
}

type DefaultLoader struct {
	logger logger.Logger
}

func NewDefaultLoader(logger logger.Logger) Loader {
	return &DefaultLoader{
		logger: logger,
	}
}

// Загрузка конфига
//
// configPath - путь к файлу конфигурации
//
// configType - тип приложения (оркестратор или агент)
func (l *DefaultLoader) Load(configType, configPath string) (*Config, error) {
	l.logger.Debug("Started loading config",
		"configType", configType,
		"configPath", configPath)
	config := NewDefaultConfig()

	locator := NewConfigLocator(l.logger, configType, configPath)

	path, err := locator.Find()
	if err != nil {
		return nil, err
	} else {
		l.logger.Info("Loading config from file",
			"path", path)
		if err := l.loadFromFile(config, path); err != nil {
			l.logger.Error("Failed to load config from file",
				"path", path,
				"error", err)
			return nil, fmt.Errorf("failed to load config from file: %w", err)
		}
	}

	l.logger.Debug("Loading config from env")
	if err := LoadFromEnv(config); err != nil {
		l.logger.Error("Failed to load config from env",
			"error", err)
		return nil, fmt.Errorf("failed to load config from env: %w", err)
	}

	l.logger.Debug("Validating config")
	if err := config.Validate(); err != nil {
		l.logger.Error("Config not validated",
			"config", config.String())
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	l.logger.Info("Successfully loaded config",
		"config", config.String())
	return config, nil
}

// Загрузка конфигурации из файла
func (l *DefaultLoader) loadFromFile(config *Config, path string) error {
	l.logger.Debug("Started loadFromFile",
		"config", config.String(),
		"path", path)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		l.logger.Error("Config file not found",
			"path", path)
		return fmt.Errorf("config file not found: %s", path)
	}

	file, err := os.Open(path)
	if err != nil {
		l.logger.Error("Error opening file",
			"error", err)
		return err
	}
	defer file.Close()

	if err := yaml.NewDecoder(file).Decode(config); err != nil {
		l.logger.Error("Error decoding config",
			"error", err)
		return err
	}

	l.logger.Debug("loadFromFile completed")
	return nil
}
