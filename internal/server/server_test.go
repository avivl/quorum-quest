package server

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	pb "github.com/avivl/quorum-quest/api/gen/go/v1"
	"github.com/avivl/quorum-quest/internal/config"
	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store"
	"github.com/avivl/quorum-quest/internal/store/scylladb"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
)

var srv *Server[*scylladb.ScyllaDBConfig]

// createStoreInitializer creates a wrapper function that adapts scylladb.New to the expected signature
func createStoreInitializer() func(context.Context, *config.GlobalConfig[*scylladb.ScyllaDBConfig], *observability.SLogger) (store.Store, error) {
	return func(ctx context.Context, cfg *config.GlobalConfig[*scylladb.ScyllaDBConfig], logger *observability.SLogger) (store.Store, error) {
		return scylladb.New(ctx, cfg.Store, logger)
	}
}

func TestMain(m *testing.M) {
	if err := setup("localhost:5050"); err != nil {
		fmt.Println("Failed to setup tests", err)
		os.Exit(1)
	}
	exitCode := m.Run()
	tearDown()
	os.Exit(exitCode)
}

func setup(addr string) error {
	ctx := context.TODO()
	logger, _, err := observability.NewTestLogger()
	if err != nil {
		return fmt.Errorf("failed to create test logger: %w", err)
	}

	// Create the global config
	cfg := &config.GlobalConfig[*scylladb.ScyllaDBConfig]{
		Store: &scylladb.ScyllaDBConfig{
			Host:        "localhost",
			Port:        9042,
			Keyspace:    "quorum-quest",
			Table:       "services",
			TTL:         15,
			Consistency: "CONSISTENCY_QUORUM",
			Endpoints:   []string{"localhost:9042"},
		},
		Observability: observability.Config{
			ServiceName:    "quorum-quest",
			ServiceVersion: "0.1.0",
			Environment:    "test",
			OTelEndpoint:   "localhost:4317",
		},
		Logger: observability.LoggerConfig{
			Level: observability.LogLevel(zapcore.InfoLevel.String()),
		},
		ServerAddress: addr,
	}

	metrics, err := observability.NewMetricsClient(cfg.Observability, logger)
	if err != nil {
		return fmt.Errorf("failed to create metrics client: %w", err)
	}

	storeInit := createStoreInitializer()
	s, err := NewServer(cfg, logger, metrics, storeInit)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	go func() {
		err = s.Start(ctx, storeInit)
	}()

	// Wait for server to start
	time.Sleep(time.Second * 2)
	srv = s
	return err
}

func tearDown() {
	if srv != nil {
		srv.Stop()
	}
}

func TestNilConfig(t *testing.T) {
	logger, _, _ := observability.NewTestLogger()
	metrics, _ := observability.NewMetricsClient(observability.Config{
		ServiceName: "test",
	}, logger)

	storeInit := createStoreInitializer()
	s, err := NewServer[*scylladb.ScyllaDBConfig](nil, logger, metrics, storeInit)
	assert.Nil(t, s)
	assert.Error(t, err)
}

func TestTryAcquireLockSuccess(t *testing.T) {
	resp, err := srv.TryAcquireLock(context.TODO(), &pb.TryAcquireLockRequest{
		Service:  "service1",
		Domain:   "domain1",
		ClientId: "client1",
		Ttl:      20,
	})
	assert.True(t, resp.IsLeader)
	assert.NoError(t, err)
}

func TestTryAcquireLockFail(t *testing.T) {
	resp, err := srv.TryAcquireLock(context.TODO(), &pb.TryAcquireLockRequest{
		Service:  "service1",
		Domain:   "domain1",
		ClientId: "client1",
	})
	assert.True(t, resp.IsLeader)
	assert.NoError(t, err)

	resp, err = srv.TryAcquireLock(context.TODO(), &pb.TryAcquireLockRequest{
		Service:  "service1",
		Domain:   "domain1",
		ClientId: "client2",
	})
	assert.False(t, resp.IsLeader)
	assert.NoError(t, err)
}

func TestReleaseLock(t *testing.T) {
	resp, err := srv.TryAcquireLock(context.TODO(), &pb.TryAcquireLockRequest{
		Service:  "service1",
		Domain:   "domain1",
		ClientId: "client1",
	})
	assert.True(t, resp.IsLeader)
	assert.NoError(t, err)

	_, err = srv.ReleaseLock(context.TODO(), &pb.ReleaseLockRequest{
		Service:  "service1",
		Domain:   "domain1",
		ClientId: "client1",
	})
	assert.NoError(t, err)
}

func TestKeepAlive(t *testing.T) {
	resp, err := srv.TryAcquireLock(context.TODO(), &pb.TryAcquireLockRequest{
		Service:  "service1",
		Domain:   "domain1",
		ClientId: "client1",
		Ttl:      20,
	})
	assert.True(t, resp.IsLeader)
	assert.NoError(t, err)

	keepAliveResp, err := srv.KeepAlive(context.TODO(), &pb.KeepAliveRequest{
		Service:  "service1",
		Domain:   "domain1",
		ClientId: "client1",
		Ttl:      20,
	})
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, keepAliveResp.LeaseLength.AsDuration(), time.Second*1)
}

func TestKeepAliveFail(t *testing.T) {
	resp, err := srv.TryAcquireLock(context.TODO(), &pb.TryAcquireLockRequest{
		Service:  "service1",
		Domain:   "domain1",
		ClientId: "client1",
		Ttl:      20,
	})
	assert.True(t, resp.IsLeader)
	assert.NoError(t, err)

	keepAliveResp, err := srv.KeepAlive(context.TODO(), &pb.KeepAliveRequest{
		Service:  "service1",
		Domain:   "domain1",
		ClientId: "client2",
		Ttl:      20,
	})
	assert.NoError(t, err)
	assert.LessOrEqual(t, keepAliveResp.LeaseLength.AsDuration(), time.Second*0)
}

func TestFailStart(t *testing.T) {
	assert.Error(t, setup("localhost:5050"))
}

package server

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/avivl/quorum-quest/internal/config"
	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store"
	"github.com/avivl/quorum-quest/internal/store/scylladb"
	pb "github.com/avivl/quorum-quest/api/gen/go/v1"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
)

var srv *Server[*scylladb.ScyllaDBConfig]

// createStoreInitializer creates a wrapper function that adapts scylladb.New to the expected signature
func createStoreInitializer() func(context.Context, *config.GlobalConfig[*scylladb.ScyllaDBConfig], *observability.SLogger) (store.Store, error) {
	return func(ctx context.Context, cfg *config.GlobalConfig[*scylladb.ScyllaDBConfig], logger *observability.SLogger) (store.Store, error) {
		return scylladb.New(ctx, cfg.Store, logger)
	}
}

func TestMain(m *testing.M) {
	if err := setup("localhost:5050"); err != nil {
		fmt.Println("Failed to setup tests", err)
		os.Exit(1)
	}
	exitCode := m.Run()
	tearDown()
	os.Exit(exitCode)
}

func setup(addr string) error {
	ctx := context.TODO()
	logger, _, err := observability.NewTestLogger()
	if err != nil {
		return fmt.Errorf("failed to create test logger: %w", err)
	}

	// Create the global config
	cfg := &config.GlobalConfig[*scylladb.ScyllaDBConfig]{
		Store: &scylladb.ScyllaDBConfig{
			Host:        "localhost",
			Port:        9042,
			Keyspace:    "quorum-quest",
			Table:       "services",
			TTL:         15,
			Consistency: "CONSISTENCY_QUORUM",
			Endpoints:   []string{"localhost:9042"},
		},
		Observability: observability.Config{
			ServiceName:    "quorum-quest",
			ServiceVersion: "0.1.0",
			Environment:    "test",
			OTelEndpoint:   "localhost:4317",
		},
		Logger: observability.LoggerConfig{
			Level: observability.LogLevel(zapcore.InfoLevel.String()),
		},
		ServerAddress: addr,
	}

	metrics, err := observability.NewMetricsClient(cfg.Observability, logger)
	if err != nil {
		return fmt.Errorf("failed to create metrics client: %w", err)
	}

	storeInit := createStoreInitializer()
	s, err := NewServer(cfg, logger, metrics, storeInit)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	go func() {
		err = s.Start(ctx, storeInit)
	}()

	// Wait for server to start
	time.Sleep(time.Second * 2)
	srv = s
	return err
}

func tearDown() {
	if srv != nil {
		srv.Stop()
	}
}

func TestNilConfig(t *testing.T) {
	logger, _, _ := observability.NewTestLogger()
	metrics, _ := observability.NewMetricsClient(observability.Config{
		ServiceName: "test",
	}, logger)
	
	storeInit := createStoreInitializer()
	s, err := NewServer[*scylladb.ScyllaDBConfig](nil, logger, metrics, storeInit)
	assert.Nil(t, s)
	assert.Error(t, err)
}

func TestTryAcquireLockSuccess(t *testing.T) {
	resp, err := srv.TryAcquireLock(context.TODO(), &pb.TryAcquireLockRequest{
		Service:  "service1",
		Domain:   "domain1",
		ClientId: "client1",
		Ttl:      20,
	})
	assert.True(t, resp.IsLeader)
	assert.NoError(t, err)
}

func TestTryAcquireLockFail(t *testing.T) {
	resp, err := srv.TryAcquireLock(context.TODO(), &pb.TryAcquireLockRequest{
		Service:  "service1",
		Domain:   "domain1",
		ClientId: "client1",
	})
	assert.True(t, resp.IsLeader)
	assert.NoError(t, err)

	resp, err = srv.TryAcquireLock(context.TODO(), &pb.TryAcquireLockRequest{
		Service:  "service1",
		Domain:   "domain1",
		ClientId: "client2",
	})
	assert.False(t, resp.IsLeader)
	assert.NoError(t, err)
}

func TestReleaseLock(t *testing.T) {
	resp, err := srv.TryAcquireLock(context.TODO(), &pb.TryAcquireLockRequest{
		Service:  "service1",
		Domain:   "domain1",
		ClientId: "client1",
	})
	assert.True(t, resp.IsLeader)
	assert.NoError(t, err)

	_, err = srv.ReleaseLock(context.TODO(), &pb.ReleaseLockRequest{
		Service:  "service1",
		Domain:   "domain1",
		ClientId: "client1",
	})
	assert.NoError(t, err)
}

func TestKeepAlive(t *testing.T) {
	resp, err := srv.TryAcquireLock(context.TODO(), &pb.TryAcquireLockRequest{
		Service:  "service1",
		Domain:   "domain1",
		ClientId: "client1",
		Ttl:      20,
	})
	assert.True(t, resp.IsLeader)
	assert.NoError(t, err)

	keepAliveResp, err := srv.KeepAlive(context.TODO(), &pb.KeepAliveRequest{
		Service:  "service1",
		Domain:   "domain1",
		ClientId: "client1",
		Ttl:      20,
	})
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, keepAliveResp.LeaseLength.AsDuration(), time.Second*1)
}

func TestKeepAliveFail(t *testing.T) {
	resp, err := srv.TryAcquireLock(context.TODO(), &pb.TryAcquireLockRequest{
		Service:  "service1",
		Domain:   "domain1",
		ClientId: "client1",
		Ttl:      20,
	})
	assert.True(t, resp.IsLeader)
	assert.NoError(t, err)

	keepAliveResp, err := srv.KeepAlive(context.TODO(), &pb.KeepAliveRequest{
		Service:  "service1",
		Domain:   "domain1",
		ClientId: "client2",
		Ttl:      20,
	})
	assert.NoError(t, err)
	assert.LessOrEqual(t, keepAliveResp.LeaseLength.AsDuration(), time.Second*0)
}

func TestFailStart(t *testing.T) {
	assert.Error(t, setup("localhost:5050"))
}

func TestStop(t *testing.T) {
	assert.NoError(t, srv.Stop())
	err := setup("localhost:5050")
	assert.NoError(t, err)
}

func TestFailInitStore(t *testing.T) {
	oldPort := srv.config.Store.Port
	srv.config.Store.Port = 9999
	storeInit := createStoreInitializer()
	assert.Error(t, srv.initStore(context.TODO(), storeInit))
	srv.config.Store.Port = oldPort
}
