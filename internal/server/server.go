package server

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	pb "github.com/avivl/quorum-quest/api/gen/go/v1"
	"github.com/avivl/quorum-quest/internal/config"
	"github.com/avivl/quorum-quest/internal/observability"
	"github.com/avivl/quorum-quest/internal/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
)

type Server[T store.StoreConfig] struct {
	pb.UnimplementedLeaderElectionServiceServer
	server   *grpc.Server
	listener net.Listener
	logger   *observability.SLogger
	config   *config.GlobalConfig[T]
	store    store.Store
	metrics  *observability.OTelMetrics
}

func NewServer[T store.StoreConfig](
	config *config.GlobalConfig[T],
	logger *observability.SLogger,
	metrics *observability.OTelMetrics,
	storeInitializer func(context.Context, *config.GlobalConfig[T], *observability.SLogger) (store.Store, error),
) (*Server[T], error) {
	if config == nil {
		return nil, errors.New("config is nil")
	}

	return &Server[T]{
		logger:  logger,
		config:  config,
		metrics: metrics,
	}, nil
}

func (s *Server[T]) initStore(ctx context.Context, storeInitializer func(context.Context, *config.GlobalConfig[T], *observability.SLogger) (store.Store, error)) error {
	var err error
	s.store, err = storeInitializer(ctx, s.config, s.logger)
	if err != nil {
		s.logger.ErrorCtx(ctx, err)
		return err
	}
	return nil
}

func (s *Server[T]) Start(ctx context.Context, storeInitializer func(context.Context, *config.GlobalConfig[T], *observability.SLogger) (store.Store, error)) error {
	var err error
	s.listener, err = net.Listen("tcp", s.config.ServerAddress)
	if err != nil {
		s.logger.ErrorCtx(ctx, err)
		return err
	}

	if err := s.initStore(ctx, storeInitializer); err != nil {
		s.logger.ErrorCtx(ctx, err)
		return err
	}

	return s.serve(ctx)
}

func (s *Server[T]) serve(ctx context.Context) error {
	if s.listener == nil {
		return errors.New("listener is required")
	}

	keepaliveParams := keepalive.ServerParameters{
		MaxConnectionAge:      30 * time.Second,
		MaxConnectionAgeGrace: 10 * time.Second,
	}

	ui := grpc.ChainUnaryInterceptor(
		s.unaryServerInterceptor(),
	)

	opts := []grpc.ServerOption{grpc.KeepaliveParams(keepaliveParams), ui}
	s.server = grpc.NewServer(opts...)
	pb.RegisterLeaderElectionServiceServer(s.server, s)

	s.logger.InfoCtx(ctx, "server listening at "+s.listener.Addr().String())

	if err := s.server.Serve(s.listener); err != nil && err.Error() != "closed" {
		return err
	}
	return nil
}

func (s *Server[T]) Stop() error {
	s.logger.Info("stopping server")
	if s.server != nil {
		s.server.GracefulStop()
		if s.store != nil {
			s.store.Close()
		}
		s.server = nil
		s.store = nil
	}
	return nil
}

func (s *Server[T]) TryAcquireLock(ctx context.Context, in *pb.TryAcquireLockRequest) (*pb.TryAcquireLockResponse, error) {
	start := time.Now()
	isLeader := s.store.TryAcquireLock(ctx, in.GetService(), in.GetDomain(), in.GetClientId(), in.GetTtl())

	// Record metrics
	if err := s.metrics.RecordLatency(ctx, time.Since(start),
		"operation", "TryAcquireLock",
		"service", in.GetService(),
		"domain", in.GetDomain(),
	); err != nil {
		s.logger.ErrorCtx(ctx, err)
	}

	return &pb.TryAcquireLockResponse{IsLeader: isLeader}, nil
}

func (s *Server[T]) ReleaseLock(ctx context.Context, in *pb.ReleaseLockRequest) (*pb.ReleaseLockResponse, error) {
	start := time.Now()
	s.store.ReleaseLock(ctx, in.GetService(), in.GetDomain(), in.GetClientId())

	// Record metrics
	if err := s.metrics.RecordLatency(ctx, time.Since(start),
		"operation", "ReleaseLock",
		"service", in.GetService(),
		"domain", in.GetDomain(),
	); err != nil {
		s.logger.ErrorCtx(ctx, err)
	}

	return &pb.ReleaseLockResponse{}, nil
}

func (s *Server[T]) KeepAlive(ctx context.Context, in *pb.KeepAliveRequest) (*pb.KeepAliveResponse, error) {
	start := time.Now()
	duration := s.store.KeepAlive(ctx, in.GetService(), in.GetDomain(), in.GetClientId(), in.GetTtl())

	// Record metrics
	if err := s.metrics.RecordLatency(ctx, time.Since(start),
		"operation", "KeepAlive",
		"service", in.GetService(),
		"domain", in.GetDomain(),
	); err != nil {
		s.logger.ErrorCtx(ctx, err)
	}

	return &pb.KeepAliveResponse{LeaseLength: durationpb.New(duration)}, nil
}

func (s *Server[T]) unaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		methodName := strings.TrimPrefix(info.FullMethod, "/")

		// Call the handler
		resp, err := handler(ctx, req)

		// Record metrics
		s.metrics.Increment(ctx, "grpc.requests.total", 1,
			"method", methodName,
			"status", status.Code(err).String(),
		)

		if err := s.metrics.RecordLatency(ctx, time.Since(start),
			"method", methodName,
			"status", status.Code(err).String(),
		); err != nil {
			s.logger.ErrorCtx(ctx, err)
		}

		return resp, err
	}
}
