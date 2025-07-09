package menu

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/dungeongate/internal/session/banner"
	"github.com/dungeongate/internal/session/client"
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

// ShowAnonymousMenu displays the main menu for anonymous users and handles input
func (mh *MenuHandler) ShowAnonymousMenu(ctx context.Context, channel ssh.Channel, username string) (*MenuChoice, error) {
	// Render the anonymous banner
	banner, err := mh.bannerManager.RenderMainAnon()
	if err != nil {
		mh.logger.Error("Failed to render anonymous banner", "error", err)
		// Fallback to simple banner
		banner = mh.getFallbackAnonymousBanner()
	}

	// Display the banner
	_, err = channel.Write([]byte(banner))
	if err != nil {
		return nil, fmt.Errorf("failed to write banner: %w", err)
	}

	// Wait for user input
	buffer := make([]byte, 1)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			// Note: SSH channels don't support SetReadDeadline, 
			// so we'll use context timeout instead for cancellation

			n, err := channel.Read(buffer)
			if err != nil {
				return nil, fmt.Errorf("failed to read user input: %w", err)
			}

			if n > 0 {
				choice := string(buffer[:n])
				choice = string(choice[0]) // Take first character only

				switch choice {
				case "l", "L":
					return &MenuChoice{Action: "login", Value: ""}, nil
				case "r", "R":
					return &MenuChoice{Action: "register", Value: ""}, nil
				case "w", "W":
					return &MenuChoice{Action: "watch", Value: ""}, nil
				case "g", "G":
					return &MenuChoice{Action: "list_games", Value: ""}, nil
				case "q", "Q":
					return &MenuChoice{Action: "quit", Value: ""}, nil
				default:
					// Invalid choice, show error and redisplay menu
					errorMsg := fmt.Sprintf("\r\nInvalid choice '%s'. Please try again.\r\n\r\n", choice)
					channel.Write([]byte(errorMsg))
					// Redisplay the banner
					channel.Write([]byte(banner))
				}
			}
		}
	}
}

// ShowUserMenu displays the main menu for authenticated users and handles input
func (mh *MenuHandler) ShowUserMenu(ctx context.Context, channel ssh.Channel, username string) (*MenuChoice, error) {
	// Render the user banner
	banner, err := mh.bannerManager.RenderMainUser(username)
	if err != nil {
		mh.logger.Error("Failed to render user banner", "error", err, "username", username)
		// Fallback to simple banner
		banner = mh.getFallbackUserBanner(username)
	}

	// Display the banner
	_, err = channel.Write([]byte(banner))
	if err != nil {
		return nil, fmt.Errorf("failed to write banner: %w", err)
	}

	// Wait for user input
	buffer := make([]byte, 1)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			// Note: SSH channels don't support SetReadDeadline, 
			// so we'll use context timeout instead for cancellation

			n, err := channel.Read(buffer)
			if err != nil {
				return nil, fmt.Errorf("failed to read user input: %w", err)
			}

			if n > 0 {
				choice := string(buffer[:n])
				choice = string(choice[0]) // Take first character only

				switch choice {
				case "p", "P":
					return &MenuChoice{Action: "play", Value: ""}, nil
				case "w", "W":
					return &MenuChoice{Action: "watch", Value: ""}, nil
				case "e", "E":
					return &MenuChoice{Action: "edit_profile", Value: ""}, nil
				case "l", "L":
					return &MenuChoice{Action: "list_games", Value: ""}, nil
				case "r", "R":
					return &MenuChoice{Action: "view_recordings", Value: ""}, nil
				case "s", "S":
					return &MenuChoice{Action: "statistics", Value: ""}, nil
				case "q", "Q":
					return &MenuChoice{Action: "quit", Value: ""}, nil
				default:
					// Invalid choice, show error and redisplay menu
					errorMsg := fmt.Sprintf("\r\nInvalid choice '%s'. Please try again.\r\n\r\n", choice)
					channel.Write([]byte(errorMsg))
					// Redisplay the banner
					channel.Write([]byte(banner))
				}
			}
		}
	}
}

// getFallbackAnonymousBanner returns a simple fallback banner for anonymous users
func (mh *MenuHandler) getFallbackAnonymousBanner() string {
	return fmt.Sprintf("\r\n=== DungeonGate ===\r\n\r\n"+
		"Connected as: Anonymous\r\n"+
		"Date: %s | Time: %s\r\n\r\n"+
		"Menu Options:\r\n"+
		"  [l] Login\r\n"+
		"  [r] Register\r\n"+
		"  [w] Watch games\r\n"+
		"  [g] List games\r\n"+
		"  [q] Quit\r\n\r\n"+
		"Choice: ",
		time.Now().Format("2006-01-02"),
		time.Now().Format("15:04:05"))
}

// getFallbackUserBanner returns a simple fallback banner for authenticated users
func (mh *MenuHandler) getFallbackUserBanner(username string) string {
	return fmt.Sprintf("\r\n=== DungeonGate ===\r\n\r\n"+
		"Welcome back, %s!\r\n"+
		"Date: %s | Time: %s\r\n\r\n"+
		"Menu Options:\r\n"+
		"  [p] Play a game\r\n"+
		"  [w] Watch games\r\n"+
		"  [e] Edit profile\r\n"+
		"  [l] List games\r\n"+
		"  [r] View recordings\r\n"+
		"  [s] Statistics\r\n"+
		"  [q] Quit\r\n\r\n"+
		"Choice: ",
		username,
		time.Now().Format("2006-01-02"),
		time.Now().Format("15:04:05"))
}