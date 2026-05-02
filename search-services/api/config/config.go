package config

import (
	"fmt"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	HTTPServer   HTTPServerConfig `yaml:"http_server"`
	LogLevel     string           `yaml:"log_level" env:"LOG_LEVEL" env-default:"DEBUG"`
	WordsAddress string           `yaml:"words_address" env:"WORDS_ADDRESS" env-default:"localhost:8080"`
}
type HTTPServerConfig struct {
	Address        string        `yaml:"address" env:"HTTP_SERVER_ADDRESS" env-default:"localhost:8888"`
	ReadTimeout    time.Duration `yaml:"timeout" env:"HTTP_SERVER_TIMEOUT" env-default:"30s"`
	ShutdownPeriod time.Duration `yaml:"shutdown_period" env:"HTTP_SHUTDOWN_PERIOD" env-default:"30s"`
}

func LoadConfig(configPath string) (Config, error) {
	var cfg Config

	if configPath != "" {
		if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
			return Config{}, fmt.Errorf("failed to read config: %w", err)
		}
	} else {
		if err := cleanenv.ReadEnv(&cfg); err != nil {
			return Config{}, fmt.Errorf("failed to read env: %w", err)
		}
	}

	return cfg, nil
}
