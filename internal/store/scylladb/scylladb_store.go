// internal/store/scylladb/scylladb_store.go
package scylladb

// TODO Go over all ttl values and conversion
import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/avivl/quorum-quest/internal/lockservice"
	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store"
	"github.com/gocql/gocql"
)

var (
	// ErrMultipleEndpointsUnsupported is returned when more than one endpoint is provided.
	ErrMultipleEndpointsUnsupported = errors.New("ScyllaDB only supports one endpoint")
	ErrConfigOptionMissing          = errors.New("ScyllaDB requires a config option")
)

// StoreName the name of the store.
const StoreName string = "scylladb"

// init registers the ScyllaDB store with the lockservice package.
func init() {
	// Register the ScyllaDB store with the lockservice package using the StoreName and newStore function.
	lockservice.Register(StoreName, newStore)
}

// newStore creates a new ScyllaDB store instance based on the provided context, endpoints, and configuration options.
// It returns a store.Store interface and an error if any.
// If more than one endpoint is provided, it returns ErrMultipleEndpointsUnsupported.
// If the configuration options are missing or invalid, it returns ErrConfigOptionMissing.
func newStore(ctx context.Context, options lockservice.Config, logger *observability.SLogger) (store.LockStore, error) {
	cfg, ok := options.(*ScyllaDBConfig)
	if !ok && options != nil {
		return nil, &store.InvalidConfigurationError{Store: StoreName, Config: options}
	}
	return New(ctx, cfg, logger)
}

// Store implements the store.Store interface.
type Store struct {
	session             *gocql.Session
	tableName           string
	keyspaceName        string
	fullTableName       string
	ttl                 int32
	l                   *observability.SLogger
	TryAcquireLockQuery string
	ValidateLockQuery   string
	ReleaseLockQuery    string
	config              *ScyllaDBConfig
}

// GetConfig returns the current store configuration
func (s *Store) GetConfig() store.StoreConfig {
	return s.config
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

// New creates a new ScyllaDB client.
func New(ctx context.Context, config *ScyllaDBConfig, logger *observability.SLogger) (*Store, error) {
	if len(config.Endpoints) > 1 {
		return nil, ErrMultipleEndpointsUnsupported
	}
	if config == nil {
		return nil, ErrConfigOptionMissing
	}

	cluster := gocql.NewCluster(config.Host + ":" + strconv.Itoa(int(config.Port)))
	cluster.ProtoVersion = 4
	cluster.Consistency = parseConsistency(config.Consistency)

	session, err := cluster.CreateSession()
	if err != nil {
		logger.Errorf("Error creating session: %v", err)
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	sdb := &Store{
		session:       session,
		tableName:     config.Table,
		keyspaceName:  config.Keyspace,
		fullTableName: fmt.Sprintf(`"%s"."%s"`, config.Keyspace, config.Table), // Use double quotes here too
		ttl:           int32(math.Round((time.Duration(config.TTL).Seconds() * 1000000000))),
		l:             logger,
		config:        config,
	}

	sdb.initSession()

	return sdb, nil
}

func (sdb *Store) initSession() {
	sdb.validateKeyspace()
	sdb.validateTable()
	sdb.TryAcquireLockQuery = fmt.Sprintf("INSERT INTO %s (service, domain, client_id) VALUES (?, ?, ?) IF NOT EXISTS USING TTL ?", sdb.fullTableName)
	sdb.ValidateLockQuery = fmt.Sprintf("SELECT client_id FROM %s WHERE service =? and domain = ? and client_id =? ALLOW FILTERING", sdb.fullTableName)
	// Fix: Remove client_id from WHERE clause and remove USING TTL
	sdb.ReleaseLockQuery = fmt.Sprintf("DELETE FROM %s WHERE service =? and domain =?", sdb.fullTableName)
}

func (sdb *Store) validateKeyspace() {
	err := sdb.session.Query(fmt.Sprintf(`CREATE KEYSPACE IF NOT EXISTS %s
	WITH replication = {
		'class' : 'SimpleStrategy',
		'replication_factor' :3
	}`, sdb.keyspaceName)).Exec()
	if err != nil {
		sdb.l.Fatal(err)
	}

}

func (sdb *Store) validateTable() error {
	// Create the main table with a compound primary key
	defaultTTL := 15 // Store this as a const at package level
	err := sdb.session.Query(fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.%s (
        service text,
        domain text,
        client_id text,
        PRIMARY KEY ((service, domain))
    ) WITH default_time_to_live = %d`, sdb.keyspaceName, sdb.tableName, defaultTTL)).Exec()
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	// Create an index on client_id to support our queries
	err = sdb.session.Query(fmt.Sprintf(
		"CREATE INDEX IF NOT EXISTS ON %s.%s (client_id)",
		sdb.keyspaceName, sdb.tableName)).Exec()
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}

	return nil
}

func (sdb *Store) TryAcquireLock(ctx context.Context, service, domain, clientId string, ttl int32) bool {
	_ttl := ttl
	if ttl == 0 {
		_ttl = sdb.ttl
	}

	// First attempt the insertion
	err := sdb.session.Query(sdb.TryAcquireLockQuery,
		service, domain, clientId, _ttl).WithContext(ctx).Exec()
	if err != nil {
		sdb.l.Errorf("Error acquiring lock: %v", err)
		return false
	}

	// Verify the lock was acquired by reading it back
	var value string
	err = sdb.session.Query(sdb.ValidateLockQuery, service, domain, clientId).WithContext(ctx).Scan(&value)
	if err != nil {
		sdb.l.Errorf("Error validating lock: %v", err)
		return false
	}

	return true
}

func (sdb *Store) ReleaseLock(ctx context.Context, service, domain, clientId string) {
	// First validate that this client owns the lock
	var storedClientId string
	err := sdb.session.Query(sdb.ValidateLockQuery, service, domain, clientId).WithContext(ctx).Scan(&storedClientId)

	if err == nil && storedClientId == clientId {
		// This client owns the lock, so it can release it
		err = sdb.session.Query(sdb.ReleaseLockQuery, service, domain).WithContext(ctx).Exec()
		if err != nil {
			sdb.l.Errorf("Error ReleaseLock %v", err)
		}
	} else if err != nil && err != gocql.ErrNotFound {
		// Log errors other than "not found"
		sdb.l.Errorf("Error validating lock ownership before release: %v", err)
	}
	// If the lock doesn't exist or isn't owned by this client, do nothing
}

func (sdb *Store) KeepAlive(ctx context.Context, service, domain, client_id string, ttl int32) time.Duration {
	var value string
	_ttl := ttl
	if ttl == 0 {
		_ttl = sdb.ttl
	}
	err := sdb.session.Query(sdb.ValidateLockQuery, service, domain, client_id).WithContext(ctx).Scan(&value)
	if err != nil {
		sdb.l.Errorf("Error from KeepAlive select %v", err)
		return time.Duration(-1) * time.Second
	}
	err = sdb.session.Query(sdb.TryAcquireLockQuery,
		service, domain, client_id, _ttl).WithContext(ctx).Exec()
	if err != nil {
		sdb.l.Errorf("Error from KeepAlive insert %v", err)
		return time.Duration(-1) * time.Second
	}
	return time.Duration(sdb.ttl) * time.Second
}
func (sdb *Store) Close() {
	sdb.session.Close()

}
