// internal/store/scylladb/scylladbconfig.go
package scylladb

import (
	"errors"
)

// ScyllaDBConfig implements the Config interface for ScyllaDB
type ScyllaDBConfig struct {
	Host        string   `yaml:"host"`
	Port        int32    `yaml:"port"`
	Keyspace    string   `yaml:"keyspace"`
	Table       string   `yaml:"table"`
	TTL         int32    `yaml:"ttl"`
	Consistency string   `yaml:"consistency"`
	Endpoints   []string `yaml:"endpoints"`
}

func (c *ScyllaDBConfig) GetTableName() string {
	return c.Table
}

func (c *ScyllaDBConfig) GetTTL() int32 {
	return c.TTL
}

func (c *ScyllaDBConfig) GetEndpoints() []string {
	return c.Endpoints
}

func (c *ScyllaDBConfig) Validate() error {
	if c.Host == "" {
		return errors.New("host is required")
	}
	if c.Port <= 0 {
		return errors.New("invalid port")
	}
	if c.Keyspace == "" {
		return errors.New("keyspace is required")
	}
	if c.Table == "" {
		return errors.New("table is required")
	}
	if c.TTL <= 0 {
		return errors.New("invalid TTL")
	}
	if c.Consistency == "" {
		return errors.New("consistency level is required")
	}
	if len(c.Endpoints) == 0 {
		return errors.New("at least one endpoint is required")
	}
	return nil
}

// NewScyllaDBConfig creates a new ScyllaDB configuration with default values
func NewScyllaDBConfig() *ScyllaDBConfig {
	return &ScyllaDBConfig{
		Host:        "127.0.0.1",
		Port:        9042,
		Keyspace:    "quorum-quest",
		Table:       "services",
		TTL:         15,
		Consistency: "CONSISTENCY_QUORUM",
		Endpoints:   []string{"localhost:9042"},
	}
}
