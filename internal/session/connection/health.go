package connection

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/dungeongate/internal/session/client"
	"github.com/dungeongate/internal/session/menu"
	"golang.org/x/crypto/ssh"
)

// ServiceHealthChecker handles service health monitoring and unavailable service handling
type ServiceHealthChecker struct {
	authClient  *client.AuthClient
	gameClient  *client.GameClient
	menuHandler *menu.MenuHandler
	logger      *slog.Logger
}

// NewServiceHealthChecker creates a new service health checker
func NewServiceHealthChecker(
	authClient *client.AuthClient,
	gameClient *client.GameClient,
	menuHandler *menu.MenuHandler,
	logger *slog.Logger,
) *ServiceHealthChecker {
	return &ServiceHealthChecker{
		authClient:  authClient,
		gameClient:  gameClient,
		menuHandler: menuHandler,
		logger:      logger,
	}
}

// CheckServiceHealth checks the health of all required services and returns status
func (h *ServiceHealthChecker) CheckServiceHealth(ctx context.Context) (bool, string) {
	var unavailableServices []string

	// Check Auth Service
	if !h.authClient.IsHealthy(ctx) {
		unavailableServices = append(unavailableServices, "• Auth Service: Unavailable")
	}

	// Check Game Service
	if !h.gameClient.IsHealthy(ctx) {
		unavailableServices = append(unavailableServices, "• Game Service: Unavailable")
	}

	// Format status message
	if len(unavailableServices) == 0 {
		return true, "All services are operational. Please restart the connection."
	}

	statusMessage := strings.Join(unavailableServices, "\n│ ")
	return false, statusMessage
}

// HandleServiceUnavailable displays service unavailable message and auto-disconnects after 5 minutes
func (h *ServiceHealthChecker) HandleServiceUnavailable(ctx context.Context, channel ssh.Channel, connID, username string) error {
	h.logger.Info("Services unavailable, entering maintenance mode", "username", username, "connection_id", connID)

	// Clear screen and position cursor at top
	if _, err := channel.Write([]byte("\033[2J\033[H")); err != nil {
		if err == io.EOF {
			return fmt.Errorf("connection closed")
		}
	}

	// 5 minute timeout (300 seconds)
	totalTimeout := 5 * time.Minute
	startTime := time.Now()

	// Set up display update timer - update every second
	updateTicker := time.NewTicker(1 * time.Second)
	defer updateTicker.Stop()

	// Handle input for immediate quit
	inputChan := make(chan byte, 1)
	errorChan := make(chan error, 1)

	// Start input reading goroutine
	go func() {
		buffer := make([]byte, 1)
		for {
			n, err := channel.Read(buffer)
			if err != nil {
				select {
				case errorChan <- err:
				default:
				}
				return
			}
			if n > 0 {
				select {
				case inputChan <- buffer[0]:
				default:
				}
			}
		}
	}()

	// Initial display
	elapsed := time.Since(startTime)
	remaining := totalTimeout - elapsed
	remainingMinutes := int(remaining.Minutes())
	remainingSeconds := int(remaining.Seconds()) % 60

	// Get current service status
	_, serviceStatus := h.CheckServiceHealth(ctx)

	banner, err := h.menuHandler.RenderServiceUnavailable(username, remainingMinutes, remainingSeconds, serviceStatus)
	if err != nil {
		h.logger.Error("Failed to render service unavailable banner", "error", err, "username", username)
		return fmt.Errorf("failed to render banner: %w", err)
	}

	_, err = channel.Write([]byte(banner))
	if err != nil {
		if err == io.EOF {
			return fmt.Errorf("connection closed")
		}
		h.logger.Error("Failed to write service unavailable banner", "error", err, "username", username)
		return fmt.Errorf("failed to write banner: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled")
		case err := <-errorChan:
			if err == io.EOF {
				return fmt.Errorf("connection closed")
			}
			h.logger.Debug("Error reading from channel in service unavailable mode", "error", err, "username", username)
			return fmt.Errorf("read error: %w", err)
		case input := <-inputChan:
			// Handle user input - only 'q' to quit
			if strings.ToLower(string(input)) == "q" {
				h.logger.Info("User pressed 'q' to quit during maintenance", "username", username)
				channel.Write([]byte("\r\n\r\nGoodbye!\r\n"))
				return fmt.Errorf("user quit")
			}
			// Ignore all other input
		case <-updateTicker.C:
			// Update countdown display every second
			elapsed := time.Since(startTime)
			remaining := totalTimeout - elapsed

			if remaining <= 0 {
				// Time's up, auto-disconnect
				h.logger.Info("Service unavailable timeout reached, disconnecting user", "username", username)
				channel.Write([]byte("\r\n\r\nConnection timeout reached. Please try again later.\r\nGoodbye!\r\n"))
				return fmt.Errorf("user quit")
			}

			// Update display
			remainingMinutes := int(remaining.Minutes())
			remainingSeconds := int(remaining.Seconds()) % 60

			// Get current service status
			_, serviceStatus := h.CheckServiceHealth(ctx)

			banner, err := h.menuHandler.RenderServiceUnavailable(username, remainingMinutes, remainingSeconds, serviceStatus)
			if err == nil {
				channel.Write([]byte("\033[2J\033[H" + banner))
			}
		}
	}
}
