package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	Default string `json:"default"` // gemini
	Gemini  struct {
		ApiKey string `json:"api_key"`
		Model  string `json:"model"`
	} `json:"gemini"`
}

func getConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".config", "boteco", "config.json"), nil
}

func (c *Config) SaveConfig() error {
	configFilePath, err := getConfigPath()
	if err != nil {
		return err
	}

	configPath := filepath.Dir(configFilePath)
	err = os.MkdirAll(configPath, 0755)
	if err != nil {
		return err
	}

	j, err := json.MarshalIndent(c, "", " ")
	if err != nil {
		return err
	}

	return os.WriteFile(configFilePath, j, 0644)
}

func GetConfig() (Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return Config{}, err
	}

	j, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}, err
	}

	c := Config{}
	err = json.Unmarshal(j, &c)
	return c, err
}
