package config

import (
	"flag"
	"fmt"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

type ConfigPort struct {
	Port string `yaml:"port" env:"HELLO_PORT" env-default:"8080"`
}

func GetConfigPort() (ConfigPort, error) {
	var cfg ConfigPort

	var configPath string

	flag.StringVar(&configPath, "config", "", "path to config file")
	flag.Parse()

	if configPath != "" {
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			return ConfigPort{}, fmt.Errorf("config file not found: %v", configPath)
		}
		if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
			return ConfigPort{}, fmt.Errorf("failed to read config: %w", err)
		}
	} else {
		if err := cleanenv.ReadEnv(&cfg); err != nil {
			return ConfigPort{}, fmt.Errorf("failed to read env: %w", err)
		}
	}
	return cfg, nil
}
