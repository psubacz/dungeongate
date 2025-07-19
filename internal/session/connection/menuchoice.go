package connection

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/dungeongate/internal/session/menu"
	authv1 "github.com/dungeongate/pkg/api/auth/v1"
	"golang.org/x/crypto/ssh"
)

// MenuChoiceProcessor handles processing of user menu choices
type MenuChoiceProcessor struct {
	authManager       *UserAuthManager
	gameIOHandler     *GameIOHandler
	spectatingHandler *SpectatingHandler
	menuHandler       *menu.MenuHandler
	logger            *slog.Logger
}

// NewMenuChoiceProcessor creates a new menu choice processor
func NewMenuChoiceProcessor(
	authManager *UserAuthManager,
	gameIOHandler *GameIOHandler,
	spectatingHandler *SpectatingHandler,
	menuHandler *menu.MenuHandler,
	logger *slog.Logger,
) *MenuChoiceProcessor {
	return &MenuChoiceProcessor{
		authManager:       authManager,
		gameIOHandler:     gameIOHandler,
		spectatingHandler: spectatingHandler,
		menuHandler:       menuHandler,
		logger:            logger,
	}
}

// HandleMenuChoice handles the user's menu choice
func (p *MenuChoiceProcessor) HandleMenuChoice(
	ctx context.Context,
	channel ssh.Channel,
	choice *menu.MenuChoice,
	userInfo *authv1.User,
	connID, username string,
	terminalCols, terminalRows int,
	sshConn *ssh.ServerConn,
) error {
	switch choice.Action {
	case "quit":
		channel.Write([]byte("Goodbye!\r\n"))
		return fmt.Errorf("user quit") // This will exit the session

	case "play":
		// Show game selection menu for authenticated users
		if userInfo != nil {
			return p.handleGameSelection(ctx, channel, userInfo, connID, username, terminalCols, terminalRows, sshConn)
		} else {
			channel.Write([]byte("Please login first to play games.\r\n"))
			// Brief pause to let user read the message
			time.Sleep(2 * time.Second)
			return nil
		}

	case "login":
		return p.authManager.HandleLogin(ctx, channel, connID, username, sshConn)

	case "register":
		for {
			err := p.authManager.HandleRegister(ctx, channel, connID, username, sshConn)
			if err != nil && err.Error() == "retry_register" {
				// User chose to retry registration, loop back
				continue
			}
			// Either success (nil), user quit, or other error - return
			return err
		}

	case "start_game":
		// Start a specific game session with the selected game ID
		if userInfo != nil {
			return p.gameIOHandler.StartSpecificGameSession(ctx, channel, userInfo, connID, username, choice.Value, terminalCols, terminalRows)
		} else {
			channel.Write([]byte("Please login first to play games.\r\n"))
			// Brief pause to let user read the message
			time.Sleep(2 * time.Second)
			return nil
		}

	case "spectate_session":
		// Start spectating a specific game session
		return p.spectatingHandler.StartSpectating(ctx, channel, userInfo, choice.Value)

	case "watch":
		// Show the new formatted spectate menu
		spectateChoice, err := p.menuHandler.ShowSpectateMenu(ctx, channel, userInfo)
		if err != nil {
			p.logger.Error("Spectate menu failed", "error", err)
			return err
		}

		// If no choice was made (user quit), return to main menu
		if spectateChoice == nil {
			return nil
		}

		// Handle the spectate menu choice
		return p.HandleMenuChoice(ctx, channel, spectateChoice, userInfo, connID, username, terminalCols, terminalRows, sshConn)

	case "edit_profile":
		channel.Write([]byte("Profile editing functionality not yet implemented.\r\n"))
		// Brief pause to let user read the message
		time.Sleep(2 * time.Second)
		return nil

	case "view_recordings":
		channel.Write([]byte("Recording viewing functionality not yet implemented.\r\n"))
		// Brief pause to let user read the message
		time.Sleep(2 * time.Second)
		return nil

	case "statistics":
		channel.Write([]byte("Statistics functionality not yet implemented.\r\n"))
		// Brief pause to let user read the message
		time.Sleep(2 * time.Second)
		return nil

	case "credit":
		// Clear screen and show credits with ASCII art
		channel.Write([]byte("\033[2J\033[H"))
		channel.Write([]byte("\r\n"))

		// DungeonGate ASCII Art
		channel.Write([]byte(" ____\r\n"))
		channel.Write([]byte("|  _ \\ _   _ _ __   __ _  ___  ___  _ __\r\n"))
		channel.Write([]byte("| | | | | | | ._ \\ / _. |/ _ \\/ _ \\| ._ \\\r\n"))
		channel.Write([]byte("| |_| | |_| | | | | (_| |  __/ (_) | | | |\r\n"))
		channel.Write([]byte("|____/ \\__,_|_| |_|\\__, |\\___|\\____| |_| |\r\n"))
		channel.Write([]byte("        ___        |___/\r\n"))
		channel.Write([]byte("       / __|  __ _| |_ ___\r\n"))
		channel.Write([]byte("      | |___ / _. | __/ _ \\\r\n"))
		channel.Write([]byte("      | |__ | (_| |  ||  _/\r\n"))
		channel.Write([]byte("      |____/ \\__,_|\\__\\___|\r\n"))
		channel.Write([]byte("\r\n"))

		// Credits information
		channel.Write([]byte("=== Credits ===\r\n\r\n"))
		channel.Write([]byte("DungeonGate - Terminal Game Platform\r\n"))
		channel.Write([]byte("Developed with <3 and Claude Code\r\n\r\n"))
		channel.Write([]byte("Directed by Peter Subacz \r\n\r\n"))
		channel.Write([]byte("Press any key to return to menu...\r\n"))

		// Wait for any key press to return
		buffer := make([]byte, 1)
		channel.Read(buffer)
		return nil

	default:
		channel.Write([]byte(fmt.Sprintf("Unknown action: %s\r\n", choice.Action)))
		// Brief pause to let user read the message
		time.Sleep(2 * time.Second)
		return nil
	}
}

// handleGameSelection shows the game selection menu and handles the choice
func (p *MenuChoiceProcessor) handleGameSelection(ctx context.Context, channel ssh.Channel, userInfo *authv1.User, connID, username string, terminalCols, terminalRows int, sshConn *ssh.ServerConn) error {
	choice, err := p.menuHandler.ShowGameSelectionMenu(ctx, channel, userInfo.Username)
	if err != nil {
		p.logger.Error("Game selection menu failed", "error", err, "username", username)
		channel.Write([]byte("Failed to display game selection menu.\r\n"))
		return nil
	}

	// If choice is nil, user chose to go back to main menu
	if choice == nil {
		return nil
	}

	// Handle the game selection choice
	return p.HandleMenuChoice(ctx, channel, choice, userInfo, connID, username, terminalCols, terminalRows, sshConn)
}