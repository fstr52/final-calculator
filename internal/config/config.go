package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type OrchestratorConfig struct {
	Port                  int   `yaml:"port" env:"ORCHESTRATOR_PORT"`
	TimeAdditionMS        int64 `yaml:"time_addition_ms" env:"TIME_ADDITION_MS"`
	TimeSubtractionMS     int64 `yaml:"time_subtraction_ms" env:"TIME_SUBTRACTION_MS"`
	TimeMultiplicationsMS int64 `yaml:"time_multiplications_ms" env:"TIME_MULTIPLICATIONS_MS"`
	TimeDivisionsMS       int64 `yaml:"time_divisions_ms" env:"TIME_DIVISIONS_MS"`
}

type AgentConfig struct {
	OrchestratorHost string `yaml:"orchestrator_host" env:"ORCHESTRATOR_HOST"`
	OrchestratorPort string `yaml:"orchestrator_port" env:"ORCHESTRATOR_PORT"`
	ComputingPower   int64  `yaml:"computing_power" env:"COMPUTING_POWER"` // Количество запускаемых горутин для каждого агента
}

type DataBaseConfig struct {
	Username   string `yaml:"username" env:"DB_USERNAME"`
	Password   string `yaml:"password" env:"DB_PASSWORD"`
	Host       string `yaml:"host" env:"DB_HOST"`
	Port       string `yaml:"port" env:"DB_PORT"`
	DbName     string `yaml:"db_name" env:"DB_NAME"`
	MaxAttemps int    `yaml:"max_attmeps" env:"DB_MAXATTEMPS"`
}

type LoggingConfig struct {
	ToFile   bool   `yaml:"to_file" env:"TO_FILE"`
	Dir      string `yaml:"logging_dir" env:"LOGGING_DIR"`       // Дирректория для логирования
	Format   string `yaml:"logging_format" env:"LOGGING_FORMAT"` // Формат логирования, поддерживаемые форматы: json, текстовый (по стандарту - текстовый)
	MaxSize  int    `yaml:"logging_file_max_size" env:"LOGGING_FILE_MAX_SIZE"`
	MaxFiles int    `yaml:"logging_max_filex" env:"LOGGING_MAX_FILES"`
}

type Config struct {
	SigningKey   string             `yaml:"signing_key" env:"SIGNING_KEY"`
	Debug        bool               `yaml:"debug" env:"DEBUG"`
	Orchestrator OrchestratorConfig `yaml:"orchestrator"`
	Agent        AgentConfig        `yaml:"agent"`
	DataBase     DataBaseConfig     `yaml:"data_base"`
	Logging      LoggingConfig      `yaml:"logging" env:"LOGGING"`
}

func (c *Config) String() string {
	out, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Sprintf("Failed to convert config to string: %v", err)
	}
	return string(out)
}

func NewDefaultConfig() *Config {
	cfg := &Config{}

	cfg.Orchestrator.Port = 8080
	cfg.Orchestrator.TimeAdditionMS = 5000
	cfg.Orchestrator.TimeSubtractionMS = 5000
	cfg.Orchestrator.TimeMultiplicationsMS = 10000
	cfg.Orchestrator.TimeDivisionsMS = 10000

	cfg.Agent.OrchestratorHost = "orchestrator"
	cfg.Agent.OrchestratorPort = "50051"
	cfg.Agent.ComputingPower = 5

	cfg.Logging.ToFile = false
	cfg.Logging.Format = "json"
	cfg.Logging.MaxSize = 10
	cfg.Logging.MaxFiles = 3

	cfg.DataBase.DbName = "calculator"
	cfg.DataBase.Host = "postgres"
	cfg.DataBase.MaxAttemps = 5
	cfg.DataBase.Password = "postgres"
	cfg.DataBase.Port = "5432"
	cfg.DataBase.Username = "postgres"

	return cfg
}

func (c *Config) Validate() error {
	if c.Orchestrator.Port <= 0 || c.Orchestrator.Port > 65535 {
		return fmt.Errorf("invalid orchestrator port: %d", c.Orchestrator.Port)
	}

	if c.Orchestrator.TimeAdditionMS <= 0 {
		return fmt.Errorf("invalid time additions: %d", c.Orchestrator.TimeAdditionMS)
	}

	if c.Orchestrator.TimeDivisionsMS <= 0 {
		return fmt.Errorf("invalid time divisions: %d", c.Orchestrator.TimeDivisionsMS)
	}

	if c.Orchestrator.TimeMultiplicationsMS <= 0 {
		return fmt.Errorf("invalid time multiplications: %d", c.Orchestrator.TimeMultiplicationsMS)
	}

	if c.Orchestrator.TimeSubtractionMS <= 0 {
		return fmt.Errorf("invalid time substractions: %d", c.Orchestrator.TimeSubtractionMS)
	}

	if c.Agent.ComputingPower <= 0 {
		return fmt.Errorf("invalid computing power: %d", c.Agent.ComputingPower)
	}

	if len(c.Agent.OrchestratorHost) == 0 {
		return fmt.Errorf("invalid orchestrator host: %s", c.Agent.OrchestratorHost)
	}

	if c.DataBase.DbName == "" {
		return fmt.Errorf("db name can't be empty")
	}

	if c.DataBase.Host == "" {
		return fmt.Errorf("db host can't be empty")
	}

	if c.DataBase.MaxAttemps <= 0 {
		return fmt.Errorf("invalid database max attemps: %d", c.DataBase.MaxAttemps)
	}

	if c.DataBase.Password == "" {
		return fmt.Errorf("db password can't be empty")
	}

	if c.DataBase.Port == "" {
		return fmt.Errorf("db port can't be empty")
	}

	if c.DataBase.Username == "" {
		return fmt.Errorf("db username can't be empty")
	}

	return nil
}
