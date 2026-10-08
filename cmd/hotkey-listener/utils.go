package main

import (
	"encoding/json"
	"os"
)

type HotkeyConfig map[string]string

func loadConfig(path string) (HotkeyConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg HotkeyConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
