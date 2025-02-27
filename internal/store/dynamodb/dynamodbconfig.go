// internal/store/dynamodb/dynamodbconfig.go
package dynamodb

import (
	"errors"
)

type DynamoDBConfig struct {
	Region          string   `yaml:"region"`
	Table           string   `yaml:"table"`
	TTL             int32    `yaml:"ttl"`
	Endpoints       []string `yaml:"endpoints"`
	Profile         string   `yaml:"profile,omitempty"`
	AccessKeyID     string   `yaml:"accessKeyId,omitempty"`
	SecretAccessKey string   `yaml:"secretAccessKey,omitempty"`
}

func (c *DynamoDBConfig) GetTableName() string {
	return c.Table
}

func (c *DynamoDBConfig) GetTTL() int32 {
	return c.TTL
}

func (c *DynamoDBConfig) GetEndpoints() []string {
	return c.Endpoints
}

func (c *DynamoDBConfig) Validate() error {
	if c.Region == "" {
		return errors.New("region is required")
	}
	if c.Table == "" {
		return errors.New("table is required")
	}
	if c.TTL <= 0 {
		return errors.New("invalid TTL")
	}
	if len(c.Endpoints) == 0 {
		return errors.New("at least one endpoint is required")
	}
	// Check if credentials are provided consistently
	if (c.AccessKeyID != "" && c.SecretAccessKey == "") ||
		(c.AccessKeyID == "" && c.SecretAccessKey != "") {
		return errors.New("both access key and secret key must be provided together")
	}
	return nil
}

// NewDynamoDBConfig creates a new DynamoDB configuration with default values
func NewDynamoDBConfig() *DynamoDBConfig {
	return &DynamoDBConfig{
		Region:    "us-west-2",
		Table:     "quorum-quest",
		TTL:       15,
		Endpoints: []string{"dynamodb.us-west-2.amazonaws.com"},
	}
}
