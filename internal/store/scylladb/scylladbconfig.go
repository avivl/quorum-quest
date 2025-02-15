package scylladb

import (
	"errors"
	"fmt"

	"github.com/spf13/viper"
)

// ScyllaDBConfig implements the Config interface for ScyllaDB
type ScyllaDBConfig struct {
	Host        string   `json:"host"`
	Port        int32    `json:"port"`
	Keyspace    string   `json:"keyspace"`
	Table       string   `json:"table"`
	TTL         int32    `json:"ttl"`
	Consistency string   `json:"consistency"`
	Endpoints   []string `json:"endpoints"`
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

func ScyllaConfigLoader(v *viper.Viper) (*ScyllaDBConfig, error) {
	// Set ScyllaDB defaults
	v.SetDefault("scylla.host", "127.0.0.1")
	v.SetDefault("scylla.port", 9042)
	v.SetDefault("scylla.keyspace", "ballot")
	v.SetDefault("scylla.table", "services")
	v.SetDefault("scylla.ttl", 15)
	v.SetDefault("scylla.consistency", "CONSISTENCY_QUORUM")
	v.SetDefault("scylla.endpoints", []string{"localhost:9042"})

	config := &ScyllaDBConfig{}
	if err := v.UnmarshalKey("scylla", config); err != nil {
		return nil, fmt.Errorf("unable to decode ScyllaDB config: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid ScyllaDB configuration: %w", err)
	}

	return config, nil
}

//example
// Load ScyllaDB configuration
// scyllaConfig, err := LoadConfig("/etc/myapp", "scylla", ScyllaConfigLoader)
// if err != nil {
// 	log.Fatalf("Failed to load ScyllaDB configuration: %v", err)
// }
