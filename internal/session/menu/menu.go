package menu

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/dungeongate/internal/session/banner"
	"github.com/dungeongate/internal/session/client"
	"github.com/dungeongate/internal/session/terminal"
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
func (mh *MenuHandler) ShowUserMenu(ctx context.Context, channel ssh.Channel, username string) (*MenuChoice, error) {
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
	banner, err := mh.bannerManager.RenderMainUser(username)
	if err != nil {
		mh.logger.Error("Failed to render user banner", "error", err, "username", username)
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
