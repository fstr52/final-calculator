package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fstr52/final-calculator/internal/logger"
)

type ConfigLocator struct {
	Logger       logger.Logger
	ConfigType   string
	ProvidedPath string
}

func NewConfigLocator(logger logger.Logger, configType, providedPath string) *ConfigLocator {
	return &ConfigLocator{
		Logger:       logger,
		ConfigType:   configType,
		ProvidedPath: providedPath,
	}
}

// Нахождения файла конфигурации по заданному пути, в случае ненахода будет производиться поиск
// по стандартным путям
func (cl *ConfigLocator) Find() (string, error) {
	cl.Logger.Debug("Started cl.Find",
		"ConfigType", cl.ConfigType,
		"ProvidedPath", cl.ProvidedPath)

	if cl.ProvidedPath != "" {
		absPath, err := filepath.Abs(cl.ProvidedPath)
		if err == nil {
			if _, err := os.Stat(absPath); err == nil {
				cl.Logger.Debug("Found config file",
					"path", absPath)
				return absPath, nil
			}
		}
	}

	fileNames := getFileNames(cl.ConfigType)

	searchPaths := []string{
		"final-calculator/configs/",
		"./configs",
		filepath.Join(os.Getenv("HOME"), ".config/myapp/"),
	}

	for _, dir := range searchPaths {
		for _, name := range fileNames {
			path := filepath.Join(dir, name)
			if _, err := os.Stat(path); err == nil {
				absPath, err := filepath.Abs(path)
				if err == nil {
					cl.Logger.Debug("Found config file",
						"path", absPath)
					return absPath, nil
				}
			}
		}
	}

	cl.Logger.Error("No config file found")
	return "", fmt.Errorf("no config file found")
}

func getFileNames(configType string) []string {
	switch configType {
	case "orchestrator":
		return []string{"orchestrator.yml", "orchestrator.yaml", "config.yml", "config.yaml"}
	case "agent":
		return []string{"agent.yml", "agent.yaml", "agents.yml", "agents.yaml"}
	default:
		return []string{"config.yml", "config.yaml"}
	}
}
