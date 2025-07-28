package menu

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/dungeongate/internal/session/banner"
	"github.com/dungeongate/internal/session/client"
	"github.com/dungeongate/internal/session/terminal"
	authv1 "github.com/dungeongate/pkg/api/auth/v1"
	gamev2 "github.com/dungeongate/pkg/api/games/v2"
	"golang.org/x/crypto/ssh"
)

// MenuHandler handles the main menu display and user interaction
type MenuHandler struct {
	bannerManager *banner.BannerManager
	gameClient    *client.GameClient
	authClient    *client.AuthClient
	logger        *slog.Logger
}

// NewMenuHandler creates a new menu handler
func NewMenuHandler(bannerManager *banner.BannerManager, gameClient *client.GameClient, authClient *client.AuthClient, logger *slog.Logger) *MenuHandler {
	return &MenuHandler{
		bannerManager: bannerManager,
		gameClient:    gameClient,
		authClient:    authClient,
		logger:        logger,
	}
}

// MenuChoice represents a user's menu choice
type MenuChoice struct {
	Action string
	Value  string
}

// InputValidator handles menu input validation and error messages
type InputValidator struct {
	ValidOptions []string
	MenuName     string
}

// ValidateInput checks if input is valid and returns appropriate error message
func (iv *InputValidator) ValidateInput(input string) (bool, string) {
	inputLower := strings.ToLower(input)

	for _, option := range iv.ValidOptions {
		if inputLower == strings.ToLower(option) {
			return true, ""
		}
	}

	// Create helpful error message
	optionsList := strings.Join(iv.ValidOptions, ", ")
	errorMsg := fmt.Sprintf("Invalid choice '%s'. Valid options: %s\r\n", input, optionsList)
	return false, errorMsg
}

// handleCtrlD processes Ctrl+D input consistently across all menus
func handleCtrlD() *MenuChoice {
	return &MenuChoice{Action: "quit", Value: ""}
}

// handleInvalidInput shows error message and redisplays menu
func (mh *MenuHandler) handleInvalidInput(channel ssh.Channel, errorMsg, banner string) error {
	// Show error message
	if _, err := channel.Write([]byte(errorMsg)); err != nil {
		if err == io.EOF {
			return err
		}
		return fmt.Errorf("failed to write error message: %w", err)
	}

	// Brief pause for user to read
	time.Sleep(1 * time.Second)

	// Clear screen and redisplay menu
	if _, err := channel.Write([]byte("\033[2J\033[H")); err != nil {
		if err == io.EOF {
			return err
		}
	}

	if _, err := channel.Write([]byte(banner)); err != nil {
		if err == io.EOF {
			return err
		}
		return fmt.Errorf("failed to redisplay banner: %w", err)
	}

	return nil
}

// ShowAnonymousMenu displays the main menu for anonymous users and handles input
func (mh *MenuHandler) ShowAnonymousMenu(ctx context.Context, channel ssh.Channel, username string) (*MenuChoice, error) {
	// Create input validator for anonymous menu
	validator := &InputValidator{
		ValidOptions: []string{"[L]ogin", "[R]egister", "[W]atch", "[C]redits", "[Q]uit"},
		MenuName:     "Anonymous Menu",
	}

	// Clear screen and position cursor at top
	if _, err := channel.Write([]byte("\033[2J\033[H")); err != nil {
		if err == io.EOF {
			return handleCtrlD(), nil
		}
	}

	// Render the anonymous banner
	banner, err := mh.bannerManager.RenderMainAnon()
	if err != nil {
		mh.logger.Error("Failed to render anonymous banner", "error", err)
		return nil, fmt.Errorf("failed to render banner: %w", err)
	}

	// Display the banner
	_, err = channel.Write([]byte(banner))
	if err != nil {
		if err == io.EOF {
			return handleCtrlD(), nil
		}
		return nil, fmt.Errorf("failed to write banner: %w", err)
	}

	// Create terminal input handler for proper keyboard support
	inputHandler := terminal.NewInputHandler(channel)

	// Wait for user input
	for {
		event, err := inputHandler.ReadInput(ctx)
		if err != nil {
			if err.Error() == "user cancelled" {
				return handleCtrlD(), nil
			}
			return nil, fmt.Errorf("failed to read user input: %w", err)
		}

		// Handle character input for menu choices
		if event.Type == terminal.EventCharacter {
			choice := string(event.Character)

			switch strings.ToLower(choice) {
			case "l":
				return &MenuChoice{Action: "login", Value: ""}, nil
			case "r":
				return &MenuChoice{Action: "register", Value: ""}, nil
			case "w":
				return &MenuChoice{Action: "watch", Value: ""}, nil
			case "c":
				return &MenuChoice{Action: "credit", Value: ""}, nil
			case "q":
				return handleCtrlD(), nil
			default:
				// Invalid choice - use validator for consistent error message
				_, errorMsg := validator.ValidateInput(choice)
				if err := mh.handleInvalidInput(channel, errorMsg, banner); err != nil {
					if err == io.EOF {
						return handleCtrlD(), nil
					}
					return nil, err
				}
			}
		} else if event.Type == terminal.EventKey {
			switch event.KeyCode {
			case terminal.KeyCtrlC, terminal.KeyCtrlD:
				return handleCtrlD(), nil
			}
		}
	}
}

// ShowUserMenu displays the main menu for authenticated users and handles input
func (mh *MenuHandler) ShowUserMenu(ctx context.Context, channel ssh.Channel, user *authv1.User) (*MenuChoice, error) {
	// Check if user is admin to show appropriate menu
	if user != nil && user.IsAdmin {
		return mh.ShowAdminMenu(ctx, channel, user)
	}

	// Create input validator for user menu
	validator := &InputValidator{
		ValidOptions: []string{"[P]lay", "[W]atch", "[E]dit profile", "[L]ist games", "[R]ecordings", "[S]tatistics", "[C]redits", "[Q]uit"},
		MenuName:     "User Menu",
	}

	// Clear screen and position cursor at top
	if _, err := channel.Write([]byte("\033[2J\033[H")); err != nil {
		if err == io.EOF {
			return handleCtrlD(), nil
		}
	}

	// Render the user banner
	banner, err := mh.bannerManager.RenderMainUser(user.Username)
	if err != nil {
		mh.logger.Error("Failed to render user banner", "error", err, "username", user.Username)
		return nil, fmt.Errorf("failed to render banner: %w", err)
	}

	// Display the banner
	_, err = channel.Write([]byte(banner))
	if err != nil {
		if err == io.EOF {
			return handleCtrlD(), nil
		}
		return nil, fmt.Errorf("failed to write banner: %w", err)
	}

	// Create terminal input handler for proper keyboard support
	inputHandler := terminal.NewInputHandler(channel)

	// Wait for user input
	for {
		event, err := inputHandler.ReadInput(ctx)
		if err != nil {
			if err.Error() == "user cancelled" {
				return handleCtrlD(), nil
			}
			return nil, fmt.Errorf("failed to read user input: %w", err)
		}

		// Handle character input for menu choices
		if event.Type == terminal.EventCharacter {
			choice := string(event.Character)

			switch strings.ToLower(choice) {
			case "p":
				return &MenuChoice{Action: "play", Value: ""}, nil
			case "w":
				return &MenuChoice{Action: "watch", Value: ""}, nil
			case "e":
				return &MenuChoice{Action: "edit_profile", Value: ""}, nil
			case "r":
				return &MenuChoice{Action: "view_recordings", Value: ""}, nil
			case "s":
				return &MenuChoice{Action: "statistics", Value: ""}, nil
			case "c":
				return &MenuChoice{Action: "credit", Value: ""}, nil
			case "q":
				return handleCtrlD(), nil
			default:
				// Invalid choice - use validator for consistent error message
				_, errorMsg := validator.ValidateInput(choice)
				if err := mh.handleInvalidInput(channel, errorMsg, banner); err != nil {
					if err == io.EOF {
						return handleCtrlD(), nil
					}
					return nil, err
				}
			}
		} else if event.Type == terminal.EventKey {
			switch event.KeyCode {
			case terminal.KeyCtrlC, terminal.KeyCtrlD:
				return handleCtrlD(), nil
			}
		}
	}
}

// ShowAdminMenu displays the admin menu for admin users and handles input
func (mh *MenuHandler) ShowAdminMenu(ctx context.Context, channel ssh.Channel, user *authv1.User) (*MenuChoice, error) {
	// Create input validator for admin menu
	validator := &InputValidator{
		ValidOptions: []string{"[P]lay", "[W]atch", "[E]dit profile", "[V]iew recordings", "[G]ame Stats", "[U]nlock User", "[D]elete User", "[R]eset Password", "[A]dd Admin", "[S]erver Statistics", "[C]redits", "[Q]uit"},
		MenuName:     "Admin Menu",
	}

	// Clear screen and position cursor at top
	if _, err := channel.Write([]byte("\033[2J\033[H")); err != nil {
		if err == io.EOF {
			return handleCtrlD(), nil
		}
	}

	// Render the admin banner
	banner, err := mh.bannerManager.RenderMainAdmin(user.Username)
	if err != nil {
		mh.logger.Error("Failed to render admin banner", "error", err, "username", user.Username)
		return nil, fmt.Errorf("failed to render banner: %w", err)
	}

	// Display the banner
	_, err = channel.Write([]byte(banner))
	if err != nil {
		if err == io.EOF {
			return handleCtrlD(), nil
		}
		return nil, fmt.Errorf("failed to write banner: %w", err)
	}

	// Create terminal input handler for proper keyboard support
	inputHandler := terminal.NewInputHandler(channel)

	// Wait for user input
	for {
		event, err := inputHandler.ReadInput(ctx)
		if err != nil {
			if err.Error() == "user cancelled" {
				return handleCtrlD(), nil
			}
			return nil, fmt.Errorf("failed to read user input: %w", err)
		}

		// Handle character input for menu choices
		if event.Type == terminal.EventCharacter {
			choice := string(event.Character)

			switch strings.ToLower(choice) {
			// Regular user menu options (admins can use these too)
			case "p":
				return &MenuChoice{Action: "play", Value: ""}, nil
			case "w":
				return &MenuChoice{Action: "watch", Value: ""}, nil
			case "e":
				return &MenuChoice{Action: "edit_profile", Value: ""}, nil
			case "v":
				return &MenuChoice{Action: "view_recordings", Value: ""}, nil
			case "g":
				return &MenuChoice{Action: "statistics", Value: ""}, nil
			case "c":
				return &MenuChoice{Action: "credit", Value: ""}, nil
			// Admin-specific functions
			case "u":
				return &MenuChoice{Action: "admin_unlock_user", Value: ""}, nil
			case "d":
				return &MenuChoice{Action: "admin_delete_user", Value: ""}, nil
			case "r":
				return &MenuChoice{Action: "admin_reset_password", Value: ""}, nil
			case "a":
				return &MenuChoice{Action: "admin_promote_user", Value: ""}, nil
			case "s":
				return &MenuChoice{Action: "admin_server_stats", Value: ""}, nil
			case "q":
				return handleCtrlD(), nil
			default:
				// Invalid choice - use validator for consistent error message
				_, errorMsg := validator.ValidateInput(choice)
				if err := mh.handleInvalidInput(channel, errorMsg, banner); err != nil {
					if err == io.EOF {
						return handleCtrlD(), nil
					}
					return nil, err
				}
			}
		} else if event.Type == terminal.EventKey {
			switch event.KeyCode {
			case terminal.KeyCtrlC, terminal.KeyCtrlD:
				return handleCtrlD(), nil
			}
		}
	}
}

// RenderServiceUnavailable renders the service unavailable banner with countdown and service status
func (mh *MenuHandler) RenderServiceUnavailable(username string, remainingMinutes, remainingSeconds int, serviceStatus string) (string, error) {
	// Render the service unavailable banner with countdown and service status
	return mh.bannerManager.RenderServiceUnavailable(username, remainingMinutes, remainingSeconds, serviceStatus)
}

// ShowGameSelectionMenu displays the game selection menu and handles input
func (mh *MenuHandler) ShowGameSelectionMenu(ctx context.Context, channel ssh.Channel, username string) (*MenuChoice, error) {
	// Get list of available games from Game Service
	games, err := mh.gameClient.ListGames(ctx)
	if err != nil {
		mh.logger.Error("Failed to get available games", "error", err, "username", username)
		channel.Write([]byte("\r\nFailed to load available games. Please try again later.\r\n"))
		// Brief pause to let user read the message
		time.Sleep(2 * time.Second)
		return nil, nil
	}

	if len(games) == 0 {
		channel.Write([]byte("\r\nNo games are currently available.\r\n"))
		// Brief pause to let user read the message
		time.Sleep(2 * time.Second)
		return nil, nil
	}

	// Display game selection menu
	banner := mh.buildGameSelectionBanner(games, username)
	_, err = channel.Write([]byte(banner))
	if err != nil {
		if err == io.EOF {
			// Client disconnected gracefully
			return &MenuChoice{Action: "quit", Value: ""}, nil
		}
		return nil, fmt.Errorf("failed to write game selection banner: %w", err)
	}

	// Use proper input handler to avoid character echoing
	inputHandler := terminal.NewInputHandler(channel)
	var inputBuffer strings.Builder

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			event, err := inputHandler.ReadInput(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to read user input: %w", err)
			}

			switch event.Type {
			case terminal.EventCharacter:
				char := event.Character

				// Handle immediate single-character commands
				if char == 'q' || char == 'Q' || char == 'b' || char == 'B' {
					return nil, nil // Return to main menu
				}

				// For digits, accumulate input until Enter
				if char >= '0' && char <= '9' {
					inputBuffer.WriteRune(char)
					// Echo the character for visual feedback
					channel.Write([]byte(string(char)))
				}

			case terminal.EventKey:
				key := event.KeyCode

				// Handle Ctrl+D consistently
				if key == terminal.KeyCtrlD {
					return handleCtrlD(), nil
				}

				if key == terminal.KeyEnter {
					choice := strings.TrimSpace(inputBuffer.String())
					inputBuffer.Reset()

					if choice == "" {
						continue // Ignore empty input
					}

					// Try to parse game selection number
					if gameIndex, parseErr := parseGameChoice(choice, len(games)); parseErr == nil {
						selectedGame := games[gameIndex]
						// Clear the line to remove echoed input
						channel.Write([]byte("\r\n"))
						return &MenuChoice{
							Action: "start_game",
							Value:  selectedGame.Id,
						}, nil
					} else {
						// Invalid choice, show error with helpful options
						validOptions := fmt.Sprintf("1-%d", len(games))
						errorMsg := fmt.Sprintf("\r\nInvalid choice '%s'. Valid options: %s, or [Q]uit\r\n\r\n", choice, validOptions)
						channel.Write([]byte(errorMsg))
						// Redisplay the banner
						channel.Write([]byte(banner))
					}
				} else if key == terminal.KeyBackspace {
					// Handle backspace for multi-digit input
					if inputBuffer.Len() > 0 {
						// Remove last character from buffer
						str := inputBuffer.String()
						inputBuffer.Reset()
						inputBuffer.WriteString(str[:len(str)-1])
						// Send backspace sequence to terminal
						channel.Write([]byte("\b \b"))
					}
				}
			}
		}
	}
}

// buildGameSelectionBanner creates the game selection menu display with header and footer
func (mh *MenuHandler) buildGameSelectionBanner(games []*gamev2.Game, username string) string {
	// Get template variables for header/footer
	variables := mh.bannerManager.GetTemplateVariables(username)

	// Get header
	header := mh.bannerManager.RenderHeader("game_selection", variables)

	// Build main content
	banner := fmt.Sprintf("=== DungeonGate - Game Selection ===\r\n\r\n")
	banner += fmt.Sprintf("Welcome, %s! Choose a game to play:\r\n\r\n", username)

	for i, game := range games {
		status := "Available"
		if game.Status != gamev2.GameStatus_GAME_STATUS_UNSPECIFIED {
			status = fmt.Sprintf("Available (%s)", game.Status.String())
		}

		banner += fmt.Sprintf("  [%d] %s\r\n", i+1, game.Name)
		if game.Description != "" {
			banner += fmt.Sprintf("      %s\r\n", game.Description)
		}
		banner += fmt.Sprintf("      Status: %s\r\n", status)
		if game.Version != "" {
			banner += fmt.Sprintf("      Version: %s\r\n", game.Version)
		}
		banner += "\r\n"
	}

	banner += "  [q] Return to main menu\r\n\r\n"
	banner += "Enter your choice: "

	// Get footer
	footer := mh.bannerManager.RenderFooter("game_selection", variables)

	// Combine header + banner + footer
	result := header + banner + footer
	return result
}

// parseGameChoice parses the user's game selection input
func parseGameChoice(input string, maxGames int) (int, error) {
	// Check for decimal points or other invalid characters
	if strings.Contains(input, ".") || strings.Contains(input, ",") {
		return -1, fmt.Errorf("invalid input format: decimal numbers not allowed")
	}

	choice, err := fmt.Sscanf(input, "%d", new(int))
	if err != nil || choice != 1 {
		return -1, fmt.Errorf("invalid input format")
	}

	var gameIndex int
	fmt.Sscanf(input, "%d", &gameIndex)

	if gameIndex < 1 || gameIndex > maxGames {
		return -1, fmt.Errorf("choice out of range")
	}

	return gameIndex - 1, nil // Convert to 0-based index
}

// ShowSpectateMenu displays the formatted spectate menu with active game sessions and live updates
func (mh *MenuHandler) ShowSpectateMenu(ctx context.Context, channel ssh.Channel, user *authv1.User) (*MenuChoice, error) {
	// Get initial active sessions available for spectating
	sessions, err := mh.gameClient.GetActiveGameSessions(ctx)
	if err != nil {
		mh.logger.Error("Failed to get active sessions", "error", err)
		channel.Write([]byte("Failed to get active sessions. Please try again later.\r\n"))
		time.Sleep(2 * time.Second)
		return nil, nil
	}

	// Filter out user's own sessions (authenticated users only)
	availableSessions := mh.filterUserSessions(sessions, user)
	if availableSessions == nil {
		// Error already handled in filterUserSessions
		return nil, nil
	}

	if len(availableSessions) == 0 {
		// Clear screen and show informative message
		channel.Write([]byte("\033[2J\033[H"))

		if user == nil {
			// Message for anonymous users
			channel.Write([]byte("=== No Games Available to Watch ===\r\n\r\n"))
			channel.Write([]byte("There are currently no active game sessions to spectate.\r\n\r\n"))
			channel.Write([]byte("To have games to watch:\r\n"))
			channel.Write([]byte("• Login and start playing a game\r\n"))
			channel.Write([]byte("• Wait for other players to start games\r\n\r\n"))
			channel.Write([]byte("Press any key to return to the main menu...\r\n"))
		} else {
			// Message for authenticated users
			channel.Write([]byte("=== No Games Available to Watch ===\r\n\r\n"))
			channel.Write([]byte("There are currently no active game sessions to spectate.\r\n\r\n"))
			channel.Write([]byte("You can:\r\n"))
			channel.Write([]byte("• Start playing a game yourself\r\n"))
			channel.Write([]byte("• Wait for other players to start games\r\n\r\n"))
			channel.Write([]byte("Press any key to return to the main menu...\r\n"))
		}

		// Wait for any key press to return
		buffer := make([]byte, 1)
		channel.Read(buffer)
		return nil, nil
	}

	// Clear screen initially
	channel.Write([]byte("\033[2J\033[H"))

	// Create a ticker for updating the display every second
	updateTicker := time.NewTicker(1 * time.Second)
	defer updateTicker.Stop()

	// Create context for input handling with timeout
	inputCtx, inputCancel := context.WithCancel(ctx)
	defer inputCancel()

	// Channel for handling input events
	inputChan := make(chan *inputEvent, 10)
	errorChan := make(chan error, 1)

	// Start goroutine for handling input
	go mh.handleSpectateMenuInput(inputCtx, channel, inputChan, errorChan)

	// Initial display
	banner := mh.buildSpectateMenuBanner(availableSessions)
	if _, err := channel.Write([]byte(banner)); err != nil {
		if err == io.EOF {
			return &MenuChoice{Action: "quit", Value: ""}, nil
		}
		return nil, fmt.Errorf("failed to write watch menu banner: %w", err)
	}

	var inputBuffer strings.Builder
	lastUpdateTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()

		case err := <-errorChan:
			if err == io.EOF {
				return &MenuChoice{Action: "quit", Value: ""}, nil
			}
			return nil, err

		case event := <-inputChan:
			// Handle input event
			choice := mh.processInputEvent(event, &inputBuffer, availableSessions, channel)
			if choice != nil {
				return choice, nil
			}

		case <-updateTicker.C:
			// Update display every second
			now := time.Now()

			// Only refresh if it's been at least 1 second since last update
			if now.Sub(lastUpdateTime) >= time.Second {
				// Get fresh session data every 30 seconds or if session count might have changed
				if int(now.Unix())%30 == 0 {
					if freshSessions, err := mh.gameClient.GetActiveGameSessions(ctx); err == nil {
						newAvailableSessions := mh.filterUserSessions(freshSessions, user)
						if newAvailableSessions != nil {
							availableSessions = newAvailableSessions
						}
					}
				}

				// Rebuild and redisplay the banner with updated idle times
				newBanner := mh.buildSpectateMenuBanner(availableSessions)

				// Only update if the banner actually changed or if idle times need updating
				if newBanner != banner || mh.hasIdleTimeUpdates(availableSessions) {
					// Clear screen and redisplay
					channel.Write([]byte("\033[2J\033[H"))
					if _, err := channel.Write([]byte(newBanner)); err != nil {
						if err == io.EOF {
							return &MenuChoice{Action: "quit", Value: ""}, nil
						}
						// Don't return error for write failures during updates
						continue
					}

					// Restore any typed input
					if inputBuffer.Len() > 0 {
						channel.Write([]byte(inputBuffer.String()))
					}

					banner = newBanner
				}
				lastUpdateTime = now
			}
		}
	}
}

// inputEvent represents a user input event
type inputEvent struct {
	eventType terminal.InputEventType
	character rune
	keyCode   terminal.KeyCode
}

// handleSpectateMenuInput handles user input in a separate goroutine
func (mh *MenuHandler) handleSpectateMenuInput(ctx context.Context, channel ssh.Channel, inputChan chan<- *inputEvent, errorChan chan<- error) {
	inputHandler := terminal.NewInputHandler(channel)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			event, err := inputHandler.ReadInput(ctx)
			if err != nil {
				select {
				case errorChan <- err:
				case <-ctx.Done():
				}
				return
			}

			inputEvent := &inputEvent{
				eventType: event.Type,
				character: event.Character,
				keyCode:   event.KeyCode,
			}

			select {
			case inputChan <- inputEvent:
			case <-ctx.Done():
				return
			}
		}
	}
}

// processInputEvent processes a single input event and returns a menu choice if selection is made
func (mh *MenuHandler) processInputEvent(event *inputEvent, inputBuffer *strings.Builder, availableSessions []*gamev2.GameSession, channel ssh.Channel) *MenuChoice {
	switch event.eventType {
	case terminal.EventCharacter:
		char := event.character

		// Handle immediate single-character commands
		if char == 'q' || char == 'Q' {
			return nil // Return to main menu
		}

		// Handle help
		if char == '?' {
			mh.showSpectateHelp(channel)
			return nil // Continue showing menu
		}

		// Handle random selection
		if char == '*' {
			if len(availableSessions) > 0 {
				// Select a random session
				randomIndex := int(time.Now().UnixNano()) % len(availableSessions)
				selectedSession := availableSessions[randomIndex]
				channel.Write([]byte(fmt.Sprintf("\r\nRandomly selected: %s\r\n", selectedSession.Username)))
				return &MenuChoice{
					Action: "spectate_session",
					Value:  selectedSession.Id,
				}
			}
		}

		// Handle pagination (placeholder for now)
		if char == '>' {
			// TODO: Next page
			channel.Write([]byte("\r\nNext page not yet implemented.\r\n"))
			return nil
		}
		if char == '<' {
			// TODO: Previous page
			channel.Write([]byte("\r\nPrevious page not yet implemented.\r\n"))
			return nil
		}

		// Handle sorting (placeholder for now)
		if char == '.' || char == ',' {
			// TODO: Change sorting method
			channel.Write([]byte("\r\nSorting method change not yet implemented.\r\n"))
			return nil
		}

		// For letters a-z and A-Z, handle session selection
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') {
			var sessionIndex int
			if char >= 'a' && char <= 'z' {
				sessionIndex = int(char - 'a')
			} else {
				sessionIndex = int(char-'A') + 26 // A-Z maps to sessions 26-51
			}

			if sessionIndex < len(availableSessions) {
				selectedSession := availableSessions[sessionIndex]
				// Clear the line to remove echoed input
				channel.Write([]byte("\r\n"))
				return &MenuChoice{
					Action: "spectate_session",
					Value:  selectedSession.Id,
				}
			} else {
				// Invalid session selection
				var maxLetter rune
				if len(availableSessions) <= 26 {
					maxLetter = 'a' + rune(len(availableSessions)-1)
				} else {
					maxLetter = 'Z'
				}
				errorMsg := fmt.Sprintf("\r\nInvalid choice '%c'. Valid options: a-%c, '?' for help, or 'q' to quit\r\n\r\n",
					char, maxLetter)
				channel.Write([]byte(errorMsg))
			}
		}

		// For digits, accumulate input until Enter (for numbered selection)
		if char >= '0' && char <= '9' {
			inputBuffer.WriteRune(char)
			// Echo the character for visual feedback
			channel.Write([]byte(string(char)))
		}

	case terminal.EventKey:
		key := event.keyCode

		// Handle Ctrl+D consistently
		if key == terminal.KeyCtrlD {
			return &MenuChoice{Action: "quit", Value: ""}
		}

		if key == terminal.KeyEnter {
			choice := strings.TrimSpace(inputBuffer.String())
			inputBuffer.Reset()

			if choice == "" {
				return nil // Ignore empty input
			}

			// Try to parse session selection number (1-based)
			if sessionIndex, parseErr := parseGameChoice(choice, len(availableSessions)); parseErr == nil {
				selectedSession := availableSessions[sessionIndex]
				// Clear the line to remove echoed input
				channel.Write([]byte("\r\n"))
				return &MenuChoice{
					Action: "spectate_session",
					Value:  selectedSession.Id,
				}
			} else {
				// Invalid choice, show error with helpful options
				validLetters := fmt.Sprintf("a-%c", 'a'+rune(len(availableSessions)-1))
				validNumbers := fmt.Sprintf("1-%d", len(availableSessions))
				errorMsg := fmt.Sprintf("\r\nInvalid choice '%s'. Valid options: %s, %s, '?' for help, or 'q' to quit\r\n\r\n",
					choice, validLetters, validNumbers)
				channel.Write([]byte(errorMsg))
			}
		} else if key == terminal.KeyBackspace {
			// Handle backspace for multi-digit input
			if inputBuffer.Len() > 0 {
				// Remove last character from buffer
				str := inputBuffer.String()
				inputBuffer.Reset()
				inputBuffer.WriteString(str[:len(str)-1])
				// Send backspace sequence to terminal
				channel.Write([]byte("\b \b"))
			}
		}
	}

	return nil // Continue showing menu
}

// filterUserSessions filters out user's own sessions for authenticated users
func (mh *MenuHandler) filterUserSessions(sessions []*gamev2.GameSession, user *authv1.User) []*gamev2.GameSession {
	availableSessions := make([]*gamev2.GameSession, 0)

	if user != nil {
		// For authenticated users, filter out their own sessions
		userID, err := strconv.ParseInt(user.Id, 10, 32)
		if err != nil {
			mh.logger.Error("Invalid user ID format", "user_id", user.Id, "error", err)
			return nil
		}

		for _, session := range sessions {
			if session.UserId != int32(userID) {
				availableSessions = append(availableSessions, session)
			}
		}
	} else {
		// For anonymous users, show all sessions
		availableSessions = sessions
	}

	return availableSessions
}

// hasIdleTimeUpdates checks if any sessions have idle times that would change
func (mh *MenuHandler) hasIdleTimeUpdates(sessions []*gamev2.GameSession) bool {
	now := time.Now()
	for _, session := range sessions {
		if session.LastActivity != nil {
			lastActivity := session.LastActivity.AsTime()
			duration := now.Sub(lastActivity)
			// If the idle time is actively counting (30s+), then we have updates
			if duration >= 30*time.Second {
				return true
			}
		}
	}
	return false
}

// buildSpectateMenuBanner creates the formatted spectate menu display
func (mh *MenuHandler) buildSpectateMenuBanner(sessions []*gamev2.GameSession) string {
	var banner strings.Builder

	banner.WriteString("The following games are in progress:\r\n\r\n")

	// Header
	banner.WriteString("    Username         Game    Size    Start date & time    Idle time   Watchers\r\n")

	// Session entries
	for i, session := range sessions {
		// Convert session data to display format (a-z, then A-Z)
		var letter string
		if i < 26 {
			letter = string('a' + rune(i))
		} else {
			letter = string('A' + rune(i-26))
		}
		username := session.Username
		if len(username) > 15 {
			username = username[:12] + "..."
		}

		// Game ID formatting (convert "nethack" to "NH370" format)
		gameDisplay := mh.formatGameDisplay(session.GameId)

		// Terminal size
		size := "80x24" // Default
		if session.TerminalSize != nil {
			size = fmt.Sprintf("%dx%d", session.TerminalSize.Width, session.TerminalSize.Height)
		}

		// Start time formatting
		startTime := "Unknown"
		if session.StartTime != nil {
			startTime = session.StartTime.AsTime().Format("2006-01-02 15:04:05")
		}

		// Calculate idle time
		idleTime := mh.calculateIdleTime(session)

		// Spectator count
		spectatorCount := len(session.Spectators)

		// Format the line with proper spacing
		banner.WriteString(fmt.Sprintf(" %s) %-15s %-7s %-8s %-19s %-11s %d\r\n",
			letter, username, gameDisplay, size, startTime, idleTime, spectatorCount))
	}

	// Footer with pagination info and prompt
	banner.WriteString(fmt.Sprintf("\r\n (1-%d of %d)\r\n\r\n", len(sessions), len(sessions)))
	banner.WriteString(" Spectate which game? ('?' for help) => ")

	return banner.String()
}

// formatGameDisplay converts game IDs to display format
func (mh *MenuHandler) formatGameDisplay(gameID string) string {
	switch strings.ToLower(gameID) {
	case "nethack":
		return "NH370"
	case "dcss", "crawl":
		return "DCSS"
	case "angband":
		return "ANG"
	case "tome":
		return "TOME"
	default:
		// For unknown games, take first 5 characters and uppercase
		if len(gameID) <= 5 {
			return strings.ToUpper(gameID)
		}
		return strings.ToUpper(gameID[:5])
	}
}

// calculateIdleTime calculates how long since last activity
func (mh *MenuHandler) calculateIdleTime(session *gamev2.GameSession) string {
	if session.LastActivity == nil {
		return ""
	}

	now := time.Now()
	lastActivity := session.LastActivity.AsTime()
	duration := now.Sub(lastActivity)

	// If less than 30 seconds, consider not idle
	if duration < 30*time.Second {
		return ""
	}

	// Format duration in a human-readable way
	if duration < time.Minute {
		return fmt.Sprintf("%ds", int(duration.Seconds()))
	} else if duration < time.Hour {
		minutes := int(duration.Minutes())
		seconds := int(duration.Seconds()) % 60
		if seconds == 0 {
			return fmt.Sprintf("%dm", minutes)
		}
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	} else {
		hours := int(duration.Hours())
		minutes := int(duration.Minutes()) % 60
		if minutes == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
}

// showSpectateHelp displays help information for the spectate menu
func (mh *MenuHandler) showSpectateHelp(channel ssh.Channel) {
	help := "\r\n  Help for watching-menu\r\n"
	help += "  ----------------------\r\n"
	help += "  ?        show this help.\r\n"
	help += "  > <      next/previous page.\r\n"
	help += "  . ,      change sorting method.\r\n"
	help += "  q Q      return to main menu.\r\n"
	help += "  a-zA-Z   select a game to watch.\r\n"
	help += "  *        start showing a randomly selected game.\r\n"
	help += "  enter    start watching already selected game.\r\n"
	help += "\r\n\r\n"
	help += "  While watching a game\r\n"
	help += "  ---------------------\r\n"
	help += "  q        return back to the watching menu.\r\n"
	help += "  m        send mail to the player (requires login).\r\n"
	help += "  s        toggle charset stripping between DEC/IBM/none.\r\n"
	help += "  r        resize your terminal to match the player's terminal.\r\n"
	help += "\r\n\r\n"
	help += "Press any key to continue...\r\n"

	channel.Write([]byte(help))

	// Wait for any key press
	buffer := make([]byte, 1)
	channel.Read(buffer)

	// Clear screen and position cursor
	channel.Write([]byte("\033[2J\033[H"))
}
