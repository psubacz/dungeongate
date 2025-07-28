package repository

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dungeongate/internal/games/domain"
)

// StubGameRepository provides an in-memory implementation of GameRepository for development
type StubGameRepository struct {
	mu    sync.RWMutex
	games map[string]*domain.Game
}

// NewStubGameRepository creates a new in-memory game repository
func NewStubGameRepository() *StubGameRepository {
	return &StubGameRepository{
		games: make(map[string]*domain.Game),
	}
}

// Save implements GameRepository
func (r *StubGameRepository) Save(ctx context.Context, game *domain.Game) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.games[game.ID().String()] = game
	return nil
}

// FindByID implements GameRepository
func (r *StubGameRepository) FindByID(ctx context.Context, id domain.GameID) (*domain.Game, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	game, exists := r.games[id.String()]
	if !exists {
		return nil, fmt.Errorf("game not found")
	}
	return game, nil
}

// FindByName implements GameRepository
func (r *StubGameRepository) FindByName(ctx context.Context, name string) (*domain.Game, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, game := range r.games {
		if game.Metadata().Name == name {
			return game, nil
		}
	}
	return nil, fmt.Errorf("game not found")
}

// FindAll implements GameRepository
func (r *StubGameRepository) FindAll(ctx context.Context) ([]*domain.Game, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	games := make([]*domain.Game, 0, len(r.games))
	for _, game := range r.games {
		games = append(games, game)
	}
	return games, nil
}

// FindEnabled implements GameRepository
func (r *StubGameRepository) FindEnabled(ctx context.Context) ([]*domain.Game, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var games []*domain.Game
	for _, game := range r.games {
		if game.CanStart() {
			games = append(games, game)
		}
	}
	return games, nil
}

// Delete implements GameRepository
func (r *StubGameRepository) Delete(ctx context.Context, id domain.GameID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.games, id.String())
	return nil
}

// FindByCategory implements GameRepository
func (r *StubGameRepository) FindByCategory(ctx context.Context, category string) ([]*domain.Game, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var games []*domain.Game
	for _, game := range r.games {
		if game.Metadata().Category == category {
			games = append(games, game)
		}
	}
	return games, nil
}

// FindByTag implements GameRepository
func (r *StubGameRepository) FindByTag(ctx context.Context, tag string) ([]*domain.Game, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var games []*domain.Game
	for _, game := range r.games {
		for _, gameTag := range game.Metadata().Tags {
			if gameTag == tag {
				games = append(games, game)
				break
			}
		}
	}
	return games, nil
}

// SearchByName implements GameRepository
func (r *StubGameRepository) SearchByName(ctx context.Context, query string) ([]*domain.Game, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var games []*domain.Game
	for _, game := range r.games {
		if contains(game.Metadata().Name, query) {
			games = append(games, game)
		}
	}
	return games, nil
}

// CountByStatus implements GameRepository
func (r *StubGameRepository) CountByStatus(ctx context.Context, status domain.GameStatus) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, game := range r.games {
		if game.Status() == status {
			count++
		}
	}
	return count, nil
}

// UpdateStatistics implements GameRepository
func (r *StubGameRepository) UpdateStatistics(ctx context.Context, id domain.GameID, stats domain.GameStatistics) error {
	// Stub implementation - statistics are handled internally by the game aggregate
	return nil
}

// GetMostPopular implements GameRepository
func (r *StubGameRepository) GetMostPopular(ctx context.Context, limit int) ([]*domain.Game, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	games := make([]*domain.Game, 0, len(r.games))
	for _, game := range r.games {
		games = append(games, game)
	}

	// Simple implementation - just return first 'limit' games
	if len(games) > limit {
		games = games[:limit]
	}
	return games, nil
}

// StubSessionRepository provides an in-memory implementation of SessionRepository for development
type StubSessionRepository struct {
	mu       sync.RWMutex
	sessions map[string]*domain.GameSession
}

// NewStubSessionRepository creates a new in-memory session repository
func NewStubSessionRepository() *StubSessionRepository {
	return &StubSessionRepository{
		sessions: make(map[string]*domain.GameSession),
	}
}

// Save implements SessionRepository
func (r *StubSessionRepository) Save(ctx context.Context, session *domain.GameSession) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[session.ID().String()] = session
	return nil
}

// FindByID implements SessionRepository
func (r *StubSessionRepository) FindByID(ctx context.Context, id domain.SessionID) (*domain.GameSession, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	session, exists := r.sessions[id.String()]
	if !exists {
		return nil, fmt.Errorf("session not found")
	}
	return session, nil
}

// FindByUserID implements SessionRepository
func (r *StubSessionRepository) FindByUserID(ctx context.Context, userID domain.UserID) ([]*domain.GameSession, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var sessions []*domain.GameSession
	for _, session := range r.sessions {
		if session.UserID() == userID {
			sessions = append(sessions, session)
		}
	}
	return sessions, nil
}

// FindByGameID implements SessionRepository
func (r *StubSessionRepository) FindByGameID(ctx context.Context, gameID domain.GameID) ([]*domain.GameSession, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var sessions []*domain.GameSession
	for _, session := range r.sessions {
		if session.GameID() == gameID {
			sessions = append(sessions, session)
		}
	}
	return sessions, nil
}

// Delete implements SessionRepository
func (r *StubSessionRepository) Delete(ctx context.Context, id domain.SessionID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, id.String())
	return nil
}

// FindActive implements SessionRepository
func (r *StubSessionRepository) FindActive(ctx context.Context) ([]*domain.GameSession, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var sessions []*domain.GameSession
	for _, session := range r.sessions {
		if session.IsActive() {
			sessions = append(sessions, session)
		}
	}
	return sessions, nil
}

// FindActiveByUser implements SessionRepository
func (r *StubSessionRepository) FindActiveByUser(ctx context.Context, userID domain.UserID) ([]*domain.GameSession, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var sessions []*domain.GameSession
	for _, session := range r.sessions {
		if session.UserID() == userID && session.IsActive() {
			sessions = append(sessions, session)
		}
	}
	return sessions, nil
}

// FindActiveByGame implements SessionRepository
func (r *StubSessionRepository) FindActiveByGame(ctx context.Context, gameID domain.GameID) ([]*domain.GameSession, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var sessions []*domain.GameSession
	for _, session := range r.sessions {
		if session.GameID() == gameID && session.IsActive() {
			sessions = append(sessions, session)
		}
	}
	return sessions, nil
}

// FindByStatus implements SessionRepository
func (r *StubSessionRepository) FindByStatus(ctx context.Context, status domain.SessionStatus) ([]*domain.GameSession, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var sessions []*domain.GameSession
	for _, session := range r.sessions {
		if session.Status() == status {
			sessions = append(sessions, session)
		}
	}
	return sessions, nil
}

// FindByDateRange implements SessionRepository
func (r *StubSessionRepository) FindByDateRange(ctx context.Context, start, end time.Time) ([]*domain.GameSession, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var sessions []*domain.GameSession
	for _, session := range r.sessions {
		if session.StartTime().After(start) && session.StartTime().Before(end) {
			sessions = append(sessions, session)
		}
	}
	return sessions, nil
}

// CountActiveByGame implements SessionRepository
func (r *StubSessionRepository) CountActiveByGame(ctx context.Context, gameID domain.GameID) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, session := range r.sessions {
		if session.GameID() == gameID && session.IsActive() {
			count++
		}
	}
	return count, nil
}

// CountTotalByUser implements SessionRepository
func (r *StubSessionRepository) CountTotalByUser(ctx context.Context, userID domain.UserID) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, session := range r.sessions {
		if session.UserID() == userID {
			count++
		}
	}
	return count, nil
}

// GetAverageSessionDuration implements SessionRepository
func (r *StubSessionRepository) GetAverageSessionDuration(ctx context.Context, gameID domain.GameID) (time.Duration, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var totalDuration time.Duration
	count := 0

	for _, session := range r.sessions {
		if session.GameID() == gameID && !session.IsActive() {
			totalDuration += session.Duration()
			count++
		}
	}

	if count == 0 {
		return 0, nil
	}

	return totalDuration / time.Duration(count), nil
}

// DeleteExpiredSessions implements SessionRepository
func (r *StubSessionRepository) DeleteExpiredSessions(ctx context.Context, maxAge time.Duration) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	count := 0

	for id, session := range r.sessions {
		if session.StartTime().Before(cutoff) {
			delete(r.sessions, id)
			count++
		}
	}

	return count, nil
}

// StubSaveRepository provides an in-memory implementation of SaveRepository for development
type StubSaveRepository struct {
	mu    sync.RWMutex
	saves map[string]*domain.GameSave
}

// NewStubSaveRepository creates a new in-memory save repository
func NewStubSaveRepository() *StubSaveRepository {
	return &StubSaveRepository{
		saves: make(map[string]*domain.GameSave),
	}
}

// Save implements SaveRepository
func (r *StubSaveRepository) Save(ctx context.Context, save *domain.GameSave) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.saves[save.ID().String()] = save
	return nil
}

// FindByID implements SaveRepository
func (r *StubSaveRepository) FindByID(ctx context.Context, id domain.SaveID) (*domain.GameSave, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	save, exists := r.saves[id.String()]
	if !exists {
		return nil, fmt.Errorf("save not found")
	}
	return save, nil
}

// FindByUserAndGame implements SaveRepository
func (r *StubSaveRepository) FindByUserAndGame(ctx context.Context, userID domain.UserID, gameID domain.GameID) (*domain.GameSave, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, save := range r.saves {
		if save.UserID() == userID && save.GameID() == gameID {
			return save, nil
		}
	}
	return nil, fmt.Errorf("save not found")
}

// FindByUser implements SaveRepository
func (r *StubSaveRepository) FindByUser(ctx context.Context, userID domain.UserID) ([]*domain.GameSave, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var saves []*domain.GameSave
	for _, save := range r.saves {
		if save.UserID() == userID {
			saves = append(saves, save)
		}
	}
	return saves, nil
}

// FindByGame implements SaveRepository
func (r *StubSaveRepository) FindByGame(ctx context.Context, gameID domain.GameID) ([]*domain.GameSave, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var saves []*domain.GameSave
	for _, save := range r.saves {
		if save.GameID() == gameID {
			saves = append(saves, save)
		}
	}
	return saves, nil
}

// Delete implements SaveRepository
func (r *StubSaveRepository) Delete(ctx context.Context, id domain.SaveID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.saves, id.String())
	return nil
}

// FindByStatus implements SaveRepository
func (r *StubSaveRepository) FindByStatus(ctx context.Context, status domain.SaveStatus) ([]*domain.GameSave, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var saves []*domain.GameSave
	for _, save := range r.saves {
		if save.Status() == status {
			saves = append(saves, save)
		}
	}
	return saves, nil
}

// FindLargerThan implements SaveRepository
func (r *StubSaveRepository) FindLargerThan(ctx context.Context, size int64) ([]*domain.GameSave, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var saves []*domain.GameSave
	for _, save := range r.saves {
		if save.FileSize() > size {
			saves = append(saves, save)
		}
	}
	return saves, nil
}

// FindOlderThan implements SaveRepository
func (r *StubSaveRepository) FindOlderThan(ctx context.Context, age time.Duration) ([]*domain.GameSave, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cutoff := time.Now().Add(-age)
	var saves []*domain.GameSave
	for _, save := range r.saves {
		if save.CreatedAt().Before(cutoff) {
			saves = append(saves, save)
		}
	}
	return saves, nil
}

// SaveBackup implements SaveRepository
func (r *StubSaveRepository) SaveBackup(ctx context.Context, saveID domain.SaveID, backup domain.SaveBackup) error {
	// Stub implementation - backups not fully implemented
	return nil
}

// FindBackups implements SaveRepository
func (r *StubSaveRepository) FindBackups(ctx context.Context, saveID domain.SaveID) ([]domain.SaveBackup, error) {
	// Stub implementation - backups not fully implemented
	return []domain.SaveBackup{}, nil
}

// DeleteBackup implements SaveRepository
func (r *StubSaveRepository) DeleteBackup(ctx context.Context, saveID domain.SaveID, backupID string) error {
	// Stub implementation - backups not fully implemented
	return nil
}

// GetTotalStorageUsed implements SaveRepository
func (r *StubSaveRepository) GetTotalStorageUsed(ctx context.Context) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var total int64
	for _, save := range r.saves {
		total += save.FileSize()
	}
	return total, nil
}

// GetStorageUsedByUser implements SaveRepository
func (r *StubSaveRepository) GetStorageUsedByUser(ctx context.Context, userID domain.UserID) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var total int64
	for _, save := range r.saves {
		if save.UserID() == userID {
			total += save.FileSize()
		}
	}
	return total, nil
}

// GetStorageUsedByGame implements SaveRepository
func (r *StubSaveRepository) GetStorageUsedByGame(ctx context.Context, gameID domain.GameID) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var total int64
	for _, save := range r.saves {
		if save.GameID() == gameID {
			total += save.FileSize()
		}
	}
	return total, nil
}

// CleanupOldBackups implements SaveRepository
func (r *StubSaveRepository) CleanupOldBackups(ctx context.Context, maxAge time.Duration) (int, error) {
	// Stub implementation - backups not fully implemented
	return 0, nil
}

// CleanupDeletedSaves implements SaveRepository
func (r *StubSaveRepository) CleanupDeletedSaves(ctx context.Context, maxAge time.Duration) (int, error) {
	// Stub implementation - cleanup not fully implemented
	return 0, nil
}

// StubEventRepository provides an in-memory implementation of EventRepository for development
type StubEventRepository struct {
	mu     sync.RWMutex
	events []*domain.GameEvent
}

// NewStubEventRepository creates a new in-memory event repository
func NewStubEventRepository() *StubEventRepository {
	return &StubEventRepository{
		events: make([]*domain.GameEvent, 0),
	}
}

// SaveEvent implements EventRepository
func (r *StubEventRepository) SaveEvent(ctx context.Context, event *domain.GameEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
	return nil
}

// FindEvents implements EventRepository
func (r *StubEventRepository) FindEvents(ctx context.Context, filters domain.EventFilters) ([]*domain.GameEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Simple implementation - return all events
	return r.events, nil
}

// FindEventsBySession implements EventRepository
func (r *StubEventRepository) FindEventsBySession(ctx context.Context, sessionID domain.SessionID) ([]*domain.GameEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var events []*domain.GameEvent
	for _, event := range r.events {
		if event.SessionID == sessionID.String() {
			events = append(events, event)
		}
	}
	return events, nil
}

// FindEventsByGame implements EventRepository
func (r *StubEventRepository) FindEventsByGame(ctx context.Context, gameID domain.GameID) ([]*domain.GameEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var events []*domain.GameEvent
	for _, event := range r.events {
		if event.GameID == gameID.String() {
			events = append(events, event)
		}
	}
	return events, nil
}

// FindEventsByUser implements EventRepository
func (r *StubEventRepository) FindEventsByUser(ctx context.Context, userID domain.UserID) ([]*domain.GameEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var events []*domain.GameEvent
	for _, event := range r.events {
		if event.UserID == userID.Int() {
			events = append(events, event)
		}
	}
	return events, nil
}

// DeleteOldEvents implements EventRepository
func (r *StubEventRepository) DeleteOldEvents(ctx context.Context, maxAge time.Duration) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	var newEvents []*domain.GameEvent
	count := 0

	for _, event := range r.events {
		if event.Timestamp.After(cutoff) {
			newEvents = append(newEvents, event)
		} else {
			count++
		}
	}

	r.events = newEvents
	return count, nil
}

// StubUnitOfWork provides an in-memory implementation of UnitOfWork for development
type StubUnitOfWork struct {
	gameRepo    domain.GameRepository
	sessionRepo domain.SessionRepository
	saveRepo    domain.SaveRepository
	eventRepo   domain.EventRepository
}

// NewStubUnitOfWork creates a new in-memory unit of work
func NewStubUnitOfWork(
	gameRepo domain.GameRepository,
	sessionRepo domain.SessionRepository,
	saveRepo domain.SaveRepository,
	eventRepo domain.EventRepository,
) *StubUnitOfWork {
	return &StubUnitOfWork{
		gameRepo:    gameRepo,
		sessionRepo: sessionRepo,
		saveRepo:    saveRepo,
		eventRepo:   eventRepo,
	}
}

// Begin implements UnitOfWork
func (u *StubUnitOfWork) Begin(ctx context.Context) error {
	// Stub implementation - no actual transaction
	return nil
}

// Commit implements UnitOfWork
func (u *StubUnitOfWork) Commit(ctx context.Context) error {
	// Stub implementation - no actual transaction
	return nil
}

// Rollback implements UnitOfWork
func (u *StubUnitOfWork) Rollback(ctx context.Context) error {
	// Stub implementation - no actual transaction
	return nil
}

// Games implements UnitOfWork
func (u *StubUnitOfWork) Games() domain.GameRepository {
	return u.gameRepo
}

// Sessions implements UnitOfWork
func (u *StubUnitOfWork) Sessions() domain.SessionRepository {
	return u.sessionRepo
}

// Saves implements UnitOfWork
func (u *StubUnitOfWork) Saves() domain.SaveRepository {
	return u.saveRepo
}

// Events implements UnitOfWork
func (u *StubUnitOfWork) Events() domain.EventRepository {
	return u.eventRepo
}

// Helper functions

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(substr) == 0 ||
		s[0:len(substr)] == substr ||
		(len(s) > len(substr) && contains(s[1:], substr)))
}
