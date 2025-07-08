package domain

import (
	"context"
	"time"
)

// GameRepository defines the interface for game persistence
type GameRepository interface {
	// Game CRUD operations
	Save(ctx context.Context, game *Game) error
	FindByID(ctx context.Context, id GameID) (*Game, error)
	FindByName(ctx context.Context, name string) (*Game, error)
	FindAll(ctx context.Context) ([]*Game, error)
	FindEnabled(ctx context.Context) ([]*Game, error)
	Delete(ctx context.Context, id GameID) error

	// Game queries
	FindByCategory(ctx context.Context, category string) ([]*Game, error)
	FindByTag(ctx context.Context, tag string) ([]*Game, error)
	SearchByName(ctx context.Context, query string) ([]*Game, error)
	CountByStatus(ctx context.Context, status GameStatus) (int, error)

	// Statistics
	UpdateStatistics(ctx context.Context, id GameID, stats GameStatistics) error
	GetMostPopular(ctx context.Context, limit int) ([]*Game, error)
}

// SessionRepository defines the interface for game session persistence
type SessionRepository interface {
	// Session CRUD operations
	Save(ctx context.Context, session *GameSession) error
	FindByID(ctx context.Context, id SessionID) (*GameSession, error)
	FindByUserID(ctx context.Context, userID UserID) ([]*GameSession, error)
	FindByGameID(ctx context.Context, gameID GameID) ([]*GameSession, error)
	Delete(ctx context.Context, id SessionID) error

	// Session queries
	FindActive(ctx context.Context) ([]*GameSession, error)
	FindActiveByUser(ctx context.Context, userID UserID) ([]*GameSession, error)
	FindActiveByGame(ctx context.Context, gameID GameID) ([]*GameSession, error)
	FindByStatus(ctx context.Context, status SessionStatus) ([]*GameSession, error)
	FindByDateRange(ctx context.Context, start, end time.Time) ([]*GameSession, error)

	// Statistics
	CountActiveByGame(ctx context.Context, gameID GameID) (int, error)
	CountTotalByUser(ctx context.Context, userID UserID) (int, error)
	GetAverageSessionDuration(ctx context.Context, gameID GameID) (time.Duration, error)

	// Cleanup
	DeleteExpiredSessions(ctx context.Context, maxAge time.Duration) (int, error)
}

// SaveRepository defines the interface for game save persistence
type SaveRepository interface {
	// Save CRUD operations
	Save(ctx context.Context, save *GameSave) error
	FindByID(ctx context.Context, id SaveID) (*GameSave, error)
	FindByUserAndGame(ctx context.Context, userID UserID, gameID GameID) (*GameSave, error)
	FindByUser(ctx context.Context, userID UserID) ([]*GameSave, error)
	FindByGame(ctx context.Context, gameID GameID) ([]*GameSave, error)
	Delete(ctx context.Context, id SaveID) error

	// Save queries
	FindByStatus(ctx context.Context, status SaveStatus) ([]*GameSave, error)
	FindLargerThan(ctx context.Context, size int64) ([]*GameSave, error)
	FindOlderThan(ctx context.Context, age time.Duration) ([]*GameSave, error)

	// Backup operations
	SaveBackup(ctx context.Context, saveID SaveID, backup SaveBackup) error
	FindBackups(ctx context.Context, saveID SaveID) ([]SaveBackup, error)
	DeleteBackup(ctx context.Context, saveID SaveID, backupID string) error

	// Storage statistics
	GetTotalStorageUsed(ctx context.Context) (int64, error)
	GetStorageUsedByUser(ctx context.Context, userID UserID) (int64, error)
	GetStorageUsedByGame(ctx context.Context, gameID GameID) (int64, error)

	// Cleanup
	CleanupOldBackups(ctx context.Context, maxAge time.Duration) (int, error)
	CleanupDeletedSaves(ctx context.Context, maxAge time.Duration) (int, error)
}

// GameEvent represents a game-related event (defined here to avoid circular imports)
type GameEvent struct {
	ID        string                 `json:"id"`
	Type      GameEventType          `json:"type"`
	GameID    string                 `json:"game_id"`
	SessionID string                 `json:"session_id"`
	UserID    int                    `json:"user_id"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

// GameEventType represents the type of game event
type GameEventType string

const (
	GameEventTypeSessionStart   GameEventType = "game.session.start"
	GameEventTypeSessionEnd     GameEventType = "game.session.end"
	GameEventTypeSessionPause   GameEventType = "game.session.pause"
	GameEventTypeSessionResume  GameEventType = "game.session.resume"
	GameEventTypeGameSave       GameEventType = "game.save"
	GameEventTypeGameLoad       GameEventType = "game.load"
	GameEventTypeSpectatorJoin  GameEventType = "game.spectator.join"
	GameEventTypeSpectatorLeave GameEventType = "game.spectator.leave"
)

// EventRepository defines the interface for game event persistence
type EventRepository interface {
	// Event operations
	SaveEvent(ctx context.Context, event *GameEvent) error
	FindEvents(ctx context.Context, filters EventFilters) ([]*GameEvent, error)
	FindEventsBySession(ctx context.Context, sessionID SessionID) ([]*GameEvent, error)
	FindEventsByGame(ctx context.Context, gameID GameID) ([]*GameEvent, error)
	FindEventsByUser(ctx context.Context, userID UserID) ([]*GameEvent, error)

	// Event cleanup
	DeleteOldEvents(ctx context.Context, maxAge time.Duration) (int, error)
}

// EventFilters represents filters for querying events
type EventFilters struct {
	SessionID *SessionID
	GameID    *GameID
	UserID    *UserID
	EventType *GameEventType
	StartTime *time.Time
	EndTime   *time.Time
	Limit     int
	Offset    int
}

// UnitOfWork defines a unit of work pattern for transactional operations
type UnitOfWork interface {
	// Transaction management
	Begin(ctx context.Context) error
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error

	// Repository access within transaction
	Games() GameRepository
	Sessions() SessionRepository
	Saves() SaveRepository
	Events() EventRepository
}
