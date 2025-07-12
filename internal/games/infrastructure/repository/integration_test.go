package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dungeongate/internal/games/domain"
	"github.com/dungeongate/pkg/config"
	"github.com/dungeongate/pkg/database"
)

// Test database types
const (
	DBTypeSQLite     = "sqlite"
	DBTypePostgreSQL = "postgresql"
)

// getTestDBType returns the database type to use for testing
// Set TEST_DB_TYPE=postgresql and TEST_DB_DSN=<connection-string> to test with PostgreSQL
func getTestDBType() string {
	dbType := os.Getenv("TEST_DB_TYPE")
	if dbType == "" {
		dbType = DBTypeSQLite // Default to SQLite for local testing
	}
	return dbType
}

// convertQueryParams converts PostgreSQL-style placeholders ($1, $2) to SQLite style (?, ?)
func convertQueryParams(query string, dbType string) string {
	if dbType == DBTypeSQLite {
		// Simple conversion - this works for sequential parameters
		for i := 20; i >= 1; i-- {
			query = strings.ReplaceAll(query, fmt.Sprintf("$%d", i), "?")
		}
	}
	return query
}

// Integration test setup
func setupTestDB(t *testing.T) (*database.Connection, string, func()) {
	dbType := getTestDBType()

	t.Logf("=== INTEGRATION TEST DATABASE CONFIGURATION ===")
	t.Logf("Database Type: %s", dbType)

	switch dbType {
	case DBTypeSQLite:
		t.Logf("Using SQLite for testing (local/CI mode)")
		t.Logf("Note: Some tests may be skipped due to SQL dialect differences")
		return setupSQLiteDB(t)
	case DBTypePostgreSQL:
		t.Logf("Using PostgreSQL for testing (full compatibility mode)")
		t.Logf("Ensure PostgreSQL is running and TEST_DB_DSN is set")
		return setupPostgreSQLDB(t)
	default:
		t.Fatalf("Unsupported test database type: %s", dbType)
		return nil, "", nil
	}
}

func setupSQLiteDB(t *testing.T) (*database.Connection, string, func()) {
	// Create temporary database file
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	// Create database connection using legacy config for simplicity
	legacyConfig := &config.LegacyDatabaseConfig{
		Type: "sqlite",
		Connection: map[string]interface{}{
			"dsn": dbPath,
		},
	}

	db, err := database.NewConnectionFromLegacy(legacyConfig)
	require.NoError(t, err)

	// Create tables
	err = createTestTablesSQLite(db)
	require.NoError(t, err)

	cleanup := func() {
		db.Close()
		os.Remove(dbPath)
	}

	return db, DBTypeSQLite, cleanup
}

func setupPostgreSQLDB(t *testing.T) (*database.Connection, string, func()) {
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Skip("TEST_DB_DSN not set, skipping PostgreSQL tests")
	}

	// Create database connection
	legacyConfig := &config.LegacyDatabaseConfig{
		Type: "postgresql",
		Connection: map[string]interface{}{
			"dsn": dsn,
		},
	}

	db, err := database.NewConnectionFromLegacy(legacyConfig)
	require.NoError(t, err)

	// Create tables
	err = createTestTablesPostgreSQL(db)
	require.NoError(t, err)

	cleanup := func() {
		// Clean up test data
		db.ExecContext(context.Background(), "DROP TABLE IF EXISTS game_save_backups CASCADE")
		db.ExecContext(context.Background(), "DROP TABLE IF EXISTS game_saves CASCADE")
		db.ExecContext(context.Background(), "DROP TABLE IF EXISTS game_sessions CASCADE")
		db.ExecContext(context.Background(), "DROP TABLE IF EXISTS games CASCADE")
		db.ExecContext(context.Background(), "DROP TABLE IF EXISTS users CASCADE")
		db.Close()
	}

	return db, DBTypePostgreSQL, cleanup
}

func createTestTablesSQLite(db *database.Connection) error {
	// Enable foreign keys for SQLite
	_, err := db.DB(database.QueryTypeWrite).ExecContext(context.Background(), "PRAGMA foreign_keys = ON")
	if err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	schema := `
	-- Users table (required for foreign keys)
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username VARCHAR(255) NOT NULL UNIQUE,
		email VARCHAR(255) NOT NULL UNIQUE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- Games table
	CREATE TABLE IF NOT EXISTS games (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name VARCHAR(50) NOT NULL UNIQUE,
		display_name VARCHAR(100) NOT NULL,
		description TEXT,
		executable_path VARCHAR(255) NOT NULL,
		version VARCHAR(20),
		min_terminal_width INTEGER DEFAULT 80,
		min_terminal_height INTEGER DEFAULT 24,
		max_players INTEGER DEFAULT 1,
		supports_saves BOOLEAN DEFAULT true,
		supports_spectating BOOLEAN DEFAULT true,
		config TEXT DEFAULT '{}',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- Game sessions table
	CREATE TABLE IF NOT EXISTS game_sessions (
		id TEXT PRIMARY KEY,
		game_id TEXT NOT NULL,
		user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		status VARCHAR(20) NOT NULL DEFAULT 'starting',
		process_id INTEGER,
		pod_id VARCHAR(255),
		started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		ended_at TIMESTAMP,
		last_activity TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		terminal_width INTEGER DEFAULT 80,
		terminal_height INTEGER DEFAULT 24,
		resource_usage TEXT DEFAULT '{}',
		metadata TEXT DEFAULT '{}'
	);

	-- Game saves table
	CREATE TABLE IF NOT EXISTS game_saves (
		id TEXT PRIMARY KEY,
		user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		game_id TEXT NOT NULL,
		session_id TEXT REFERENCES game_sessions(id) ON DELETE SET NULL,
		name VARCHAR(255) NOT NULL,
		description TEXT,
		file_path VARCHAR(500) NOT NULL,
		file_size BIGINT DEFAULT 0,
		checksum VARCHAR(64),
		version INTEGER DEFAULT 1,
		is_active BOOLEAN DEFAULT true,
		metadata TEXT DEFAULT '{}',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- Game save backups table
	CREATE TABLE IF NOT EXISTS game_save_backups (
		id TEXT PRIMARY KEY,
		save_id TEXT NOT NULL REFERENCES game_saves(id) ON DELETE CASCADE,
		backup_number INTEGER NOT NULL,
		file_path VARCHAR(500) NOT NULL,
		file_size BIGINT DEFAULT 0,
		checksum VARCHAR(64),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- Insert test data
	INSERT OR IGNORE INTO users (id, username, email) VALUES 
		(1, 'testuser1', 'test1@example.com'),
		(2, 'testuser2', 'test2@example.com');

	INSERT OR IGNORE INTO games (id, name, display_name, description, executable_path) VALUES 
		(1, 'nethack', 'NetHack', 'Classic roguelike game', '/usr/games/nethack'),
		(2, 'dcss', 'Dungeon Crawl Stone Soup', 'Modern roguelike game', '/usr/games/crawl');
	`

	_, err = db.DB(database.QueryTypeWrite).ExecContext(context.Background(), schema)
	return err
}

func createTestTablesPostgreSQL(db *database.Connection) error {
	schema := `
	-- Users table (required for foreign keys)
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		username VARCHAR(255) NOT NULL UNIQUE,
		email VARCHAR(255) NOT NULL UNIQUE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- Games table
	CREATE TABLE IF NOT EXISTS games (
		id SERIAL PRIMARY KEY,
		name VARCHAR(50) NOT NULL UNIQUE,
		display_name VARCHAR(100) NOT NULL,
		description TEXT,
		executable_path VARCHAR(255) NOT NULL,
		version VARCHAR(20),
		min_terminal_width INTEGER DEFAULT 80,
		min_terminal_height INTEGER DEFAULT 24,
		max_players INTEGER DEFAULT 1,
		supports_saves BOOLEAN DEFAULT true,
		supports_spectating BOOLEAN DEFAULT true,
		config TEXT DEFAULT '{}',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- Game sessions table
	CREATE TABLE IF NOT EXISTS game_sessions (
		id TEXT PRIMARY KEY,
		game_id TEXT NOT NULL,
		user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		status VARCHAR(20) NOT NULL DEFAULT 'starting',
		process_id INTEGER,
		pod_id VARCHAR(255),
		started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		ended_at TIMESTAMP,
		last_activity TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		terminal_width INTEGER DEFAULT 80,
		terminal_height INTEGER DEFAULT 24,
		resource_usage JSONB DEFAULT '{}',
		metadata JSONB DEFAULT '{}'
	);

	-- Game saves table
	CREATE TABLE IF NOT EXISTS game_saves (
		id TEXT PRIMARY KEY,
		user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		game_id TEXT NOT NULL,
		session_id TEXT REFERENCES game_sessions(id) ON DELETE SET NULL,
		name VARCHAR(255) NOT NULL,
		description TEXT,
		file_path VARCHAR(500) NOT NULL,
		file_size BIGINT DEFAULT 0,
		checksum VARCHAR(64),
		version INTEGER DEFAULT 1,
		is_active BOOLEAN DEFAULT true,
		metadata JSONB DEFAULT '{}',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- Game save backups table
	CREATE TABLE IF NOT EXISTS game_save_backups (
		id TEXT PRIMARY KEY,
		save_id TEXT NOT NULL REFERENCES game_saves(id) ON DELETE CASCADE,
		backup_number INTEGER NOT NULL,
		file_path VARCHAR(500) NOT NULL,
		file_size BIGINT DEFAULT 0,
		checksum VARCHAR(64),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- Insert test data
	INSERT INTO users (id, username, email) VALUES 
		(1, 'testuser1', 'test1@example.com'),
		(2, 'testuser2', 'test2@example.com')
	ON CONFLICT DO NOTHING;

	INSERT INTO games (id, name, display_name, description, executable_path) VALUES 
		(1, 'nethack', 'NetHack', 'Classic roguelike game', '/usr/games/nethack'),
		(2, 'dcss', 'Dungeon Crawl Stone Soup', 'Modern roguelike game', '/usr/games/crawl')
	ON CONFLICT DO NOTHING;
	`

	_, err := db.DB(database.QueryTypeWrite).ExecContext(context.Background(), schema)
	return err
}

// Test SessionRepository integration

func TestSessionRepository_Integration_CRUD(t *testing.T) {
	db, dbType, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewPostgreSQLSessionRepository(db)
	ctx := context.Background()

	// Create test session
	sessionID := domain.NewSessionID(uuid.New().String())
	userID := domain.NewUserID(1)
	gameID := domain.NewGameID("1")
	terminalSize := domain.TerminalSize{Width: 120, Height: 40}

	session := domain.NewGameSession(sessionID, userID, "testuser1", gameID, domain.GameConfig{}, terminalSize)

	// Test Create
	err := repo.Create(ctx, session)
	require.NoError(t, err)

	// Test FindByID
	foundSession, err := repo.FindByID(ctx, uuid.MustParse(sessionID.String()))
	require.NoError(t, err)
	require.NotNil(t, foundSession)
	assert.Equal(t, sessionID.String(), foundSession.ID().String())
	assert.Equal(t, userID.Int(), foundSession.UserID().Int())

	// Test Update
	if dbType == DBTypeSQLite {
		t.Log("SKIPPING Update test for SQLite - parameter placeholder incompatibility")
		t.Log("SQLite uses ? placeholders while repository uses PostgreSQL $1, $2 syntax")
	} else {
		// For PostgreSQL, the Update should work
		foundSession.Start(domain.ProcessInfo{PID: 12345, PodName: "test-pod"})
		err = repo.Update(ctx, foundSession)
		require.NoError(t, err)

		// Verify update
		updatedSession, err := repo.FindByID(ctx, uuid.MustParse(sessionID.String()))
		require.NoError(t, err)
		assert.Equal(t, 12345, updatedSession.ProcessInfo().PID)
	}

	// Test FindByUserID
	userSessions, err := repo.FindByUserID(ctx, 1)
	require.NoError(t, err)
	assert.Len(t, userSessions, 1)

	// Test FindByStatus - new sessions start with "starting" status
	startingSessions, err := repo.FindByStatus(ctx, "starting")
	require.NoError(t, err)
	assert.Len(t, startingSessions, 1)

	// Test UpdateActivity
	err = repo.UpdateActivity(ctx, uuid.MustParse(sessionID.String()))
	require.NoError(t, err)

	// Test Delete
	err = repo.Delete(ctx, uuid.MustParse(sessionID.String()))
	require.NoError(t, err)

	// Verify deletion
	_, err = repo.FindByID(ctx, uuid.MustParse(sessionID.String()))
	assert.Error(t, err)
}

func TestSessionRepository_Integration_DeleteExpiredSessions(t *testing.T) {
	db, dbType, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewPostgreSQLSessionRepository(db)
	ctx := context.Background()

	// Create test sessions with different end times
	oldSessionID := uuid.New()
	recentSessionID := uuid.New()

	// Insert old session directly into database
	oldEndTime := time.Now().Add(-48 * time.Hour)

	sessionQuery := convertQueryParams(`
		INSERT INTO game_sessions (id, game_id, user_id, status, ended_at)
		VALUES ($1, '1', 1, 'stopped', $2)
	`, dbType)

	_, err := db.DB(database.QueryTypeWrite).ExecContext(ctx, sessionQuery, oldSessionID.String(), oldEndTime)
	require.NoError(t, err)

	// Insert recent session
	recentEndTime := time.Now().Add(-1 * time.Hour)
	_, err = db.DB(database.QueryTypeWrite).ExecContext(ctx, sessionQuery, recentSessionID.String(), recentEndTime)
	require.NoError(t, err)

	// Test cleanup with 24 hour expiration
	deletedCount, err := repo.DeleteExpiredSessions(ctx, 24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 1, deletedCount)

	// Verify only old session was deleted
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM game_sessions").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

// Test SaveRepository integration

func TestSaveRepository_Integration_CRUD(t *testing.T) {
	db, dbType, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewPostgreSQLSaveRepository(db)
	ctx := context.Background()

	// Create test save
	saveID := domain.NewSaveID(uuid.New().String())
	userID := domain.NewUserID(1)
	gameID := domain.NewGameID("1")

	saveData := []byte("test save data")
	filePath := "/tmp/test_save.dat"
	metadata := domain.SaveMetadata{
		GameVersion: "1.0",
		Character:   "TestHero",
		Level:       5,
		Score:       1000,
		PlayTime:    2 * time.Hour,
		Location:    "Test Dungeon",
	}

	save := domain.NewGameSave(saveID, userID, gameID, saveData, filePath, metadata)

	// Test Create
	err := repo.Create(ctx, save)
	require.NoError(t, err)

	// Test FindByID
	foundSave, err := repo.FindByID(ctx, uuid.MustParse(saveID.String()))
	require.NoError(t, err)
	require.NotNil(t, foundSave)
	assert.Equal(t, saveID.String(), foundSave.ID().String())
	assert.Equal(t, userID.Int(), foundSave.UserID().Int())
	assert.Equal(t, filePath, foundSave.FilePath())

	// Test Update
	if dbType == DBTypeSQLite {
		t.Log("SKIPPING Update test for SQLite - parameter placeholder incompatibility")
	} else {
		err = repo.Update(ctx, save)
		require.NoError(t, err)
	}

	// Test FindByUserAndGame
	userSaves, err := repo.FindByUserAndGame(ctx, 1, 1)
	require.NoError(t, err)
	assert.Len(t, userSaves, 1)

	// Test CreateBackup
	backupPath := "/tmp/test_save.bak"
	err = repo.CreateBackup(ctx, uuid.MustParse(saveID.String()), backupPath, 100, "abc123")
	require.NoError(t, err)

	// Test ListBackups
	backups, err := repo.ListBackups(ctx, uuid.MustParse(saveID.String()))
	require.NoError(t, err)
	assert.Len(t, backups, 1)
	assert.Equal(t, backupPath, backups[0].FilePath)

	// Test Delete
	err = repo.Delete(ctx, uuid.MustParse(saveID.String()))
	require.NoError(t, err)

	// Verify deletion
	_, err = repo.FindByID(ctx, uuid.MustParse(saveID.String()))
	assert.Error(t, err)
}

func TestSaveRepository_Integration_BackupManagement(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewPostgreSQLSaveRepository(db)
	ctx := context.Background()

	// Create test save
	saveID := uuid.New()

	// Insert save directly into database
	_, err := db.ExecContext(ctx, `
		INSERT INTO game_saves (id, user_id, game_id, name, file_path, file_size, checksum, metadata)
		VALUES (?, 1, 1, 'test_save', '/tmp/test.dat', 100, 'abc123', '{}')
	`, saveID.String())
	require.NoError(t, err)

	// Create multiple backups
	for i := 1; i <= 5; i++ {
		backupPath := fmt.Sprintf("/tmp/backup_%d.dat", i)
		err = repo.CreateBackup(ctx, saveID, backupPath, int64(100+i), fmt.Sprintf("checksum_%d", i))
		require.NoError(t, err)
	}

	// Verify all backups exist
	backups, err := repo.ListBackups(ctx, saveID)
	require.NoError(t, err)
	assert.Len(t, backups, 5)

	// Test DeleteOldBackups (keep only 3 most recent)
	err = repo.DeleteOldBackups(ctx, saveID, 3)
	require.NoError(t, err)

	// Verify only 3 backups remain
	remainingBackups, err := repo.ListBackups(ctx, saveID)
	require.NoError(t, err)
	assert.Len(t, remainingBackups, 3)

	// Verify the remaining backups are the most recent ones (highest backup numbers)
	for _, backup := range remainingBackups {
		assert.GreaterOrEqual(t, backup.BackupNumber, 3)
	}
}

// Performance benchmarks

func BenchmarkSessionRepository_Create(b *testing.B) {
	db, _, cleanup := setupTestDB(&testing.T{})
	defer cleanup()

	repo := NewPostgreSQLSessionRepository(db)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sessionID := domain.NewSessionID(uuid.New().String())
		userID := domain.NewUserID(1)
		gameID := domain.NewGameID("1")
		terminalSize := domain.TerminalSize{Width: 80, Height: 24}

		session := domain.NewGameSession(sessionID, userID, "testuser", gameID, domain.GameConfig{}, terminalSize)

		err := repo.Create(ctx, session)
		if err != nil {
			b.Fatalf("Create failed: %v", err)
		}
	}
}

func BenchmarkSaveRepository_Create(b *testing.B) {
	db, _, cleanup := setupTestDB(&testing.T{})
	defer cleanup()

	repo := NewPostgreSQLSaveRepository(db)
	ctx := context.Background()

	saveData := make([]byte, 1024) // 1KB save data
	for i := range saveData {
		saveData[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		saveID := domain.NewSaveID(uuid.New().String())
		userID := domain.NewUserID(1)
		gameID := domain.NewGameID("1")

		metadata := domain.SaveMetadata{
			GameVersion: "1.0",
			Character:   fmt.Sprintf("Hero_%d", i),
			Level:       i % 100,
			Score:       i * 100,
			PlayTime:    time.Duration(i) * time.Minute,
			Location:    "Benchmark Dungeon",
		}

		save := domain.NewGameSave(saveID, userID, gameID, saveData, fmt.Sprintf("/tmp/save_%d.dat", i), metadata)

		err := repo.Create(ctx, save)
		if err != nil {
			b.Fatalf("Create failed: %v", err)
		}
	}
}

// Test concurrent operations

func TestSessionRepository_Concurrent_Operations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewPostgreSQLSessionRepository(db)
	ctx := context.Background()

	// Test concurrent session creation
	const numGoroutines = 10
	const sessionsPerGoroutine = 5

	errChan := make(chan error, numGoroutines)

	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			for i := 0; i < sessionsPerGoroutine; i++ {
				sessionID := domain.NewSessionID(uuid.New().String())
				userID := domain.NewUserID(1)
				gameID := domain.NewGameID("1")
				terminalSize := domain.TerminalSize{Width: 80, Height: 24}

				session := domain.NewGameSession(sessionID, userID, fmt.Sprintf("user_%d_%d", goroutineID, i), gameID, domain.GameConfig{}, terminalSize)

				err := repo.Create(ctx, session)
				if err != nil {
					errChan <- fmt.Errorf("goroutine %d, session %d: %w", goroutineID, i, err)
					return
				}
			}
			errChan <- nil
		}(g)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		err := <-errChan
		require.NoError(t, err)
	}

	// Verify all sessions were created
	var count int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM game_sessions").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, numGoroutines*sessionsPerGoroutine, count)
}

// Test edge cases and error conditions

func TestSessionRepository_EdgeCases(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewPostgreSQLSessionRepository(db)
	ctx := context.Background()

	// Test finding non-existent session
	nonExistentID := uuid.New()
	_, err := repo.FindByID(ctx, nonExistentID)
	assert.Error(t, err)

	// Test updating non-existent session
	sessionID := domain.NewSessionID(uuid.New().String())
	userID := domain.NewUserID(1)
	gameID := domain.NewGameID("1")
	terminalSize := domain.TerminalSize{Width: 80, Height: 24}

	session := domain.NewGameSession(sessionID, userID, "testuser", gameID, domain.GameConfig{}, terminalSize)
	err = repo.Update(ctx, session)
	assert.Error(t, err)

	// Test deleting non-existent session
	err = repo.Delete(ctx, nonExistentID)
	assert.Error(t, err)
}

func TestSaveRepository_EdgeCases(t *testing.T) {
	db, dbType, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewPostgreSQLSaveRepository(db)
	ctx := context.Background()

	// Test finding non-existent save
	nonExistentID := uuid.New()
	_, err := repo.FindByID(ctx, nonExistentID)
	assert.Error(t, err)

	// Test creating backup for non-existent save
	if dbType == DBTypeSQLite {
		t.Log("SKIPPING CreateBackup error test for SQLite - different constraint handling")
		// SQLite may not enforce foreign key constraints the same way
	} else {
		err = repo.CreateBackup(ctx, nonExistentID, "/tmp/backup.dat", 100, "checksum")
		assert.Error(t, err)
	}

	// Test listing backups for non-existent save
	backups, err := repo.ListBackups(ctx, nonExistentID)
	require.NoError(t, err)
	assert.Len(t, backups, 0)
}
