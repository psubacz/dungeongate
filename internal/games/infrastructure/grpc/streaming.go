package grpc

import (
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/dungeongate/internal/games"
	"github.com/dungeongate/internal/games/infrastructure/pty"
	games_pb "github.com/dungeongate/pkg/api/games/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// StreamHandler handles PTY streaming for game sessions
type StreamHandler struct {
	ptyManager *pty.PTYManager
	sessions   map[string]*StreamSession
	mu         sync.RWMutex
	logger     *slog.Logger
}

// StreamSession represents an active streaming session
type StreamSession struct {
	sessionID  string
	ptySession *pty.PTYSession
	stream     games_pb.GameService_StreamGameIOServer
	closeChan  chan struct{}
	closeOnce  sync.Once
}

// GRPCSpectatorConnection implements SpectatorConnection for gRPC streams
type GRPCSpectatorConnection struct {
	stream    games_pb.GameService_StreamGameIOServer
	sessionID string
	logger    *slog.Logger
	closed    bool
	mu        sync.RWMutex
}

// NewGRPCSpectatorConnection creates a new gRPC spectator connection
func NewGRPCSpectatorConnection(stream games_pb.GameService_StreamGameIOServer, sessionID string, logger *slog.Logger) *GRPCSpectatorConnection {
	return &GRPCSpectatorConnection{
		stream:    stream,
		sessionID: sessionID,
		logger:    logger,
	}
}

// Write implements SpectatorConnection.Write
func (c *GRPCSpectatorConnection) Write(frame *games.StreamFrame) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("connection closed")
	}

	return c.stream.Send(&games_pb.GameIOResponse{
		Response: &games_pb.GameIOResponse_Output{
			Output: &games_pb.PTYOutput{
				SessionId: c.sessionID,
				Data:      frame.Data,
			},
		},
	})
}

// Close implements SpectatorConnection.Close
func (c *GRPCSpectatorConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.closed = true
	return nil
}

// GetType implements SpectatorConnection.GetType
func (c *GRPCSpectatorConnection) GetType() string {
	return "grpc"
}

// IsConnected implements SpectatorConnection.IsConnected
func (c *GRPCSpectatorConnection) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return !c.closed
}

// NewStreamHandler creates a new stream handler
func NewStreamHandler(ptyManager *pty.PTYManager, logger *slog.Logger) *StreamHandler {
	return &StreamHandler{
		ptyManager: ptyManager,
		sessions:   make(map[string]*StreamSession),
		logger:     logger,
	}
}

// HandleStream handles a bidirectional streaming connection
func (h *StreamHandler) HandleStream(stream games_pb.GameService_StreamGameIOServer) error {
	h.logger.Info("New PTY streaming connection")

	// The first message should be a connect request
	req, err := stream.Recv()
	if err != nil {
		return status.Error(codes.InvalidArgument, "failed to receive initial request")
	}

	// Validate it's a connect request
	connectReq := req.GetConnect()
	if connectReq == nil {
		return status.Error(codes.InvalidArgument, "first message must be a connect request")
	}

	sessionID := connectReq.SessionId
	if sessionID == "" {
		return status.Error(codes.InvalidArgument, "session_id is required")
	}

	// Get the PTY session
	ptySession, err := h.ptyManager.GetPTY(sessionID)
	if err != nil {
		h.logger.Error("PTY not found", "session_id", sessionID, "error", err)
		// Send error response
		if err := stream.Send(&games_pb.GameIOResponse{
			Response: &games_pb.GameIOResponse_Connected{
				Connected: &games_pb.ConnectPTYResponse{
					Success: false,
					Error:   fmt.Sprintf("PTY not found: %v", err),
				},
			},
		}); err != nil {
			return err
		}
		return status.Error(codes.NotFound, "PTY session not found")
	}

	// Create stream session
	streamSession := &StreamSession{
		sessionID:  sessionID,
		ptySession: ptySession,
		stream:     stream,
		closeChan:  make(chan struct{}),
	}

	// Register the stream session
	h.mu.Lock()
	h.sessions[sessionID] = streamSession
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.sessions, sessionID)
		h.mu.Unlock()
		streamSession.Close()
	}()

	// Send success response
	if err := stream.Send(&games_pb.GameIOResponse{
		Response: &games_pb.GameIOResponse_Connected{
			Connected: &games_pb.ConnectPTYResponse{
				Success: true,
				PtyId:   sessionID,
			},
		},
	}); err != nil {
		return err
	}

	// Send current screen state to new connection
	if streamManager := ptySession.GetStreamManager(); streamManager != nil {
		// Get recent frames to provide some context to the new connection
		recentFrames := streamManager.GetRecentFrames()

		// If we have recent frames, send them to the new connection
		if len(recentFrames) > 0 {
			h.logger.Info("Sending recent frames to new connection", "session_id", sessionID, "frame_count", len(recentFrames))

			// Send the frames in chronological order
			for _, frame := range recentFrames {
				if err := stream.Send(&games_pb.GameIOResponse{
					Response: &games_pb.GameIOResponse_Output{
						Output: &games_pb.PTYOutput{
							SessionId: sessionID,
							Data:      frame.Data,
						},
					},
				}); err != nil {
					h.logger.Error("Failed to send frame to connection", "error", err, "session_id", sessionID, "frame_id", frame.FrameID)
					return err
				}
			}
		} else {
			// No recent frames available - send a clear screen and trigger redraw
			h.logger.Info("No recent frames available, requesting fresh screen state", "session_id", sessionID)

			// Clear the connection's screen
			clearScreen := []byte("\x1b[2J\x1b[H") // Clear screen and move cursor to home
			if err := stream.Send(&games_pb.GameIOResponse{
				Response: &games_pb.GameIOResponse_Output{
					Output: &games_pb.PTYOutput{
						SessionId: sessionID,
						Data:      clearScreen,
					},
				},
			}); err != nil {
				h.logger.Error("Failed to send clear screen to connection", "error", err, "session_id", sessionID)
				return err
			}

			// Send screen redraw command to game to capture full current state
			// NetHack responds to Ctrl+L (redraw) command
			redrawCmd := []byte{0x0C} // Ctrl+L
			if err := ptySession.SendInput(redrawCmd); err != nil {
				h.logger.Warn("Failed to send redraw command to game", "error", err, "session_id", sessionID)
			}

			h.logger.Info("Sent redraw command to game for new connection", "session_id", sessionID)
		}
	}

	// Start goroutines for handling I/O
	errChan := make(chan error, 2)

	// Handle PTY output -> stream (for player connections)
	go func() {
		errChan <- h.handlePTYOutput(streamSession)
	}()

	// Handle stream input -> PTY
	go func() {
		errChan <- h.handleStreamInput(streamSession)
	}()

	// Wait for either goroutine to finish
	err = <-errChan

	// Send disconnect response if possible
	stream.Send(&games_pb.GameIOResponse{
		Response: &games_pb.GameIOResponse_Disconnected{
			Disconnected: &games_pb.DisconnectPTYResponse{
				Success: true,
			},
		},
	})

	return err
}

// handlePTYOutput reads from PTY and sends to stream
func (h *StreamHandler) handlePTYOutput(session *StreamSession) error {
	// Create a unique subscription ID for this connection
	subscriptionID := fmt.Sprintf("grpc_%p", session.stream)

	// Subscribe to PTY output
	outputChan := session.ptySession.SubscribeToOutput(subscriptionID)
	errorChan := session.ptySession.GetError()

	// Ensure we unsubscribe when done
	defer session.ptySession.UnsubscribeFromOutput(subscriptionID)

	for {
		select {
		case data, ok := <-outputChan:
			if !ok {
				h.logger.Debug("Output channel closed for session", "session_id", session.sessionID)
				return io.EOF
			}

			h.logger.Debug("Sending bytes to stream for session", "session_id", session.sessionID, "bytes", len(data), "data", string(data))
			// Send output to stream
			if err := session.stream.Send(&games_pb.GameIOResponse{
				Response: &games_pb.GameIOResponse_Output{
					Output: &games_pb.PTYOutput{
						SessionId: session.sessionID,
						Data:      data,
					},
				},
			}); err != nil {
				h.logger.Error("Failed to send to stream for session", "session_id", session.sessionID, "error", err)
				h.logger.Error("Failed to send PTY output", "error", err, "session_id", session.sessionID)
				return err
			}
			h.logger.Debug("Successfully sent to stream for session", "session_id", session.sessionID)

		case err := <-errorChan:
			// Get exit code if available
			exitCode, _ := session.ptySession.GetExitCode()

			// Send process exit event
			if err := session.stream.Send(&games_pb.GameIOResponse{
				Response: &games_pb.GameIOResponse_Event{
					Event: &games_pb.PTYEvent{
						SessionId: session.sessionID,
						Type:      games_pb.PTYEventType_PTY_EVENT_PROCESS_EXIT,
						Message:   fmt.Sprintf("Process exited: %v", err),
						Metadata: map[string]string{
							"exit_code": fmt.Sprintf("%d", exitCode),
						},
					},
				},
			}); err != nil {
				h.logger.Error("Failed to send exit event", "error", err, "session_id", session.sessionID)
			}
			return err

		case <-session.closeChan:
			return nil
		}
	}
}

// handleStreamInput reads from stream and sends to PTY
func (h *StreamHandler) handleStreamInput(session *StreamSession) error {
	for {
		req, err := session.stream.Recv()
		if err != nil {
			if err == io.EOF {
				h.logger.Info("Stream closed by client", "session_id", session.sessionID)
				return nil
			}
			h.logger.Error("Failed to receive from stream", "error", err, "session_id", session.sessionID)
			return err
		}

		// Handle different request types
		switch reqType := req.Request.(type) {
		case *games_pb.GameIORequest_Input:
			// Send input to PTY
			if err := session.ptySession.SendInput(reqType.Input.Data); err != nil {
				h.logger.Error("Failed to send input to PTY", "error", err, "session_id", session.sessionID)
				return err
			}

		case *games_pb.GameIORequest_Disconnect:
			// Client requested disconnect
			h.logger.Info("Client requested disconnect", "session_id", session.sessionID, "reason", reqType.Disconnect.Reason)
			return nil

		default:
			h.logger.Warn("Unexpected request type during streaming", "type", fmt.Sprintf("%T", reqType))
		}
	}
}

// Close closes a stream session
func (s *StreamSession) Close() {
	s.closeOnce.Do(func() {
		close(s.closeChan)
	})
}
