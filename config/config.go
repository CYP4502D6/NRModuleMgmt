package config

import (
	"fmt"
	"os"
	"time"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Serial  SerialConfig  `yaml:"serial"`
	SMS     SMSConfig     `yaml:"sms"`
	Discord DiscordConfig `yaml:"discord"`
}

type SerialConfig struct {
	IsLocal   bool   `yaml:"is_local"`
	Port      string `yaml:"port"`
	BaudRate  int    `yaml:"baud_rate"`
	RemoteAPI string `yaml:"remote_api"`
}

type SMSConfig struct {
	DBPath        string        `yaml:"db_path"`
	CheckInterval time.Duration `yaml:"check_interval"`
}

type DiscordConfig struct {
	BotToken  string `yaml:"bot_token"`
	ChannelID string `yaml:"channel_id"`
}

func Load(filename string) (*Config, error) {

	data, err := os.ReadFile(filename)

	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", filename, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config file %s: %w", filename, err)
	}

	return &cfg, nil
}
