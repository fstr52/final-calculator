package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDefaultConfig(t *testing.T) {
	cfg := NewDefaultConfig()
	assert.NotNil(t, cfg)
	assert.Equal(t, 8080, cfg.Orchestrator.Port)
	assert.Equal(t, int64(5000), cfg.Orchestrator.TimeAdditionMS)
	assert.Equal(t, int64(5000), cfg.Orchestrator.TimeSubtractionMS)
	assert.Equal(t, int64(10000), cfg.Orchestrator.TimeMultiplicationsMS)
	assert.Equal(t, int64(10000), cfg.Orchestrator.TimeDivisionsMS)
	assert.Equal(t, "orchestrator", cfg.Agent.OrchestratorHost)
	assert.Equal(t, "50051", cfg.Agent.OrchestratorPort)
	assert.Equal(t, int64(5), cfg.Agent.ComputingPower)
	assert.False(t, cfg.Logging.ToFile)
	assert.Equal(t, "json", cfg.Logging.Format)
	assert.Equal(t, 10, cfg.Logging.MaxSize)
	assert.Equal(t, 3, cfg.Logging.MaxFiles)
	assert.Equal(t, "postgres", cfg.DataBase.Username)
	assert.Equal(t, "postgres", cfg.DataBase.Password)
	assert.Equal(t, "postgres", cfg.DataBase.Host)
	assert.Equal(t, "5432", cfg.DataBase.Port)
	assert.Equal(t, "calculator", cfg.DataBase.DbName)
	assert.Equal(t, 5, cfg.DataBase.MaxAttemps)
	assert.Empty(t, cfg.SigningKey)
	assert.False(t, cfg.Debug)
	assert.Empty(t, cfg.Logging.Dir)
}

func TestConfig_Validate_Default(t *testing.T) {
	cfg := NewDefaultConfig()
	assert.NoError(t, cfg.Validate())
}

func TestConfig_String_YAML(t *testing.T) {
	cfg := NewDefaultConfig()
	yamlStr := cfg.String()
	assert.Contains(t, yamlStr, "orchestrator:")
	assert.Contains(t, yamlStr, "agent:")
}
