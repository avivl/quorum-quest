package server

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	ballotconfig "github.com/avivl/configs/gen/go/ballot/v1"
	pb "github.com/avivl/protobufs/rokt/ballot/api/v1"
	"github.com/avivl/quorum-quest/internal/store"
	"github.com/github.com/avivl/quorum-quest/internal/observability"
	scylladb "github.com/github.com/avivl/quorum-quest/internal/scylladb"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	grpctrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/google.golang.org/grpc"
)

type Server struct {
	pb.UnimplementedLeaderElectionServiceServer
	server   *grpc.Server
	listener net.Listener
	l        *observability.SLogger
	config   *ballotconfig.BallotConfig
	store    store.Store
	dgStats  *observability.DatadogStatsD
}

func NewServer(config *ballotconfig.BallotConfig, logger *observability.SLogger, dgStats *observability.DatadogStatsD) (*Server, error) {
	if config == nil {
		return nil, errors.New("config is nil")
	}

	return &Server{l: logger, config: config, dgStats: dgStats}, nil
}

func (s *Server) initStore(ctx context.Context) error {
	var err error
	s.store, err = scylladb.New(ctx, s.config, s.l)
	return err
}

func (s *Server) Start(ctx context.Context, address string) error {
	s.server = nil
	var err error
	s.listener, err = net.Listen("tcp", address)
	if err != nil {
		s.l.Error("failed to listen: %v", zap.Error(err))
		return err
	}
	if err := s.initStore(ctx); err != nil {
		s.l.Error("failed to initialize store: %v", zap.Error(err))
		return err
	}
	return s.serve(ctx)
}

func (s *Server) serve(_ context.Context) error {
	if s.listener == nil {
		return errors.New("listener is required")
	}

	var keepaliveParams = keepalive.ServerParameters{
		MaxConnectionAge:      30 * time.Second, // If any connection is alive for more than 30 seconds, send a GOAWAY
		MaxConnectionAgeGrace: 10 * time.Second, // Allow 10 seconds for pending RPCs to complete before forcibly closing connections
	}
	ui := grpc.ChainUnaryInterceptor(
		grpctrace.UnaryServerInterceptor(grpctrace.WithServiceName(s.config.DatadogConfig.Service)),
		GRPCUnaryRequestMetric(s.dgStats),
	)
	opts := []grpc.ServerOption{grpc.KeepaliveParams(keepaliveParams), ui}
	s.server = grpc.NewServer(opts...)
	pb.RegisterLeaderElectionServiceServer(s.server, s)
	s.l.Info("server listening: %v", s.listener.Addr())
	if err := s.server.Serve(s.listener); err != nil && err.Error() != "closed" {
		return err
	}
	return nil
}

func (s *Server) Stop() error {
	s.l.Info("stopping server")
	/*if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			s.l.Error("error closing listener: %v", zap.Error(err))
			return err
		}
		s.listener.

	}*/
	if s.server != nil {
		s.server.GracefulStop()
		s.store.Close()
		s.server = nil
		s.store = nil

	}
	return nil
}

func (s *Server) TryAcquireLock(ctx context.Context, in *pb.TryAcquireLockRequest) (*pb.TryAcquireLockResponse, error) {
	isLeader := s.store.TryAcquireLock(ctx, in.GetService(), in.GetDomain(), in.GetClientId(), in.GetTtl())
	return &pb.TryAcquireLockResponse{IsLeader: isLeader}, nil
}

func (s *Server) ReleaseLock(ctx context.Context, in *pb.ReleaseLockRequest) (*pb.ReleaseLockResponse, error) {
	s.store.ReleaseLock(ctx, in.GetService(), in.GetDomain(), in.GetClientId())
	return &pb.ReleaseLockResponse{}, nil
}

func (s *Server) KeepAlive(ctx context.Context, in *pb.KeepAliveRequest) (*pb.KeepAliveResponse, error) {
	duration := s.store.KeepAlive(ctx, in.GetService(), in.GetDomain(), in.GetClientId(), in.GetTtl())
	return &pb.KeepAliveResponse{LeaseLength: durationpb.New(duration)}, nil
}

func GRPCUnaryRequestMetric(statsd *observability.DatadogStatsD) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
	) (interface{}, error) {
		fullMethod := info.FullMethod

		// Trim the method string because the first part becomes our metric name
		methodName := strings.TrimPrefix(fullMethod, "/rokt.ballot")
		methodTag := "method:" + methodName

		// Call the gRPC handler to process the request and get the response.
		resp, err := handler(ctx, req)

		errCodeTag := "status:" + status.Code(err).String()
		statsd.Incr("rokt.ballot.api.request", methodTag, errCodeTag)

		return resp, err
	}
}
