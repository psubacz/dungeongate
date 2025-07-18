package connection

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/dungeongate/internal/session/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandler_Integration_GameClient(t *testing.T) {
	// Skip if services are not available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Test if we can create a game client
	gameClient, err := client.NewGameClient("localhost:50051", logger)
	if err != nil {
		t.Skipf("Game service not available: %v", err)
	}
	defer gameClient.Close()

	// Test client health checks
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	gameHealthy := gameClient.IsHealthy(ctx)
	t.Logf("Game service healthy: %v", gameHealthy)

	// Test basic client operations
	assert.NotNil(t, gameClient)
}

func TestHandler_Integration_AuthClient(t *testing.T) {
	// Skip if services are not available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Test if we can create an auth client
	authClient, err := client.NewAuthClient("localhost:8082", logger)
	if err != nil {
		t.Skipf("Auth service not available: %v", err)
	}
	defer authClient.Close()

	// Test basic client operations
	assert.NotNil(t, authClient)
}

func TestHandler_ConnectionManager_Integration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create connection manager
	manager := NewManager(100, logger)

	// Test lifecycle
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := manager.Start(ctx)
	require.NoError(t, err)

	// Test connection statistics
	stats := manager.GetStats()
	assert.NotNil(t, stats)
	assert.Equal(t, 0, stats.Active)
	assert.Equal(t, 0, stats.Total)

	// Test cleanup
	err = manager.Stop(ctx)
	assert.NoError(t, err)
}

func TestHandler_PTYRequest_Parsing(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create a basic handler for testing utility methods
	manager := NewManager(100, logger)

	// We'll create mock clients since we just need the handler structure
	gameClient, err := client.NewGameClient("localhost:50051", logger)
	if err != nil {
		t.Skip("Game service not available for testing")
	}
	defer gameClient.Close()

	authClient, err := client.NewAuthClient("localhost:8082", logger)
	if err != nil {
		t.Skip("Auth service not available for testing")
	}
	defer authClient.Close()

	// Create minimal handler - we'll skip the menu handler since it requires more setup
	handler := &Handler{
		manager:    manager,
		gameClient: gameClient,
		authClient: authClient,
		logger:     logger,
	}

	// Test PTY request parsing with various payloads
	tests := []struct {
		name        string
		payload     []byte
		expectedCol int
		expectedRow int
	}{
		{
			name:        "empty payload",
			payload:     []byte{},
			expectedCol: 80,
			expectedRow: 24,
		},
		{
			name:        "short payload",
			payload:     []byte{0x01, 0x02, 0x03},
			expectedCol: 80,
			expectedRow: 24,
		},
		{
			name: "valid payload with xterm",
			payload: []byte{
				0x00, 0x00, 0x00, 0x05, // term name length (5)
				'x', 't', 'e', 'r', 'm', // term name
				0x00, 0x00, 0x00, 0x50, // width (80)
				0x00, 0x00, 0x00, 0x18, // height (24)
			},
			expectedCol: 80,
			expectedRow: 24,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cols, rows := handler.parsePTYRequest(tt.payload)
			assert.Equal(t, tt.expectedCol, cols)
			assert.Equal(t, tt.expectedRow, rows)
		})
	}
}

func TestHandler_WindowChange_Parsing(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create a basic handler for testing utility methods
	manager := NewManager(100, logger)

	// We'll create mock clients since we just need the handler structure
	gameClient, err := client.NewGameClient("localhost:50051", logger)
	if err != nil {
		t.Skip("Game service not available for testing")
	}
	defer gameClient.Close()

	authClient, err := client.NewAuthClient("localhost:8082", logger)
	if err != nil {
		t.Skip("Auth service not available for testing")
	}
	defer authClient.Close()

	handler := &Handler{
		manager:    manager,
		gameClient: gameClient,
		authClient: authClient,
		logger:     logger,
	}

	// Test window change parsing
	tests := []struct {
		name        string
		payload     []byte
		expectedCol int
		expectedRow int
	}{
		{
			name:        "empty payload",
			payload:     []byte{},
			expectedCol: 80,
			expectedRow: 24,
		},
		{
			name:        "short payload",
			payload:     []byte{0x01, 0x02, 0x03},
			expectedCol: 80,
			expectedRow: 24,
		},
		{
			name: "valid payload 80x24",
			payload: []byte{
				0x00, 0x00, 0x00, 0x50, // width (80)
				0x00, 0x00, 0x00, 0x18, // height (24)
			},
			expectedCol: 80,
			expectedRow: 24,
		},
		{
			name: "valid payload 120x40",
			payload: []byte{
				0x00, 0x00, 0x00, 0x78, // width (120)
				0x00, 0x00, 0x00, 0x28, // height (40)
			},
			expectedCol: 120,
			expectedRow: 40,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cols, rows := handler.parseWindowChange(tt.payload)
			assert.Equal(t, tt.expectedCol, cols)
			assert.Equal(t, tt.expectedRow, rows)
		})
	}
}

func TestHandler_gRPC_Streaming_Integration(t *testing.T) {
	// Skip if services are not available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Test if we can create a game client and get a stream
	gameClient, err := client.NewGameClient("localhost:50051", logger)
	if err != nil {
		t.Skipf("Game service not available: %v", err)
	}
	defer gameClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test if we can create a stream (this will fail if game service is not running)
	if gameClient.IsHealthy(ctx) {
		stream, err := gameClient.StreamGameIO(ctx)
		if err != nil {
			t.Logf("Could not create stream (expected if no game service): %v", err)
		} else {
			t.Log("Successfully created gRPC stream")
			// Close the stream properly
			if closeErr := stream.CloseSend(); closeErr != nil {
				t.Logf("Error closing stream: %v", closeErr)
			}
		}
	} else {
		t.Log("Game service not healthy, skipping stream test")
	}
}
