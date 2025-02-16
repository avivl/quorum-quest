package store

// StoreConfig defines the interface for store configurations
type StoreConfig interface {
	Validate() error
	GetTableName() string
	GetTTL() int32
	GetEndpoints() []string
}
