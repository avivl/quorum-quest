package scylladb

// TODO Go over all ttl values and conversion
import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store"

	"github.com/avivl/quorum-quest/internal/lockservice"
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
func newStore(ctx context.Context, options lockservice.Config, logger *observability.SLogger) (store.Store, error) {
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
		return nil, err
	}
	sdb := &Store{session: session,
		tableName:     config.Table,
		keyspaceName:  config.Keyspace,
		fullTableName: config.Keyspace + "." + config.Table,
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
	sdb.ValidateLockQuery = fmt.Sprintf("SELECT client_id FROM %s WHERE service =? and  domain = ? and client_id =? ALLOW FILTERING", sdb.fullTableName)
	sdb.ReleaseLockQuery = fmt.Sprintf("DELETE FROM %s WHERE service =? and  domain =? and client_id =? USING TTL ?", sdb.fullTableName)

}
func (sdb *Store) TryAcquireLock(ctx context.Context, service, domain, client_id string, ttl int32) bool {
	_ttl := ttl
	if ttl == 0 {
		_ttl = sdb.ttl
	}
	err := sdb.session.Query(sdb.TryAcquireLockQuery,
		service, domain, client_id, _ttl).WithContext(ctx).Exec()
	if err != nil {
		sdb.l.Errorf("Error from TryAcquireLock insert %v", err)
		return false

	}
	var value string
	err = sdb.session.Query(sdb.ValidateLockQuery, service, domain, client_id).WithContext(ctx).Scan(&value)
	if err != nil {
		sdb.l.Errorf("Error from TryAcquireLock select %v", err)
		return false
	}
	return true

}

func (sdb *Store) ReleaseLock(ctx context.Context, service, domain, client_id string) {
	err := sdb.session.Query(sdb.ReleaseLockQuery,
		service, domain, client_id).WithContext(ctx).Exec()
	if err != nil {
		sdb.l.Errorf("Error ReleaseLock %v", err)
	}
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

// Close the store connection.
// Close the store connection.
// This method closes the underlying ScyllaDB session.
// It is important to call this method when the Store instance is no longer needed to free up resources.
//
// No parameters are required for this method.
//
// This method does not return any values.
func (sdb *Store) Close() {
	sdb.session.Close()

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

func (sdb *Store) validateTable() {

	err := sdb.session.Query(fmt.Sprintf(`CREATE Table IF NOT EXISTS %s.%s
	(service text,
domain  text,
client_id text,
PRIMARY KEY (service,domain))
WITH default_time_to_live = 15;
`, sdb.keyspaceName, sdb.tableName)).Exec()
	if err != nil {
		sdb.l.Fatal(err)
	}

}
