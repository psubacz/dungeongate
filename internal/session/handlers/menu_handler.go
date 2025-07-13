package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/dungeongate/internal/session/pools"
	"github.com/dungeongate/internal/session/resources"
	"github.com/dungeongate/internal/session-old/menu"
	authv1 "github.com/dungeongate/pkg/api/auth/v1"
	"golang.org/x/crypto/ssh"
)

// PoolAwareMenuHandler enhances the existing menu system with pool awareness
type PoolAwareMenuHandler struct {
	*menu.MenuHandler // Embed existing functionality
	workerPool        *pools.WorkerPool
	resourceLimiter   *resources.ResourceLimiter
	connectionPool    *pools.ConnectionPool
	authHandler       *AuthHandler
	gameHandler       *GameHandler
	streamHandler     *StreamHandler
	logger            *slog.Logger

	// Metrics
	menuActions     *resources.CounterMetric
	menuDuration    *resources.HistogramMetric
	menuErrors      *resources.CounterMetric
	userSessions    *resources.GaugeMetric
}

// NewPoolAwareMenuHandler creates a pool-aware menu handler
func NewPoolAwareMenuHandler(
	menuHandler *menu.MenuHandler,
	workerPool *pools.WorkerPool,
	resourceLimiter *resources.ResourceLimiter,
	connectionPool *pools.ConnectionPool,
	authHandler *AuthHandler,
	gameHandler *GameHandler,
	streamHandler *StreamHandler,
	metricsRegistry *resources.MetricsRegistry,
	logger *slog.Logger,
) *PoolAwareMenuHandler {
	pmh := &PoolAwareMenuHandler{
		MenuHandler:     menuHandler,
		workerPool:      workerPool,
		resourceLimiter: resourceLimiter,
		connectionPool:  connectionPool,
		authHandler:     authHandler,
		gameHandler:     gameHandler,
		streamHandler:   streamHandler,
		logger:          logger,
	}

	pmh.initializeMetrics(metricsRegistry)
	return pmh
}

// HandleMenuLoop manages the main session lifecycle with pool awareness
func (pmh *PoolAwareMenuHandler) HandleMenuLoop(ctx context.Context, conn *pools.Connection, channel ssh.Channel, userInfo interface{}, terminalCols, terminalRows int) error {
	pmh.userSessions.Inc()
	defer pmh.userSessions.Dec()

	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		pmh.menuDuration.Observe(duration.Seconds())
		pmh.logger.Info("Menu loop completed",
			"connection_id", conn.ID,
			"user_id", conn.UserID,
			"duration", duration)
	}()

	// Main menu loop
	for {
		// Refresh user info before showing menu
		currentUserInfo, err := pmh.authHandler.GetUserInfo(ctx, conn.SSHConn)
		if err == nil && currentUserInfo != nil {
			userInfo = currentUserInfo
			conn.UserID = currentUserInfo.Id
			conn.Username = currentUserInfo.Username
		}

		// Show appropriate menu based on authentication status
		var menuChoice *menu.MenuChoice
		if userInfo == nil {
			// Show anonymous menu
			menuChoice, err = pmh.MenuHandler.ShowAnonymousMenu(ctx, channel, conn.Username)
		} else {
			// Show authenticated user menu
			if authUser, ok := userInfo.(*authv1.User); ok {
				menuChoice, err = pmh.MenuHandler.ShowUserMenu(ctx, channel, authUser.Username)
			} else {
				pmh.logger.Error("Invalid user info type", "connection_id", conn.ID)
				return fmt.Errorf("invalid user info type")
			}
		}

		if err != nil {
			// Check if this is a graceful disconnection
			if strings.Contains(err.Error(), "EOF") || err.Error() == "user quit" {
				return nil // Normal disconnect
			}
			pmh.logger.Error("Error in menu handler",
				"error", err,
				"connection_id", conn.ID,
				"username", conn.Username)
			pmh.menuErrors.Inc()
			if ctx.Err() != nil {
				return ctx.Err() // Context cancelled
			}
			continue // Redisplay menu
		}

		// Handle menu choice using pool-aware execution
		if err := pmh.ExecuteAction(ctx, conn, menuChoice, userInfo, terminalCols, terminalRows); err != nil {
			if err.Error() == "user quit" {
				return nil // User chose to quit
			}
			pmh.logger.Error("Error handling menu choice",
				"error", err,
				"choice", menuChoice.Action,
				"connection_id", conn.ID,
				"username", conn.Username)
			pmh.menuErrors.Inc()
			if ctx.Err() != nil {
				return ctx.Err() // Context cancelled
			}
			continue // Return to menu
		}
	}
}

// ExecuteAction executes a menu action with resource limits and pool awareness
func (pmh *PoolAwareMenuHandler) ExecuteAction(ctx context.Context, conn *pools.Connection, choice *menu.MenuChoice, userInfo interface{}, terminalCols, terminalRows int) error {
	startTime := time.Now()
	pmh.menuActions.Inc()

	defer func() {
		duration := time.Since(startTime)
		pmh.logger.Info("Menu action completed",
			"action", choice.Action,
			"connection_id", conn.ID,
			"user_id", conn.UserID,
			"duration", duration)
	}()

	// Check resource limits before execution
	if !pmh.resourceLimiter.CanExecute(conn.UserID, choice.Action) {
		pmh.logger.Warn("Action blocked by resource limiter",
			"action", choice.Action,
			"user_id", conn.UserID,
			"connection_id", conn.ID)
		pmh.menuErrors.Inc()
		conn.SSHChannel.Write([]byte("Resource limit exceeded. Please try again later.\r\n"))
		time.Sleep(2 * time.Second)
		return nil // Return to menu, don't quit
	}

	// Create work item based on action type
	workType := pmh.getWorkTypeForAction(choice.Action)
	work := &pools.WorkItem{
		Type:       workType,
		Connection: conn,
		Handler:    pmh.getHandlerForAction(choice.Action, userInfo, terminalCols, terminalRows),
		Context:    ctx,
		Priority:   pmh.getPriorityForAction(choice.Action),
		QueuedAt:   time.Now(),
		Data:       choice,
	}

	// For immediate actions (like quit), don't use worker pool
	if choice.Action == "quit" {
		conn.SSHChannel.Write([]byte("Goodbye!\r\n"))
		return fmt.Errorf("user quit")
	}

	// For other actions, check if they should be executed immediately or via worker pool
	if pmh.shouldExecuteImmediately(choice.Action) {
		return pmh.executeActionDirectly(ctx, conn, choice, userInfo, terminalCols, terminalRows)
	}

	// Submit to worker pool for non-immediate actions
	if err := pmh.workerPool.Submit(work); err != nil {
		pmh.logger.Error("Failed to submit menu action work",
			"error", err,
			"action", choice.Action,
			"connection_id", conn.ID)
		pmh.menuErrors.Inc()
		return fmt.Errorf("failed to submit work: %w", err)
	}

	return nil
}

// shouldExecuteImmediately determines if an action should be executed immediately
func (pmh *PoolAwareMenuHandler) shouldExecuteImmediately(action string) bool {
	immediateActions := map[string]bool{
		"login":    true,
		"register": true,
		"credit":   true,
	}
	return immediateActions[action]
}

// executeActionDirectly executes an action immediately without using the worker pool
func (pmh *PoolAwareMenuHandler) executeActionDirectly(ctx context.Context, conn *pools.Connection, choice *menu.MenuChoice, userInfo interface{}, terminalCols, terminalRows int) error {
	switch choice.Action {
	case "login":
		return pmh.authHandler.HandleLogin(ctx, conn, conn.SSHChannel)

	case "register":
		for {
			err := pmh.authHandler.HandleRegister(ctx, conn, conn.SSHChannel)
			if err != nil && err.Error() == "retry_register" {
				continue // User chose to retry registration
			}
			return err // Either success, user quit, or other error
		}

	case "credit":
		return pmh.handleCredits(ctx, conn)

	default:
		return fmt.Errorf("unknown immediate action: %s", choice.Action)
	}
}

// getWorkTypeForAction determines the work type for a menu action
func (pmh *PoolAwareMenuHandler) getWorkTypeForAction(action string) pools.WorkType {
	switch action {
	case "login", "register":
		return pools.WorkTypeAuthentication
	case "play", "start_game", "list_games":
		return pools.WorkTypeGameIO
	case "watch":
		return pools.WorkTypeStreamManagement
	default:
		return pools.WorkTypeMenuAction
	}
}

// getHandlerForAction returns the appropriate handler function for an action
func (pmh *PoolAwareMenuHandler) getHandlerForAction(action string, userInfo interface{}, terminalCols, terminalRows int) pools.HandlerFunc {
	return func(ctx context.Context, conn *pools.Connection) error {
		choice, ok := conn.Context.Value("work_data").(*menu.MenuChoice)
		if !ok {
			return fmt.Errorf("invalid menu choice data")
		}

		switch action {
		case "play", "list_games":
			if authUser, ok := userInfo.(*authv1.User); ok {
				return pmh.gameHandler.HandleGameSelection(ctx, conn, authUser)
			}
			conn.SSHChannel.Write([]byte("Please login first to play games.\r\n"))
			time.Sleep(2 * time.Second)
			return nil

		case "start_game":
			if authUser, ok := userInfo.(*authv1.User); ok {
				return pmh.gameHandler.StartGameSession(ctx, conn, authUser, choice.Value)
			}
			conn.SSHChannel.Write([]byte("Please login first to play games.\r\n"))
			time.Sleep(2 * time.Second)
			return nil

		case "watch":
			return pmh.streamHandler.HandleSpectating(ctx, conn, choice.Value)

		case "edit_profile":
			conn.SSHChannel.Write([]byte("Profile editing functionality not yet implemented.\r\n"))
			time.Sleep(2 * time.Second)
			return nil

		case "view_recordings":
			conn.SSHChannel.Write([]byte("Recording viewing functionality not yet implemented.\r\n"))
			time.Sleep(2 * time.Second)
			return nil

		case "statistics":
			conn.SSHChannel.Write([]byte("Statistics functionality not yet implemented.\r\n"))
			time.Sleep(2 * time.Second)
			return nil

		default:
			conn.SSHChannel.Write([]byte(fmt.Sprintf("Unknown action: %s\r\n", action)))
			time.Sleep(2 * time.Second)
			return nil
		}
	}
}

// getPriorityForAction determines the priority for a menu action
func (pmh *PoolAwareMenuHandler) getPriorityForAction(action string) pools.Priority {
	switch action {
	case "quit":
		return pools.PriorityCritical
	case "login", "register":
		return pools.PriorityHigh
	case "play", "start_game":
		return pools.PriorityNormal
	default:
		return pools.PriorityLow
	}
}

// handleCredits shows the credits screen
func (pmh *PoolAwareMenuHandler) handleCredits(ctx context.Context, conn *pools.Connection) error {
	// Clear screen and show credits with ASCII art
	conn.SSHChannel.Write([]byte("\033[2J\033[H"))
	conn.SSHChannel.Write([]byte("\r\n"))

	// DungeonGate ASCII Art
	conn.SSHChannel.Write([]byte(" ____\r\n"))
	conn.SSHChannel.Write([]byte("|  _ \\ _   _ _ __   __ _  ___  ___  _ __\r\n"))
	conn.SSHChannel.Write([]byte("| | | | | | | ._ \\ / _. |/ _ \\/ _ \\| ._ \\\r\n"))
	conn.SSHChannel.Write([]byte("| |_| | |_| | | | | (_| |  __/ (_) | | | |\r\n"))
	conn.SSHChannel.Write([]byte("|____/ \\__,_|_| |_|\\__, |\\___|\\____| |_| |\r\n"))
	conn.SSHChannel.Write([]byte("        ___        |___/\r\n"))
	conn.SSHChannel.Write([]byte("       / __|  __ _| |_ ___\r\n"))
	conn.SSHChannel.Write([]byte("      | |___ / _. | __/ _ \\\r\n"))
	conn.SSHChannel.Write([]byte("      | |__ | (_| |  ||  _/\r\n"))
	conn.SSHChannel.Write([]byte("      |____/ \\__,_|\\__\\___|\r\n"))
	conn.SSHChannel.Write([]byte("\r\n"))

	// Credits information
	conn.SSHChannel.Write([]byte("=== Credits ===\r\n\r\n"))
	conn.SSHChannel.Write([]byte("DungeonGate - Terminal Game Platform\r\n"))
	conn.SSHChannel.Write([]byte("Developed with <3 and Claude Code\r\n\r\n"))
	conn.SSHChannel.Write([]byte("Directed by Peter Subacz \r\n\r\n"))
	conn.SSHChannel.Write([]byte("Press any key to return to menu...\r\n"))

	// Wait for any key press to return
	buffer := make([]byte, 1)
	_, err := conn.SSHChannel.Read(buffer)
	return err
}

// initializeMetrics sets up metrics for the menu handler
func (pmh *PoolAwareMenuHandler) initializeMetrics(registry *resources.MetricsRegistry) {
	pmh.menuActions = registry.RegisterCounter(
		"session_menu_actions_total",
		"Total number of menu actions executed",
		map[string]string{"handler": "menu"})

	pmh.menuDuration = registry.RegisterHistogram(
		"session_menu_action_duration_seconds",
		"Time spent processing menu actions",
		nil,
		map[string]string{"handler": "menu"})

	pmh.menuErrors = registry.RegisterCounter(
		"session_menu_errors_total",
		"Total number of menu errors",
		map[string]string{"handler": "menu"})

	pmh.userSessions = registry.RegisterGauge(
		"session_user_sessions_active",
		"Number of active user sessions",
		map[string]string{"handler": "menu"})
}