package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Storage  StorageConfig  `yaml:"storage"`
	Database DatabaseConfig `yaml:"database"`
}

type ServerConfig struct {
	Addr string `yaml:"addr"`
}

type StorageConfig struct {
	RepositoryRoot string `yaml:"repository_root"`
}

type DatabaseConfig struct {
	Driver string `yaml:"driver"`
	DSN    string `yaml:"dsn"`
}

func Load(filename string) (Config, error) {
	contents, err := os.ReadFile(filename)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(contents, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Server.Addr == "" || cfg.Storage.RepositoryRoot == "" || cfg.Database.Driver == "" || cfg.Database.DSN == "" {
		return Config{}, fmt.Errorf("config requires server.addr, storage.repository_root, database.driver, and database.dsn")
	}
	if cfg.Database.Driver != "sqlite" && cfg.Database.Driver != "pgsql" {
		return Config{}, fmt.Errorf("database.driver must be sqlite or pgsql")
	}

	baseDir, err := filepath.Abs(filepath.Dir(filename))
	if err != nil {
		return Config{}, fmt.Errorf("resolve config directory: %w", err)
	}
	cfg.Storage.RepositoryRoot = resolvePath(baseDir, cfg.Storage.RepositoryRoot)
	if cfg.Database.Driver == "sqlite" {
		cfg.Database.DSN = resolvePath(baseDir, cfg.Database.DSN)
	}
	return cfg, nil
}

func resolvePath(baseDir, value string) string {
	if filepath.IsAbs(value) {
		return value
	}
	return filepath.Join(baseDir, value)
}
