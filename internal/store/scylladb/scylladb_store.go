// internal/store/scylladb/scylladb_store.go
package scylladb

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/avivl/quorum-quest/internal/lockservice"
	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store"
	"github.com/gocql/gocql"
)

// Error definitions
var (
	ErrMultipleEndpointsUnsupported = errors.New("ScyllaDB only supports one endpoint")
	ErrConfigOptionMissing          = errors.New("ScyllaDB requires a config option")
)

// StoreName is the registered name of the ScyllaDB store
const StoreName = "scylladb"

// Register the ScyllaDB store with the lockservice package
func init() {
	lockservice.Register(StoreName, newStore)
}

// Store implements the store.Store interface for ScyllaDB
type Store struct {
	session       *gocql.Session
	tableName     string
	keyspaceName  string
	fullTableName string
	ttl           int32
	logger        *observability.SLogger
	config        *ScyllaDBConfig

	// Prepared query statements
	tryAcquireLockQuery string
	validateLockQuery   string
	releaseLockQuery    string
}

// newStore creates a new ScyllaDB store instance from configuration
func newStore(ctx context.Context, options lockservice.Config, logger *observability.SLogger) (store.LockStore, error) {
	cfg, ok := options.(*ScyllaDBConfig)
	if !ok && options != nil {
		return nil, &store.InvalidConfigurationError{Store: StoreName, Config: options}
	}
	return New(ctx, cfg, logger)
}

// GetConfig returns the current store configuration
func (s *Store) GetConfig() store.StoreConfig {
	return s.config
}

// New creates a new ScyllaDB client
func New(ctx context.Context, config *ScyllaDBConfig, logger *observability.SLogger) (*Store, error) {
	if config == nil {
		return nil, ErrConfigOptionMissing
	}

	if len(config.Endpoints) > 1 {
		return nil, ErrMultipleEndpointsUnsupported
	}

	// Create cluster configuration
	session, err := createSession(config)
	if err != nil {
		logger.Errorf("Error creating session: %v", err)
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	store := &Store{
		session:       session,
		tableName:     config.Table,
		keyspaceName:  config.Keyspace,
		fullTableName: fmt.Sprintf(`"%s"."%s"`, config.Keyspace, config.Table),
		ttl:           config.TTL,
		logger:        logger,
		config:        config,
	}

	// Initialize the session and prepare queries
	store.initSession()

	return store, nil
}

// createSession creates a new ScyllaDB session
func createSession(config *ScyllaDBConfig) (*gocql.Session, error) {
	cluster := gocql.NewCluster(config.Host + ":" + strconv.Itoa(int(config.Port)))
	cluster.ProtoVersion = 4
	cluster.Consistency = parseConsistency(config.Consistency)

	return cluster.CreateSession()
}

// parseConsistency converts string consistency to gocql.Consistency
func parseConsistency(c string) gocql.Consistency {
	switch c {
	case "CONSISTENCY_QUORUM":
		return gocql.Quorum
	case "CONSISTENCY_ONE":
		return gocql.One
	case "CONSISTENCY_ALL":
		return gocql.All
	default:
		return gocql.Quorum
	}
}

// initSession initializes the session by creating keyspace and table if needed
// and preparing query statements
func (s *Store) initSession() {
	s.ensureKeyspaceExists()
	s.ensureTableExists()
	s.prepareQueries()
}

// ensureKeyspaceExists creates the keyspace if it doesn't exist
func (s *Store) ensureKeyspaceExists() {
	query := fmt.Sprintf("CREATE KEYSPACE IF NOT EXISTS %s WITH replication = {'class':'SimpleStrategy','replication_factor':3};",
		s.keyspaceName)

	err := s.session.Query(query).Exec()
	if err != nil {
		s.logger.Errorf("Error creating keyspace: %v", err)
		// Continue anyway as the keyspace might already exist
	}
}

// ensureTableExists creates the table and index if they don't exist
func (s *Store) ensureTableExists() {
	// Create table with proper schema
	tableQuery := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s.%s (service text, domain text, client_id text, PRIMARY KEY (service, domain)) WITH default_time_to_live = %d;",
		s.keyspaceName, s.tableName, s.ttl)

	if err := s.session.Query(tableQuery).Exec(); err != nil {
		s.logger.Errorf("Failed to create table: %v", err)
		// Continue anyway as the table might already exist
	}

	// Create index on client_id for faster lookups
	indexQuery := fmt.Sprintf("CREATE INDEX IF NOT EXISTS ON %s.%s (client_id);",
		s.keyspaceName, s.tableName)

	if err := s.session.Query(indexQuery).Exec(); err != nil {
		s.logger.Errorf("Failed to create index: %v", err)
		// Continue anyway as the index might already exist
	}
}

// prepareQueries prepares the CQL queries for lock operations
func (s *Store) prepareQueries() {
	s.tryAcquireLockQuery = fmt.Sprintf("INSERT INTO %s (service, domain, client_id) VALUES (?, ?, ?) IF NOT EXISTS USING TTL ?", s.fullTableName)
	s.validateLockQuery = fmt.Sprintf("SELECT client_id FROM %s WHERE service = ? AND domain = ? AND client_id = ? ALLOW FILTERING", s.fullTableName)
	s.releaseLockQuery = fmt.Sprintf("DELETE FROM %s WHERE service = ? AND domain = ?", s.fullTableName)
}

// resolveTTL returns the provided TTL or falls back to the default
func (s *Store) resolveTTL(ttl int32) int32 {
	if ttl > 0 {
		return ttl
	}
	return s.ttl
}

// TryAcquireLock attempts to acquire a lock for the given service and domain
func (s *Store) TryAcquireLock(ctx context.Context, service, domain, clientId string, ttl int32) bool {
	lockTTL := s.resolveTTL(ttl)

	// Attempt to insert the lock record
	err := s.session.Query(s.tryAcquireLockQuery, service, domain, clientId, lockTTL).
		WithContext(ctx).
		Exec()

	if err != nil {
		s.logger.Errorf("Error acquiring lock: %v", err)
		return false
	}

	// Verify the lock was acquired by reading it back
	return s.verifyLockOwnership(ctx, service, domain, clientId)
}

// verifyLockOwnership checks if the client owns the specified lock
func (s *Store) verifyLockOwnership(ctx context.Context, service, domain, clientId string) bool {
	var storedClientId string
	err := s.session.Query(s.validateLockQuery, service, domain, clientId).
		WithContext(ctx).
		Scan(&storedClientId)

	if err != nil {
		s.logger.Errorf("Error validating lock: %v", err)
		return false
	}

	return storedClientId == clientId
}

// ReleaseLock releases a lock if it's owned by the specified client
func (s *Store) ReleaseLock(ctx context.Context, service, domain, clientId string) {
	// First validate that the client owns the lock
	if !s.verifyLockOwnership(ctx, service, domain, clientId) {
		// If verification failed due to an error, it will have been logged already
		return
	}

	// Client owns the lock, so delete it
	err := s.session.Query(s.releaseLockQuery, service, domain).
		WithContext(ctx).
		Exec()

	if err != nil {
		s.logger.Errorf("Error releasing lock: %v", err)
	}
}

// KeepAlive refreshes a lock's TTL if it's owned by the specified client
func (s *Store) KeepAlive(ctx context.Context, service, domain, clientId string, ttl int32) time.Duration {
	lockTTL := s.resolveTTL(ttl)

	s.logger.Infof("Attempting KeepAlive for service=%s, domain=%s, client_id=%s",
		service, domain, clientId)

	// Try to reacquire the lock which will reset its TTL
	err := s.session.Query(s.tryAcquireLockQuery, service, domain, clientId, lockTTL).
		WithContext(ctx).
		Exec()

	if err != nil {
		s.logger.Errorf("Error from KeepAlive operation: %v", err)
		return time.Duration(-1) * time.Second
	}

	s.logger.Infof("Lock refreshed with TTL=%d", lockTTL)
	return time.Duration(lockTTL) * time.Second
}

// Close closes the ScyllaDB session
func (s *Store) Close() {
	if s.session != nil {
		s.session.Close()
	}
}
