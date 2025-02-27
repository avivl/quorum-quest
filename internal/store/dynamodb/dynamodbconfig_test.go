// internal/store/dynamodb/dynamodbconfig_test.go
package dynamodb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDynamoDBConfig(t *testing.T) {
	t.Run("GetTableName", func(t *testing.T) {
		cfg := &DynamoDBConfig{Table: "test-table"}
		assert.Equal(t, "test-table", cfg.GetTableName())
	})

	t.Run("GetTTL", func(t *testing.T) {
		cfg := &DynamoDBConfig{TTL: 30}
		assert.Equal(t, int32(30), cfg.GetTTL())
	})

	t.Run("GetEndpoints", func(t *testing.T) {
		endpoints := []string{"endpoint1", "endpoint2"}
		cfg := &DynamoDBConfig{Endpoints: endpoints}
		assert.Equal(t, endpoints, cfg.GetEndpoints())
	})

	t.Run("Validate_Success", func(t *testing.T) {
		cfg := &DynamoDBConfig{
			Region:    "us-west-2",
			Table:     "test-table",
			TTL:       30,
			Endpoints: []string{"endpoint1"},
		}
		assert.NoError(t, cfg.Validate())
	})

	t.Run("Validate_Missing_Region", func(t *testing.T) {
		cfg := &DynamoDBConfig{
			Table:     "test-table",
			TTL:       30,
			Endpoints: []string{"endpoint1"},
		}
		assert.Error(t, cfg.Validate())
	})

	t.Run("Validate_Missing_Table", func(t *testing.T) {
		cfg := &DynamoDBConfig{
			Region:    "us-west-2",
			TTL:       30,
			Endpoints: []string{"endpoint1"},
		}
		assert.Error(t, cfg.Validate())
	})

	t.Run("Validate_Invalid_TTL", func(t *testing.T) {
		cfg := &DynamoDBConfig{
			Region:    "us-west-2",
			Table:     "test-table",
			TTL:       0,
			Endpoints: []string{"endpoint1"},
		}
		assert.Error(t, cfg.Validate())
	})

	t.Run("Validate_No_Endpoints", func(t *testing.T) {
		cfg := &DynamoDBConfig{
			Region: "us-west-2",
			Table:  "test-table",
			TTL:    30,
		}
		assert.Error(t, cfg.Validate())
	})

	t.Run("Validate_Inconsistent_Credentials", func(t *testing.T) {
		cfg := &DynamoDBConfig{
			Region:      "us-west-2",
			Table:       "test-table",
			TTL:         30,
			Endpoints:   []string{"endpoint1"},
			AccessKeyID: "key",
			// Missing SecretAccessKey
		}
		assert.Error(t, cfg.Validate())
	})

	t.Run("NewDynamoDBConfig", func(t *testing.T) {
		cfg := NewDynamoDBConfig()
		assert.Equal(t, "us-west-2", cfg.Region)
		assert.Equal(t, "quorum-quest", cfg.Table)
		assert.Equal(t, int32(15), cfg.TTL)
		assert.Equal(t, []string{"dynamodb.us-west-2.amazonaws.com"}, cfg.Endpoints)
	})
}
