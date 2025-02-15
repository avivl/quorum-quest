package store

import "time"

type StoreConfig interface {
	// Common configuration methods
	GetTableName() string
	GetTTL() int32
	GetEndpoints() []string

	// Database specific methods
	Validate() error
	ToJSON() ([]byte, error)
	FromJSON(data []byte) error
}

type BaseStoreConfig struct {
	TableName         string
	TTL               int32
	RetryInterval     time.Duration
	KeepAliveInterval time.Duration
	MaxRetries        int32
}

// Common implementation of the interface methods
func (b *BaseStoreConfig) GetTTL() int32 {
	return b.TTL
}

func (b *BaseStoreConfig) GetKeepAliveInterval() time.Duration {
	return b.KeepAliveInterval
}

func (b *BaseStoreConfig) GetRetryInterval() time.Duration {
	return b.RetryInterval
}

func (b *BaseStoreConfig) GetMaxRetries() int32 {
	return b.MaxRetries
}

func (b *BaseStoreConfig) GetTableName() string {
	return b.TableName
}
