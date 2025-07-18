package application

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dungeongate/internal/games/domain"
	"github.com/dungeongate/pkg/logging"
)

// Mock repositories for testing
type MockSessionRepository struct {
	mock.Mock
}

func (m *MockSessionRepository) Save(ctx context.Context, session *domain.GameSession) error {
	args := m.Called(ctx, session)
	return args.Error(0)
}

func (m *MockSessionRepository) FindByID(ctx context.Context, id domain.SessionID) (*domain.GameSession, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*domain.GameSession), args.Error(1)
}

func (m *MockSessionRepository) FindByUserID(ctx context.Context, userID domain.UserID) ([]*domain.GameSession, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]*domain.GameSession), args.Error(1)
}

func (m *MockSessionRepository) FindByGameID(ctx context.Context, gameID domain.GameID) ([]*domain.GameSession, error) {
	args := m.Called(ctx, gameID)
	return args.Get(0).([]*domain.GameSession), args.Error(1)
}

func (m *MockSessionRepository) Delete(ctx context.Context, id domain.SessionID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockSessionRepository) FindActive(ctx context.Context) ([]*domain.GameSession, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*domain.GameSession), args.Error(1)
}

func (m *MockSessionRepository) FindActiveByUser(ctx context.Context, userID domain.UserID) ([]*domain.GameSession, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]*domain.GameSession), args.Error(1)
}

func (m *MockSessionRepository) FindActiveByGame(ctx context.Context, gameID domain.GameID) ([]*domain.GameSession, error) {
	args := m.Called(ctx, gameID)
	return args.Get(0).([]*domain.GameSession), args.Error(1)
}

func (m *MockSessionRepository) FindByStatus(ctx context.Context, status domain.SessionStatus) ([]*domain.GameSession, error) {
	args := m.Called(ctx, status)
	return args.Get(0).([]*domain.GameSession), args.Error(1)
}

func (m *MockSessionRepository) FindByDateRange(ctx context.Context, start, end time.Time) ([]*domain.GameSession, error) {
	args := m.Called(ctx, start, end)
	return args.Get(0).([]*domain.GameSession), args.Error(1)
}

func (m *MockSessionRepository) CountActiveByGame(ctx context.Context, gameID domain.GameID) (int, error) {
	args := m.Called(ctx, gameID)
	return args.Int(0), args.Error(1)
}

func (m *MockSessionRepository) CountTotalByUser(ctx context.Context, userID domain.UserID) (int, error) {
	args := m.Called(ctx, userID)
	return args.Int(0), args.Error(1)
}

func (m *MockSessionRepository) GetAverageSessionDuration(ctx context.Context, gameID domain.GameID) (time.Duration, error) {
	args := m.Called(ctx, gameID)
	return args.Get(0).(time.Duration), args.Error(1)
}

func (m *MockSessionRepository) DeleteExpiredSessions(ctx context.Context, maxAge time.Duration) (int, error) {
	args := m.Called(ctx, maxAge)
	return args.Int(0), args.Error(1)
}

type MockSaveRepository struct {
	mock.Mock
}

func (m *MockSaveRepository) Save(ctx context.Context, save *domain.GameSave) error {
	args := m.Called(ctx, save)
	return args.Error(0)
}

func (m *MockSaveRepository) FindByID(ctx context.Context, id domain.SaveID) (*domain.GameSave, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.GameSave), args.Error(1)
}

func (m *MockSaveRepository) FindByUserAndGame(ctx context.Context, userID domain.UserID, gameID domain.GameID) (*domain.GameSave, error) {
	args := m.Called(ctx, userID, gameID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.GameSave), args.Error(1)
}

func (m *MockSaveRepository) FindByUser(ctx context.Context, userID domain.UserID) ([]*domain.GameSave, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]*domain.GameSave), args.Error(1)
}

func (m *MockSaveRepository) FindByGame(ctx context.Context, gameID domain.GameID) ([]*domain.GameSave, error) {
	args := m.Called(ctx, gameID)
	return args.Get(0).([]*domain.GameSave), args.Error(1)
}

func (m *MockSaveRepository) Delete(ctx context.Context, id domain.SaveID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockSaveRepository) FindByStatus(ctx context.Context, status domain.SaveStatus) ([]*domain.GameSave, error) {
	args := m.Called(ctx, status)
	return args.Get(0).([]*domain.GameSave), args.Error(1)
}

func (m *MockSaveRepository) FindLargerThan(ctx context.Context, size int64) ([]*domain.GameSave, error) {
	args := m.Called(ctx, size)
	return args.Get(0).([]*domain.GameSave), args.Error(1)
}

func (m *MockSaveRepository) FindOlderThan(ctx context.Context, age time.Duration) ([]*domain.GameSave, error) {
	args := m.Called(ctx, age)
	return args.Get(0).([]*domain.GameSave), args.Error(1)
}

func (m *MockSaveRepository) SaveBackup(ctx context.Context, saveID domain.SaveID, backup domain.SaveBackup) error {
	args := m.Called(ctx, saveID, backup)
	return args.Error(0)
}

func (m *MockSaveRepository) FindBackups(ctx context.Context, saveID domain.SaveID) ([]domain.SaveBackup, error) {
	args := m.Called(ctx, saveID)
	return args.Get(0).([]domain.SaveBackup), args.Error(1)
}

func (m *MockSaveRepository) DeleteBackup(ctx context.Context, saveID domain.SaveID, backupID string) error {
	args := m.Called(ctx, saveID, backupID)
	return args.Error(0)
}

func (m *MockSaveRepository) GetTotalStorageUsed(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockSaveRepository) GetStorageUsedByUser(ctx context.Context, userID domain.UserID) (int64, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockSaveRepository) GetStorageUsedByGame(ctx context.Context, gameID domain.GameID) (int64, error) {
	args := m.Called(ctx, gameID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockSaveRepository) CleanupOldBackups(ctx context.Context, maxAge time.Duration) (int, error) {
	args := m.Called(ctx, maxAge)
	return args.Int(0), args.Error(1)
}

func (m *MockSaveRepository) CleanupDeletedSaves(ctx context.Context, maxAge time.Duration) (int, error) {
	args := m.Called(ctx, maxAge)
	return args.Int(0), args.Error(1)
}

type MockGameRepository struct {
	mock.Mock
}

func (m *MockGameRepository) Save(ctx context.Context, game *domain.Game) error {
	args := m.Called(ctx, game)
	return args.Error(0)
}

func (m *MockGameRepository) FindByID(ctx context.Context, id domain.GameID) (*domain.Game, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Game), args.Error(1)
}

func (m *MockGameRepository) FindByName(ctx context.Context, name string) (*domain.Game, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Game), args.Error(1)
}

func (m *MockGameRepository) FindAll(ctx context.Context) ([]*domain.Game, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*domain.Game), args.Error(1)
}

func (m *MockGameRepository) FindEnabled(ctx context.Context) ([]*domain.Game, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*domain.Game), args.Error(1)
}

func (m *MockGameRepository) Delete(ctx context.Context, id domain.GameID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockGameRepository) FindByCategory(ctx context.Context, category string) ([]*domain.Game, error) {
	args := m.Called(ctx, category)
	return args.Get(0).([]*domain.Game), args.Error(1)
}

func (m *MockGameRepository) FindByTag(ctx context.Context, tag string) ([]*domain.Game, error) {
	args := m.Called(ctx, tag)
	return args.Get(0).([]*domain.Game), args.Error(1)
}

func (m *MockGameRepository) SearchByName(ctx context.Context, query string) ([]*domain.Game, error) {
	args := m.Called(ctx, query)
	return args.Get(0).([]*domain.Game), args.Error(1)
}

func (m *MockGameRepository) CountByStatus(ctx context.Context, status domain.GameStatus) (int, error) {
	args := m.Called(ctx, status)
	return args.Int(0), args.Error(1)
}

func (m *MockGameRepository) UpdateStatistics(ctx context.Context, id domain.GameID, stats domain.GameStatistics) error {
	args := m.Called(ctx, id, stats)
	return args.Error(0)
}

func (m *MockGameRepository) GetMostPopular(ctx context.Context, limit int) ([]*domain.Game, error) {
	args := m.Called(ctx, limit)
	return args.Get(0).([]*domain.Game), args.Error(1)
}

type MockEventRepository struct {
	mock.Mock
}

func (m *MockEventRepository) SaveEvent(ctx context.Context, event *domain.GameEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockEventRepository) FindEvents(ctx context.Context, filters domain.EventFilters) ([]*domain.GameEvent, error) {
	args := m.Called(ctx, filters)
	return args.Get(0).([]*domain.GameEvent), args.Error(1)
}

func (m *MockEventRepository) FindEventsBySession(ctx context.Context, sessionID domain.SessionID) ([]*domain.GameEvent, error) {
	args := m.Called(ctx, sessionID)
	return args.Get(0).([]*domain.GameEvent), args.Error(1)
}

func (m *MockEventRepository) FindEventsByGame(ctx context.Context, gameID domain.GameID) ([]*domain.GameEvent, error) {
	args := m.Called(ctx, gameID)
	return args.Get(0).([]*domain.GameEvent), args.Error(1)
}

func (m *MockEventRepository) FindEventsByUser(ctx context.Context, userID domain.UserID) ([]*domain.GameEvent, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]*domain.GameEvent), args.Error(1)
}

func (m *MockEventRepository) DeleteOldEvents(ctx context.Context, maxAge time.Duration) (int, error) {
	args := m.Called(ctx, maxAge)
	return args.Int(0), args.Error(1)
}

// Test helper functions
func createTestSessionManager(t *testing.T) (*SessionManager, *MockSessionRepository, *MockSaveRepository, *MockGameRepository, *MockEventRepository, string) {
	sessionRepo := &MockSessionRepository{}
	saveRepo := &MockSaveRepository{}
	gameRepo := &MockGameRepository{}
	eventRepo := &MockEventRepository{}

	// Create temporary directory for testing
	tempDir := t.TempDir()

	logger := createTestLogger(t)

	sm := NewSessionManager(sessionRepo, saveRepo, gameRepo, eventRepo, tempDir, logger)

	return sm, sessionRepo, saveRepo, gameRepo, eventRepo, tempDir
}

func createTestLogger(t *testing.T) *slog.Logger {
	return logging.NewLoggerBasic("test", "debug", "text", "stdout")
}

// Test Cases

func TestSessionManager_StartGameSession_NewUser(t *testing.T) {
	sm, sessionRepo, saveRepo, gameRepo, eventRepo, tempDir := createTestSessionManager(t)
	ctx := context.Background()

	userID := 123
	gameID := "nethack"
	terminalSize := domain.TerminalSize{Width: 80, Height: 24}

	// Create mock game
	game := createMockGame(gameID)

	// Set up mock expectations
	gameRepo.On("FindByID", ctx, domain.NewGameID(gameID)).Return(game, nil)
	saveRepo.On("FindByUserAndGame", ctx, domain.NewUserID(userID), domain.NewGameID(gameID)).Return(nil, fmt.Errorf("not found"))
	sessionRepo.On("Save", ctx, mock.AnythingOfType("*domain.GameSession")).Return(nil)
	eventRepo.On("SaveEvent", ctx, mock.AnythingOfType("*domain.GameEvent")).Return(nil)

	// Test starting a new game session
	session, err := sm.StartGameSession(ctx, userID, gameID, terminalSize)

	// Assertions
	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, userID, session.UserID().Int())
	assert.Equal(t, gameID, session.GameID().String())
	assert.Equal(t, terminalSize, session.TerminalSize())

	// Verify session directory was created
	sessionDir := filepath.Join(tempDir, "sessions", session.ID().String())
	assert.DirExists(t, sessionDir)

	// Verify mocks were called
	gameRepo.AssertExpectations(t)
	saveRepo.AssertExpectations(t)
	sessionRepo.AssertExpectations(t)
	eventRepo.AssertExpectations(t)
}

func TestSessionManager_StartGameSession_WithExistingSave(t *testing.T) {
	sm, sessionRepo, saveRepo, gameRepo, eventRepo, tempDir := createTestSessionManager(t)
	ctx := context.Background()

	userID := 123
	gameID := "nethack"
	terminalSize := domain.TerminalSize{Width: 80, Height: 24}

	// Create test save file
	testSaveData := []byte("test save data")
	testSavePath := filepath.Join(tempDir, "test_save.dat")
	err := os.WriteFile(testSavePath, testSaveData, 0644)
	require.NoError(t, err)

	// Create mock game and save
	game := createMockGame(gameID)
	existingSave := createMockSave(userID, gameID, testSavePath, testSaveData)

	// Set up mock expectations
	gameRepo.On("FindByID", ctx, domain.NewGameID(gameID)).Return(game, nil)
	saveRepo.On("FindByUserAndGame", ctx, domain.NewUserID(userID), domain.NewGameID(gameID)).Return(existingSave, nil)
	sessionRepo.On("Save", ctx, mock.AnythingOfType("*domain.GameSession")).Return(nil)
	eventRepo.On("SaveEvent", ctx, mock.AnythingOfType("*domain.GameEvent")).Return(nil)

	// Test starting game session with existing save
	session, err := sm.StartGameSession(ctx, userID, gameID, terminalSize)

	// Assertions
	require.NoError(t, err)
	require.NotNil(t, session)

	// Verify save was copied to session directory
	sessionDir := filepath.Join(tempDir, "sessions", session.ID().String())
	sessionSavePath := filepath.Join(sessionDir, "save", "save.dat")
	assert.FileExists(t, sessionSavePath)

	// Verify save content
	copiedData, err := os.ReadFile(sessionSavePath)
	require.NoError(t, err)
	assert.Equal(t, testSaveData, copiedData)

	// Verify mocks were called
	gameRepo.AssertExpectations(t)
	saveRepo.AssertExpectations(t)
	sessionRepo.AssertExpectations(t)
	eventRepo.AssertExpectations(t)
}

func TestSessionManager_CreateSaveFromSession(t *testing.T) {
	sm, _, saveRepo, _, _, tempDir := createTestSessionManager(t)
	ctx := context.Background()

	// Create test session
	session := createMockSession(123, "nethack")

	// Create session directory and save file
	sessionDir := filepath.Join(tempDir, "sessions", session.ID().String())
	saveDir := filepath.Join(sessionDir, "save")
	err := os.MkdirAll(saveDir, 0755)
	require.NoError(t, err)

	testSaveData := []byte("updated save data")
	saveFile := filepath.Join(saveDir, "save.dat")
	err = os.WriteFile(saveFile, testSaveData, 0644)
	require.NoError(t, err)

	// Set up mock expectations
	saveRepo.On("Save", ctx, mock.AnythingOfType("*domain.GameSave")).Return(nil)

	// Test creating save from session
	err = sm.createSaveFromSession(ctx, session)

	// Assertions
	require.NoError(t, err)

	// Verify permanent save directory was created
	userSaveDir := filepath.Join(tempDir, "saves", "user_123", "nethack")
	assert.DirExists(t, userSaveDir)

	// Verify permanent save file exists
	permanentSaves, err := filepath.Glob(filepath.Join(userSaveDir, "save_*.dat"))
	require.NoError(t, err)
	assert.Len(t, permanentSaves, 1)

	// Verify save content
	savedData, err := os.ReadFile(permanentSaves[0])
	require.NoError(t, err)
	assert.Equal(t, testSaveData, savedData)

	// Verify mocks were called
	saveRepo.AssertExpectations(t)
}

func TestSessionManager_EndGameSession(t *testing.T) {
	sm, sessionRepo, saveRepo, _, eventRepo, tempDir := createTestSessionManager(t)
	ctx := context.Background()

	// Create test session
	session := createMockSession(123, "nethack")
	sessionID := session.ID().String()

	// Create session directory and save file
	sessionDir := filepath.Join(tempDir, "sessions", sessionID)
	saveDir := filepath.Join(sessionDir, "save")
	err := os.MkdirAll(saveDir, 0755)
	require.NoError(t, err)

	testSaveData := []byte("final save data")
	saveFile := filepath.Join(saveDir, "save.dat")
	err = os.WriteFile(saveFile, testSaveData, 0644)
	require.NoError(t, err)

	// Set up mock expectations
	sessionRepo.On("FindByID", ctx, domain.NewSessionID(sessionID)).Return(session, nil)
	saveRepo.On("Save", ctx, mock.AnythingOfType("*domain.GameSave")).Return(nil)
	sessionRepo.On("Save", ctx, mock.AnythingOfType("*domain.GameSession")).Return(nil)
	eventRepo.On("SaveEvent", ctx, mock.AnythingOfType("*domain.GameEvent")).Return(nil)

	// Test ending game session
	err = sm.EndGameSession(ctx, sessionID)

	// Assertions
	require.NoError(t, err)

	// Verify session status was updated to ended
	assert.Equal(t, domain.SessionStatusEnded, session.Status())

	// Verify mocks were called
	sessionRepo.AssertExpectations(t)
	saveRepo.AssertExpectations(t)
	eventRepo.AssertExpectations(t)
}

// Helper functions for creating mock objects

func createMockGame(gameID string) *domain.Game {
	// Create a mock game that uses a simple test binary
	config := domain.GameConfig{
		Binary: domain.BinaryConfig{
			Path:             "/bin/sleep",  // Use sleep as a safe test binary
			Args:             []string{"1"}, // Sleep for 1 second
			WorkingDirectory: "/tmp",
		},
		Environment: map[string]string{
			"TERM": "xterm",
		},
		Resources: domain.ResourceConfig{
			CPULimit:    "50m",   // 50 millicores
			MemoryLimit: "512Mi", // 512MB
			DiskLimit:   "1Gi",   // 1GB
		},
		Security: domain.SecurityConfig{
			RunAsUser:                1000,
			RunAsGroup:               1000,
			ReadOnlyRootFilesystem:   true,
			AllowPrivilegeEscalation: false,
			Capabilities:             []string{},
		},
		Networking: domain.NetworkConfig{
			Isolated:      true,
			BlockInternet: true,
		},
	}

	metadata := domain.GameMetadata{
		Name:        "Test Game",
		ShortName:   gameID,
		Description: "Mock game for testing",
		Category:    "test",
		Tags:        []string{"test"},
		Version:     "1.0.0",
		Difficulty:  1,
	}

	return domain.NewGame(
		domain.NewGameID(gameID),
		metadata,
		config,
	)
}

func createMockSave(userID int, gameID, filePath string, data []byte) *domain.GameSave {
	saveID := domain.NewSaveID(uuid.New().String())
	userIDDomain := domain.NewUserID(userID)
	gameIDDomain := domain.NewGameID(gameID)

	metadata := domain.SaveMetadata{
		GameVersion: "1.0",
		Character:   "TestCharacter",
		Level:       5,
		Score:       1000,
		PlayTime:    time.Hour,
		Location:    "Test Dungeon",
	}

	return domain.NewGameSave(saveID, userIDDomain, gameIDDomain, data, filePath, metadata)
}

func createMockSession(userID int, gameID string) *domain.GameSession {
	sessionID := domain.NewSessionID(uuid.New().String())
	userIDDomain := domain.NewUserID(userID)
	gameIDDomain := domain.NewGameID(gameID)
	terminalSize := domain.TerminalSize{Width: 80, Height: 24}

	session := domain.NewGameSession(sessionID, userIDDomain, "testuser", gameIDDomain, domain.GameConfig{}, terminalSize)

	// Start the session to make it active
	session.Start(domain.ProcessInfo{PID: 12345, PodName: "test-pod"})

	return session
}
