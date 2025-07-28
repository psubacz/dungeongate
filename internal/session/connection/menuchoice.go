package connection

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
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

	// Admin Functions
	case "admin_unlock_user":
		return p.handleAdminUnlockUser(ctx, channel, userInfo, sshConn)

	case "admin_delete_user":
		return p.handleAdminDeleteUser(ctx, channel, userInfo, sshConn)

	case "admin_reset_password":
		return p.handleAdminResetPassword(ctx, channel, userInfo, sshConn)

	case "admin_promote_user":
		return p.handleAdminPromoteUser(ctx, channel, userInfo, sshConn)

	case "admin_server_stats":
		return p.handleAdminServerStats(ctx, channel, userInfo, sshConn)

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

// Admin Functions Implementation

// promptForUsername prompts for and reads a username from the user
func (p *MenuChoiceProcessor) promptForUsername(ctx context.Context, channel ssh.Channel, prompt string) (string, error) {
	channel.Write([]byte(fmt.Sprintf("%s: ", prompt)))

	buffer := make([]byte, 256)
	var input strings.Builder

	for {
		n, err := channel.Read(buffer[:1])
		if err != nil {
			return "", err
		}
		if n == 0 {
			continue
		}

		char := buffer[0]

		// Handle Enter key
		if char == '\r' || char == '\n' {
			channel.Write([]byte("\r\n"))
			return strings.TrimSpace(input.String()), nil
		}

		// Handle Backspace
		if char == 127 || char == 8 {
			if input.Len() > 0 {
				str := input.String()
				input.Reset()
				input.WriteString(str[:len(str)-1])
				channel.Write([]byte("\b \b"))
			}
			continue
		}

		// Handle Ctrl+C or Ctrl+D
		if char == 3 || char == 4 {
			return "", fmt.Errorf("user cancelled")
		}

		// Handle printable characters
		if char >= 32 && char < 127 {
			input.WriteByte(char)
			channel.Write([]byte{char})
		}
	}
}

// promptForPassword prompts for and reads a password from the user with asterisk masking
func (p *MenuChoiceProcessor) promptForPassword(ctx context.Context, channel ssh.Channel, prompt string) (string, error) {
	channel.Write([]byte(fmt.Sprintf("%s: ", prompt)))

	buffer := make([]byte, 256)
	var input strings.Builder

	for {
		n, err := channel.Read(buffer[:1])
		if err != nil {
			return "", err
		}
		if n == 0 {
			continue
		}

		char := buffer[0]

		// Handle Enter key
		if char == '\r' || char == '\n' {
			channel.Write([]byte("\r\n"))
			return strings.TrimSpace(input.String()), nil
		}

		// Handle Backspace
		if char == 127 || char == 8 {
			if input.Len() > 0 {
				str := input.String()
				input.Reset()
				input.WriteString(str[:len(str)-1])
				// Send backspace sequence to remove the asterisk
				channel.Write([]byte("\b \b"))
			}
			continue
		}

		// Handle Ctrl+C or Ctrl+D
		if char == 3 || char == 4 {
			return "", fmt.Errorf("user cancelled")
		}

		// Handle printable characters and show asterisk
		if char >= 32 && char < 127 {
			input.WriteByte(char)
			// Echo asterisk instead of the actual character
			channel.Write([]byte("*"))
		}
	}
}

// getAdminToken extracts the JWT access token from the SSH connection
func (p *MenuChoiceProcessor) getAdminToken(sshConn *ssh.ServerConn) string {
	if sshConn == nil || sshConn.Permissions == nil || sshConn.Permissions.Extensions == nil {
		return ""
	}
	
	accessToken, ok := sshConn.Permissions.Extensions["access_token"]
	if !ok {
		return ""
	}
	
	return accessToken
}

// handleAdminUnlockUser handles unlocking a user account
func (p *MenuChoiceProcessor) handleAdminUnlockUser(ctx context.Context, channel ssh.Channel, userInfo *authv1.User, sshConn *ssh.ServerConn) error {
	if userInfo == nil || !userInfo.IsAdmin {
		channel.Write([]byte("Access denied: Admin privileges required.\r\n"))
		time.Sleep(2 * time.Second)
		return nil
	}

	channel.Write([]byte("\033[2J\033[H")) // Clear screen
	channel.Write([]byte("=== Unlock User Account ===\r\n\r\n"))

	targetUsername, err := p.promptForUsername(ctx, channel, "Enter username to unlock")
	if err != nil {
		if err.Error() == "user cancelled" {
			return nil
		}
		return err
	}

	if targetUsername == "" {
		channel.Write([]byte("Username cannot be empty.\r\n"))
		time.Sleep(2 * time.Second)
		return nil
	}

	adminToken := p.getAdminToken(sshConn)
	if adminToken == "" {
		channel.Write([]byte("Error: Unable to get admin authentication token.\r\n"))
		time.Sleep(3 * time.Second)
		return nil
	}

	resp, err := p.authManager.authClient.UnlockUserAccount(ctx, adminToken, targetUsername)
	if err != nil {
		p.logger.Error("Failed to unlock user account", "error", err, "admin", userInfo.Username, "target", targetUsername)
		channel.Write([]byte(fmt.Sprintf("Error: %v\r\n", err)))
		time.Sleep(3 * time.Second)
		return nil
	}

	if resp.Success {
		channel.Write([]byte(fmt.Sprintf("✓ Successfully unlocked account: %s\r\n", targetUsername)))
		if resp.Message != "" {
			channel.Write([]byte(fmt.Sprintf("Message: %s\r\n", resp.Message)))
		}
		p.logger.Info("Admin unlocked user account", "admin", userInfo.Username, "target", targetUsername)
	} else {
		channel.Write([]byte(fmt.Sprintf("✗ Failed to unlock account: %s\r\n", resp.Error)))
	}

	channel.Write([]byte("\r\nPress any key to continue..."))
	buffer := make([]byte, 1)
	channel.Read(buffer)
	return nil
}

// handleAdminDeleteUser handles deleting a user account
func (p *MenuChoiceProcessor) handleAdminDeleteUser(ctx context.Context, channel ssh.Channel, userInfo *authv1.User, sshConn *ssh.ServerConn) error {
	if userInfo == nil || !userInfo.IsAdmin {
		channel.Write([]byte("Access denied: Admin privileges required.\r\n"))
		time.Sleep(2 * time.Second)
		return nil
	}

	channel.Write([]byte("\033[2J\033[H")) // Clear screen
	channel.Write([]byte("=== Delete User Account ===\r\n\r\n"))
	channel.Write([]byte("⚠️  WARNING: This action is IRREVERSIBLE! ⚠️\r\n\r\n"))

	targetUsername, err := p.promptForUsername(ctx, channel, "Enter username to delete")
	if err != nil {
		if err.Error() == "user cancelled" {
			return nil
		}
		return err
	}

	if targetUsername == "" {
		channel.Write([]byte("Username cannot be empty.\r\n"))
		time.Sleep(2 * time.Second)
		return nil
	}

	// Confirmation prompt
	channel.Write([]byte(fmt.Sprintf("Are you sure you want to delete user '%s'? This cannot be undone!\r\n", targetUsername)))
	confirmation, err := p.promptForUsername(ctx, channel, "Type 'DELETE' to confirm")
	if err != nil {
		if err.Error() == "user cancelled" {
			return nil
		}
		return err
	}

	if confirmation != "DELETE" {
		channel.Write([]byte("Deletion cancelled.\r\n"))
		time.Sleep(2 * time.Second)
		return nil
	}

	adminToken := p.getAdminToken(sshConn)
	if adminToken == "" {
		channel.Write([]byte("Error: Unable to get admin authentication token.\r\n"))
		time.Sleep(3 * time.Second)
		return nil
	}

	resp, err := p.authManager.authClient.DeleteUserAccount(ctx, adminToken, targetUsername)
	if err != nil {
		p.logger.Error("Failed to delete user account", "error", err, "admin", userInfo.Username, "target", targetUsername)
		channel.Write([]byte(fmt.Sprintf("Error: %v\r\n", err)))
		time.Sleep(3 * time.Second)
		return nil
	}

	if resp.Success {
		channel.Write([]byte(fmt.Sprintf("✓ Successfully deleted account: %s\r\n", targetUsername)))
		if resp.Message != "" {
			channel.Write([]byte(fmt.Sprintf("Message: %s\r\n", resp.Message)))
		}
		p.logger.Warn("Admin deleted user account", "admin", userInfo.Username, "target", targetUsername)
	} else {
		channel.Write([]byte(fmt.Sprintf("✗ Failed to delete account: %s\r\n", resp.Error)))
	}

	channel.Write([]byte("\r\nPress any key to continue..."))
	buffer := make([]byte, 1)
	channel.Read(buffer)
	return nil
}

// handleAdminResetPassword handles resetting a user's password
func (p *MenuChoiceProcessor) handleAdminResetPassword(ctx context.Context, channel ssh.Channel, userInfo *authv1.User, sshConn *ssh.ServerConn) error {
	if userInfo == nil || !userInfo.IsAdmin {
		channel.Write([]byte("Access denied: Admin privileges required.\r\n"))
		time.Sleep(2 * time.Second)
		return nil
	}

	channel.Write([]byte("\033[2J\033[H")) // Clear screen
	channel.Write([]byte("=== Reset User Password ===\r\n\r\n"))

	targetUsername, err := p.promptForUsername(ctx, channel, "Enter username")
	if err != nil {
		if err.Error() == "user cancelled" {
			return nil
		}
		return err
	}

	if targetUsername == "" {
		channel.Write([]byte("Username cannot be empty.\r\n"))
		time.Sleep(2 * time.Second)
		return nil
	}

	newPassword, err := p.promptForPassword(ctx, channel, "Enter new password")
	if err != nil {
		if err.Error() == "user cancelled" {
			return nil
		}
		return err
	}

	if len(newPassword) < 8 {
		channel.Write([]byte("Password must be at least 8 characters long.\r\n"))
		time.Sleep(2 * time.Second)
		return nil
	}

	adminToken := p.getAdminToken(sshConn)
	if adminToken == "" {
		channel.Write([]byte("Error: Unable to get admin authentication token.\r\n"))
		time.Sleep(3 * time.Second)
		return nil
	}

	resp, err := p.authManager.authClient.ResetUserPassword(ctx, adminToken, targetUsername, newPassword)
	if err != nil {
		p.logger.Error("Failed to reset user password", "error", err, "admin", userInfo.Username, "target", targetUsername)
		channel.Write([]byte(fmt.Sprintf("Error: %v\r\n", err)))
		time.Sleep(3 * time.Second)
		return nil
	}

	if resp.Success {
		channel.Write([]byte(fmt.Sprintf("✓ Successfully reset password for: %s\r\n", targetUsername)))
		if resp.Message != "" {
			channel.Write([]byte(fmt.Sprintf("Message: %s\r\n", resp.Message)))
		}
		p.logger.Info("Admin reset user password", "admin", userInfo.Username, "target", targetUsername)
	} else {
		channel.Write([]byte(fmt.Sprintf("✗ Failed to reset password: %s\r\n", resp.Error)))
	}

	channel.Write([]byte("\r\nPress any key to continue..."))
	buffer := make([]byte, 1)
	channel.Read(buffer)
	return nil
}

// handleAdminPromoteUser handles promoting a user to admin
func (p *MenuChoiceProcessor) handleAdminPromoteUser(ctx context.Context, channel ssh.Channel, userInfo *authv1.User, sshConn *ssh.ServerConn) error {
	if userInfo == nil || !userInfo.IsAdmin {
		channel.Write([]byte("Access denied: Admin privileges required.\r\n"))
		time.Sleep(2 * time.Second)
		return nil
	}

	channel.Write([]byte("\033[2J\033[H")) // Clear screen
	channel.Write([]byte("=== Add Admin Privileges ===\r\n\r\n"))

	targetUsername, err := p.promptForUsername(ctx, channel, "Enter username to promote")
	if err != nil {
		if err.Error() == "user cancelled" {
			return nil
		}
		return err
	}

	if targetUsername == "" {
		channel.Write([]byte("Username cannot be empty.\r\n"))
		time.Sleep(2 * time.Second)
		return nil
	}

	adminToken := p.getAdminToken(sshConn)
	if adminToken == "" {
		channel.Write([]byte("Error: Unable to get admin authentication token.\r\n"))
		time.Sleep(3 * time.Second)
		return nil
	}

	resp, err := p.authManager.authClient.PromoteUserToAdmin(ctx, adminToken, targetUsername)
	if err != nil {
		p.logger.Error("Failed to promote user to admin", "error", err, "admin", userInfo.Username, "target", targetUsername)
		channel.Write([]byte(fmt.Sprintf("Error: %v\r\n", err)))
		time.Sleep(3 * time.Second)
		return nil
	}

	if resp.Success {
		channel.Write([]byte(fmt.Sprintf("✓ Successfully promoted user to admin: %s\r\n", targetUsername)))
		if resp.Message != "" {
			channel.Write([]byte(fmt.Sprintf("Message: %s\r\n", resp.Message)))
		}
		p.logger.Info("Admin promoted user to admin", "admin", userInfo.Username, "target", targetUsername)
	} else {
		channel.Write([]byte(fmt.Sprintf("✗ Failed to promote user: %s\r\n", resp.Error)))
	}

	channel.Write([]byte("\r\nPress any key to continue..."))
	buffer := make([]byte, 1)
	channel.Read(buffer)
	return nil
}

// handleAdminServerStats handles displaying server statistics
func (p *MenuChoiceProcessor) handleAdminServerStats(ctx context.Context, channel ssh.Channel, userInfo *authv1.User, sshConn *ssh.ServerConn) error {
	if userInfo == nil || !userInfo.IsAdmin {
		channel.Write([]byte("Access denied: Admin privileges required.\r\n"))
		time.Sleep(2 * time.Second)
		return nil
	}

	channel.Write([]byte("\033[2J\033[H")) // Clear screen
	channel.Write([]byte("=== Server Statistics ===\r\n\r\n"))

	adminToken := p.getAdminToken(sshConn)
	if adminToken == "" {
		channel.Write([]byte("Error: Unable to get admin authentication token.\r\n"))
		time.Sleep(3 * time.Second)
		return nil
	}

	resp, err := p.authManager.authClient.GetServerStatistics(ctx, adminToken)
	if err != nil {
		p.logger.Error("Failed to get server statistics", "error", err, "admin", userInfo.Username)
		channel.Write([]byte(fmt.Sprintf("Error: %v\r\n", err)))
		time.Sleep(3 * time.Second)
		return nil
	}

	if resp.Success {
		channel.Write([]byte("Server Statistics:\r\n"))
		channel.Write([]byte("==================\r\n\r\n"))

		for key, value := range resp.Stats {
			channel.Write([]byte(fmt.Sprintf("%-25s: %s\r\n", key, value)))
		}

		p.logger.Info("Admin viewed server statistics", "admin", userInfo.Username)
	} else {
		channel.Write([]byte(fmt.Sprintf("✗ Failed to get statistics: %s\r\n", resp.Error)))
	}

	channel.Write([]byte("\r\nPress any key to continue..."))
	buffer := make([]byte, 1)
	channel.Read(buffer)
	return nil
}
