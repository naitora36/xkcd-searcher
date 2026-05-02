package config

import (
	"log"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	LogLevel             string        `yaml:"log_level" env:"LOG_LEVEL" env-default:"DEBUG"`
	Address              string        `yaml:"search_address" env:"ISEARCH_ADDRESS" env-default:"localhost:84"`
	DBAddress            string        `yaml:"db_address" env:"DB_ADDRESS" env-default:"localhost:82"`
	WordsAddress         string        `yaml:"words_address" env:"WORDS_ADDRESS" env-default:"localhost:81"`
	BrokerAddress        string        `yaml:"broker_address" env:"BROKER_ADDRESS" env-default:"nats://nats:4222"`
	MetricsServerAddress string        `yaml:"metrics_server_address" env:"METRICS_SERVER_ADDRESS" env-default:"localhost:8086"`
	TTL                  time.Duration `yaml:"index_timer" env:"INDEX_TTL" env-default:"24h"`
}

func MustLoad(configPath string) Config {
	var cfg Config
	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		log.Fatalf("cannot read config %q: %s", configPath, err)
	}
	return cfg
}
