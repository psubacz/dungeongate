package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// GRPCServer provides gRPC API for session management
type GRPCServer struct {
	config *GRPCConfig
	server *grpc.Server
	logger *slog.Logger
}

// GRPCConfig holds gRPC server configuration
type GRPCConfig struct {
	Address string
	Port    int
}

// NewGRPCServer creates a new gRPC server
func NewGRPCServer(config *GRPCConfig, logger *slog.Logger) *GRPCServer {
	server := grpc.NewServer()

	// Register health service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(server, healthServer)

	return &GRPCServer{
		config: config,
		server: server,
		logger: logger,
	}
}

// Start starts the gRPC server
func (g *GRPCServer) Start(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", g.config.Address, g.config.Port)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	g.logger.Info("gRPC server starting", "address", addr)

	go func() {
		if err := g.server.Serve(listener); err != nil {
			g.logger.Error("gRPC server error", "error", err)
		}
	}()

	return nil
}

// Stop stops the gRPC server
func (g *GRPCServer) Stop(ctx context.Context) error {
	if g.server != nil {
		g.logger.Info("gRPC server stopping")
		g.server.GracefulStop()
	}
	return nil
}
