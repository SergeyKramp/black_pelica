// Package config handles loading and accessing service configuration from a YAML file.
// It provides typed access to all configuration fields.
package config

import (
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Database  DatabaseConfig  `yaml:"database"`
	Kafka     KafkaConfig     `yaml:"kafka"`
	Selligent SelligentConfig `yaml:"selligent"`
	Scheduler SchedulerConfig `yaml:"scheduler"`
}

// DatabaseConfig holds PostgreSQL connection and connection pool settings.
type DatabaseConfig struct {
	URL  string             `yaml:"url"`
	Pool DatabasePoolConfig `yaml:"pool"`
}

// DatabasePoolConfig controls the size of the pgx connection pool.
// MaxConns limits total open connections; MinConns keeps a warm baseline
// to avoid cold-start latency after idle periods.
type DatabasePoolConfig struct {
	MaxConns int32 `yaml:"max_conns"`
	MinConns int32 `yaml:"min_conns"`
}

type KafkaConfig struct {
	Brokers  []string            `yaml:"brokers"`
	Vouchers KafkaVouchersConfig `yaml:"vouchers"`
}

type KafkaVouchersConfig struct {
	Topic         string `yaml:"topic"`
	ConsumerGroup string `yaml:"consumer_group"`
}

// SchedulerConfig controls how the reminder scheduler polls for due reminders.
// WorkerCount
// SELECT FOR UPDATE SKIP LOCKED, so increasing it raises throughput at
// the cost of more database connections.
type SchedulerConfig struct {
	IntervalSeconds int `yaml:"interval_seconds"` // Interval in seconds between scheduler runs
	WorkerCount     int `yaml:"worker_count"`     // Number of concurrent workers
	BatchSize       int `yaml:"batch_size"`       // Number of rows to fetch per worker
}

type SelligentConfig struct {
	BaseURL string `yaml:"base_url"`
	APIKey  string `yaml:"api_key"`
}

// Load expects a path to a file on the file system.
// It opens it and returns the parsed config.
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config file: %w", err)
	}
	defer f.Close()
	return Parse(f)
}

// Parse gets the YAML config from an [io.Reader] and returns the parsed config.
// Useful when config is retrieved from an external service such as AWS Secrets Manager.
func Parse(r io.Reader) (*Config, error) {
	var cfg Config
	if err := yaml.NewDecoder(r).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}
