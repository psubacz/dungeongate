package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"google.golang.org/grpc"
)

// PoolBasedGRPCServer implements gRPC server with pool-based architecture
type PoolBasedGRPCServer struct {
	config   GRPCServerConfig
	handler  *SessionHandler
	logger   *slog.Logger
	
	server   *grpc.Server
	listener net.Listener
}

// NewPoolBasedGRPCServer creates a new pool-based gRPC server
func NewPoolBasedGRPCServer(config GRPCServerConfig, handler *SessionHandler, logger *slog.Logger) *PoolBasedGRPCServer {
	s := &PoolBasedGRPCServer{
		config:  config,
		handler: handler,
		logger:  logger,
	}
	
	s.setupServer()
	return s
}

// Name returns the server name
func (s *PoolBasedGRPCServer) Name() string {
	return "grpc"
}

// Start starts the pool-based gRPC server
func (s *PoolBasedGRPCServer) Start(ctx context.Context) error {
	address := fmt.Sprintf("%s:%d", s.config.Address, s.config.Port)
	
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", address, err)
	}
	s.listener = listener
	
	s.logger.Info("Pool-based gRPC server starting", "address", address)
	
	// Start server in goroutine
	go func() {
		if err := s.server.Serve(listener); err != nil {
			s.logger.Error("Pool-based gRPC server error", "error", err)
		}
	}()
	
	return nil
}

// Stop stops the pool-based gRPC server
func (s *PoolBasedGRPCServer) Stop(ctx context.Context) error {
	s.logger.Info("Stopping pool-based gRPC server")
	
	if s.server != nil {
		// Graceful stop with context timeout
		stopped := make(chan struct{})
		go func() {
			s.server.GracefulStop()
			close(stopped)
		}()
		
		select {
		case <-stopped:
			s.logger.Info("Pool-based gRPC server stopped gracefully")
		case <-ctx.Done():
			s.logger.Warn("Timeout during gRPC server shutdown, forcing stop")
			s.server.Stop()
		}
	}
	
	if s.listener != nil {
		s.listener.Close()
	}
	
	return nil
}

// setupServer configures the gRPC server for pool-based architecture
func (s *PoolBasedGRPCServer) setupServer() {
	// Create gRPC server with pool-aware interceptors
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(s.unaryInterceptor),
		grpc.StreamInterceptor(s.streamInterceptor),
	}
	
	s.server = grpc.NewServer(opts...)
	
	// TODO: Register pool-based gRPC services
	// For now, we'll register basic services that interact with pools
	s.registerServices()
}

// registerServices registers gRPC services for pool-based architecture
func (s *PoolBasedGRPCServer) registerServices() {
	// TODO: Register actual pool-based services
	// This might include:
	// - Session management service
	// - Pool monitoring service  
	// - Resource management service
	// - Health check service
	
	s.logger.Info("Pool-based gRPC services registered")
}

// unaryInterceptor adds pool awareness to unary gRPC calls
func (s *PoolBasedGRPCServer) unaryInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	// TODO: Add pool-based request tracking and resource management
	
	// For now, just log and pass through
	s.logger.Debug("Pool-based gRPC unary call", "method", info.FullMethod)
	
	return handler(ctx, req)
}

// streamInterceptor adds pool awareness to streaming gRPC calls
func (s *PoolBasedGRPCServer) streamInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	// TODO: Add pool-based stream tracking and resource management
	
	// For now, just log and pass through
	s.logger.Debug("Pool-based gRPC stream call", "method", info.FullMethod)
	
	return handler(srv, ss)
}