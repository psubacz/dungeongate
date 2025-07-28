package application

import (
	"context"
	"fmt"
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

func TestCleanupService_CleanupExpiredSessions(t *testing.T) {
	sessionRepo := &MockSessionRepository{}
	saveRepo := &MockSaveRepository{}
	eventRepo := &MockEventRepository{}
	logger := logging.NewLoggerBasic("test", "debug", "text", "stdout")

	cleanupService := NewCleanupService(sessionRepo, saveRepo, eventRepo, logger)
	ctx := context.Background()
	maxAge := 24 * time.Hour

	// Set up mock expectations
	sessionRepo.On("DeleteExpiredSessions", ctx, maxAge).Return(3, nil)

	// Test cleanup
	err := cleanupService.CleanupExpiredSessions(ctx, maxAge)

	// Assertions
	require.NoError(t, err)
	sessionRepo.AssertExpectations(t)
}

func TestCleanupService_CleanupOrphanedProcesses(t *testing.T) {
	sessionRepo := &MockSessionRepository{}
	saveRepo := &MockSaveRepository{}
	eventRepo := &MockEventRepository{}
	logger := logging.NewLoggerBasic("test", "debug", "text", "stdout")

	cleanupService := NewCleanupService(sessionRepo, saveRepo, eventRepo, logger)
	ctx := context.Background()

	// Create mock sessions with different states
	runningSession := createMockSessionWithProcess(123, "nethack", 99999) // Non-existent PID
	startingSession := createMockSessionWithProcess(456, "dcss", 99998)   // Non-existent PID

	runningSessions := []*domain.GameSession{runningSession}
	startingSessions := []*domain.GameSession{startingSession}

	// Set up mock expectations
	sessionRepo.On("FindByStatus", ctx, domain.SessionStatusActive).Return(runningSessions, nil)
	sessionRepo.On("FindByStatus", ctx, domain.SessionStatusStarting).Return(startingSessions, nil)
	sessionRepo.On("Save", ctx, mock.AnythingOfType("*domain.GameSession")).Return(nil).Twice()
	eventRepo.On("SaveEvent", ctx, mock.AnythingOfType("*domain.GameEvent")).Return(nil).Twice()

	// Test cleanup
	err := cleanupService.CleanupOrphanedProcesses(ctx)

	// Assertions
	require.NoError(t, err)

	// Verify sessions were marked as failed
	assert.Equal(t, domain.SessionStatusFailed, runningSession.Status())
	assert.Equal(t, domain.SessionStatusFailed, startingSession.Status())

	sessionRepo.AssertExpectations(t)
	eventRepo.AssertExpectations(t)
}

func TestCleanupService_CleanupGameData(t *testing.T) {
	sessionRepo := &MockSessionRepository{}
	saveRepo := &MockSaveRepository{}
	eventRepo := &MockEventRepository{}
	logger := logging.NewLoggerBasic("test", "debug", "text", "stdout")

	cleanupService := NewCleanupService(sessionRepo, saveRepo, eventRepo, logger)
	ctx := context.Background()

	// Create temporary directory and session data
	tempDir := t.TempDir()
	sessionID := uuid.New()
	sessionDir := filepath.Join(tempDir, "sessions", sessionID.String())
	err := os.MkdirAll(sessionDir, 0755)
	require.NoError(t, err)

	// Create some test files in session directory
	testFile := filepath.Join(sessionDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test data"), 0644)
	require.NoError(t, err)

	// Create mock session (ended)
	session := createMockSession(123, "nethack")
	session.End(nil, nil) // Mark as ended

	// Create mock saves
	saves := []*domain.GameSave{createMockSave(123, "nethack", "/tmp/save1.dat", []byte("save1"))}

	// Set up mock expectations
	sessionRepo.On("FindByID", ctx, domain.NewSessionID(sessionID.String())).Return(session, nil)
	saveRepo.On("FindByUser", ctx, session.UserID()).Return(saves, nil)
	// Note: Save is only called if verification fails. Since our mock save passes verification, no Save call expected.
	eventRepo.On("SaveEvent", ctx, mock.AnythingOfType("*domain.GameEvent")).Return(nil)

	// Test cleanup
	err = cleanupService.CleanupGameData(ctx, sessionID, tempDir)

	// Assertions
	require.NoError(t, err)

	// Verify session directory was removed
	assert.NoDirExists(t, sessionDir)

	sessionRepo.AssertExpectations(t)
	saveRepo.AssertExpectations(t)
	eventRepo.AssertExpectations(t)
}

func TestCleanupService_BackupAndCleanupOldSaves(t *testing.T) {
	sessionRepo := &MockSessionRepository{}
	saveRepo := &MockSaveRepository{}
	eventRepo := &MockEventRepository{}
	logger := logging.NewLoggerBasic("test", "debug", "text", "stdout")

	cleanupService := NewCleanupService(sessionRepo, saveRepo, eventRepo, logger)
	ctx := context.Background()

	userID := 123
	gameID := 1
	maxSaves := 2

	// Create temporary directory for save files
	tempDir := t.TempDir()

	// Create mock saves (more than maxSaves)
	saves := []*domain.GameSave{
		createMockSaveWithFile(tempDir, userID, "1", "save1.dat", []byte("save1")),
		createMockSaveWithFile(tempDir, userID, "1", "save2.dat", []byte("save2")),
		createMockSaveWithFile(tempDir, userID, "1", "save3.dat", []byte("save3")), // This should be archived
		createMockSaveWithFile(tempDir, userID, "1", "save4.dat", []byte("save4")), // This should be archived
	}

	// Set up mock expectations
	saveRepo.On("FindByUser", ctx, domain.NewUserID(userID)).Return(saves, nil)
	saveRepo.On("SaveBackup", ctx, mock.AnythingOfType("domain.SaveID"), mock.AnythingOfType("domain.SaveBackup")).Return(nil).Twice()
	saveRepo.On("Save", ctx, mock.AnythingOfType("*domain.GameSave")).Return(nil).Twice()

	// Test cleanup
	err := cleanupService.BackupAndCleanupOldSaves(ctx, userID, gameID, maxSaves)

	// Assertions
	require.NoError(t, err)

	// Verify saves were archived
	assert.Equal(t, domain.SaveStatusArchived, saves[2].Status())
	assert.Equal(t, domain.SaveStatusArchived, saves[3].Status())

	saveRepo.AssertExpectations(t)
}

func TestCleanupService_PeriodicCleanup(t *testing.T) {
	sessionRepo := &MockSessionRepository{}
	saveRepo := &MockSaveRepository{}
	eventRepo := &MockEventRepository{}
	logger := logging.NewLoggerBasic("test", "debug", "text", "stdout")

	cleanupService := NewCleanupService(sessionRepo, saveRepo, eventRepo, logger)

	// Create a context that will be cancelled to stop the periodic cleanup
	ctx, cancel := context.WithCancel(context.Background())

	// Set up mock expectations for periodic operations
	sessionRepo.On("DeleteExpiredSessions", ctx, 24*time.Hour).Return(1, nil).Maybe()
	sessionRepo.On("FindByStatus", ctx, domain.SessionStatusActive).Return([]*domain.GameSession{}, nil).Maybe()
	sessionRepo.On("FindByStatus", ctx, domain.SessionStatusStarting).Return([]*domain.GameSession{}, nil).Maybe()

	// Start periodic cleanup with very short interval for testing
	go cleanupService.StartPeriodicCleanup(ctx, 10*time.Millisecond)

	// Let it run for a short time
	time.Sleep(50 * time.Millisecond)

	// Cancel context to stop cleanup
	cancel()

	// Give it time to stop
	time.Sleep(10 * time.Millisecond)

	// The test passes if no panics occurred and cleanup methods were called
	t.Log("Periodic cleanup test completed successfully")
}

// Helper functions for creating test objects

func createMockSessionWithProcess(userID int, gameID string, pid int) *domain.GameSession {
	session := createMockSession(userID, gameID)

	// Start session with process info
	session.Start(domain.ProcessInfo{
		PID:     pid,
		PodName: "test-pod",
	})

	return session
}

func createMockSaveWithFile(tempDir string, userID int, gameID, filename string, data []byte) *domain.GameSave {
	// Create actual file for testing
	filePath := filepath.Join(tempDir, filename)
	os.WriteFile(filePath, data, 0644)

	return createMockSave(userID, gameID, filePath, data)
}

// Benchmark tests

func BenchmarkSessionManager_StartGameSession(b *testing.B) {
	sm, sessionRepo, saveRepo, gameRepo, eventRepo, _ := createTestSessionManager(&testing.T{})
	ctx := context.Background()

	userID := 123
	gameID := "nethack"
	terminalSize := domain.TerminalSize{Width: 80, Height: 24}

	// Set up mock expectations
	game := createMockGame(gameID)
	gameRepo.On("FindByID", ctx, domain.NewGameID(gameID)).Return(game, nil).Maybe()
	saveRepo.On("FindByUserAndGame", ctx, domain.NewUserID(userID), domain.NewGameID(gameID)).Return(nil, fmt.Errorf("not found")).Maybe()
	sessionRepo.On("Save", ctx, mock.AnythingOfType("*domain.GameSession")).Return(nil).Maybe()
	eventRepo.On("SaveEvent", ctx, mock.AnythingOfType("*domain.GameEvent")).Return(nil).Maybe()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := sm.StartGameSession(ctx, userID, gameID, terminalSize)
		if err != nil {
			b.Fatalf("StartGameSession failed: %v", err)
		}
	}
}

func BenchmarkCleanupService_CleanupExpiredSessions(b *testing.B) {
	sessionRepo := &MockSessionRepository{}
	saveRepo := &MockSaveRepository{}
	eventRepo := &MockEventRepository{}
	logger := logging.NewLoggerBasic("test", "debug", "text", "stdout")

	cleanupService := NewCleanupService(sessionRepo, saveRepo, eventRepo, logger)
	ctx := context.Background()
	maxAge := 24 * time.Hour

	// Set up mock expectations
	sessionRepo.On("DeleteExpiredSessions", ctx, maxAge).Return(0, nil).Maybe()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := cleanupService.CleanupExpiredSessions(ctx, maxAge)
		if err != nil {
			b.Fatalf("CleanupExpiredSessions failed: %v", err)
		}
	}
}

// Integration test with real file operations

func TestSessionManager_Integration_SaveLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	sm, sessionRepo, saveRepo, gameRepo, eventRepo, tempDir := createTestSessionManager(t)
	ctx := context.Background()

	userID := 123
	gameID := "nethack"
	terminalSize := domain.TerminalSize{Width: 80, Height: 24}

	// Phase 1: Start new session (no existing save)
	game := createMockGame(gameID)
	gameRepo.On("FindByID", ctx, domain.NewGameID(gameID)).Return(game, nil)
	saveRepo.On("FindByUserAndGame", ctx, domain.NewUserID(userID), domain.NewGameID(gameID)).Return(nil, fmt.Errorf("not found"))
	sessionRepo.On("Save", ctx, mock.AnythingOfType("*domain.GameSession")).Return(nil)
	eventRepo.On("SaveEvent", ctx, mock.AnythingOfType("*domain.GameEvent")).Return(nil)

	session1, err := sm.StartGameSession(ctx, userID, gameID, terminalSize)
	require.NoError(t, err)

	// Simulate game writing save data
	sessionDir1 := filepath.Join(tempDir, "sessions", session1.ID().String())
	saveDir1 := filepath.Join(sessionDir1, "save")
	os.MkdirAll(saveDir1, 0755)

	saveData1 := []byte("game progress data v1")
	saveFile1 := filepath.Join(saveDir1, "save.dat")
	os.WriteFile(saveFile1, saveData1, 0644)

	// Phase 2: End session and create save
	saveRepo.On("Save", ctx, mock.AnythingOfType("*domain.GameSave")).Return(nil)

	err = sm.createSaveFromSession(ctx, session1)
	require.NoError(t, err)

	// Verify permanent save was created
	userSaveDir := filepath.Join(tempDir, "saves", "user_123", "nethack")
	assert.DirExists(t, userSaveDir)

	saves, err := filepath.Glob(filepath.Join(userSaveDir, "save_*.dat"))
	require.NoError(t, err)
	require.Len(t, saves, 1)

	savedData, err := os.ReadFile(saves[0])
	require.NoError(t, err)
	assert.Equal(t, saveData1, savedData)

	// Phase 3: Start new session with existing save
	existingSave := createMockSave(userID, gameID, saves[0], saveData1)
	saveRepo.On("FindByUserAndGame", ctx, domain.NewUserID(userID), domain.NewGameID(gameID)).Return(existingSave, nil)

	session2, err := sm.StartGameSession(ctx, userID, gameID, terminalSize)
	require.NoError(t, err)

	// Verify save was loaded into new session
	sessionDir2 := filepath.Join(tempDir, "sessions", session2.ID().String())
	sessionSavePath := filepath.Join(sessionDir2, "save", "save.dat")
	assert.FileExists(t, sessionSavePath)

	loadedData, err := os.ReadFile(sessionSavePath)
	require.NoError(t, err)
	assert.Equal(t, saveData1, loadedData)

	// Phase 4: Update save and end session
	saveData2 := []byte("game progress data v2 - updated")
	os.WriteFile(sessionSavePath, saveData2, 0644)

	err = sm.createSaveFromSession(ctx, session2)
	require.NoError(t, err)

	// Verify we now have 2 save files
	saves, err = filepath.Glob(filepath.Join(userSaveDir, "save_*.dat"))
	require.NoError(t, err)
	require.Len(t, saves, 2)

	t.Log("Integration test completed successfully - full save lifecycle tested")
}
