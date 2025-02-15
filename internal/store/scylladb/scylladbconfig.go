package scylladb

import "github.com/avivl/quorum-quest/internal/store"

type ScyllaDBConfig struct {
	store.BaseStoreConfig
	Host        string
	Port        int32
	Keyspace    string
	Consistency int32
}
