package config

import (
	"github.com/caarlos0/env/v10"
)

// Загрузка конфигурационных переменных из окружения
func LoadFromEnv(config *Config) error {
	return env.Parse(config)
}
