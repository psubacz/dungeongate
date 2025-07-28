package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dungeongate/internal/games/domain"
	"github.com/dungeongate/pkg/database"
	"github.com/google/uuid"
)

type PostgreSQLSessionRepository struct {
	db *database.Connection
}

func NewPostgreSQLSessionRepository(db *database.Connection) *PostgreSQLSessionRepository {
	return &PostgreSQLSessionRepository{db: db}
}

func (r *PostgreSQLSessionRepository) Save(ctx context.Context, session *domain.GameSession) error {
	// Check if session exists, if so update, otherwise create
	existing, err := r.FindByID(ctx, uuid.MustParse(session.ID().String()))
	if err != nil {
		// Session doesn't exist, create it
		return r.Create(ctx, session)
	}
	if existing != nil {
		// Session exists, update it
		return r.Update(ctx, session)
	}
	return r.Create(ctx, session)
}

func (r *PostgreSQLSessionRepository) Create(ctx context.Context, session *domain.GameSession) error {
	// Convert domain types to database types
	var processID sql.NullInt32
	var podID sql.NullString
	var endedAt sql.NullTime

	if session.ProcessInfo().PID != 0 {
		processID = sql.NullInt32{Int32: int32(session.ProcessInfo().PID), Valid: true}
	}
	if session.ProcessInfo().PodName != "" {
		podID = sql.NullString{String: session.ProcessInfo().PodName, Valid: true}
	}
	if session.EndTime() != nil {
		endedAt = sql.NullTime{Time: *session.EndTime(), Valid: true}
	}

	// Create minimal metadata (the domain doesn't expose direct metadata access)
	metadata := map[string]interface{}{
		"encoding": "utf-8",
		"features": map[string]bool{
			"recording":  session.RecordingInfo() != nil,
			"streaming":  session.StreamingInfo() != nil,
			"spectating": session.CanSpectate(),
		},
	}
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Create minimal resource usage tracking
	resourceUsage := map[string]interface{}{
		"spectator_count": session.SpectatorCount(),
		"duration":        session.Duration().Seconds(),
	}
	resourceUsageBytes, err := json.Marshal(resourceUsage)
	if err != nil {
		return fmt.Errorf("failed to marshal resource usage: %w", err)
	}

	query := `
		INSERT INTO game_sessions (
			id, game_id, user_id, status, process_id, pod_id,
			started_at, ended_at, last_activity, terminal_width, terminal_height,
			resource_usage, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	_, err = r.db.DB(database.QueryTypeWrite).ExecContext(ctx, query,
		session.ID().String(),
		session.GameID().String(),
		session.UserID().Int(),
		string(session.Status()),
		processID,
		podID,
		session.StartTime(),
		endedAt,
		session.StartTime(), // Use start time as initial last activity
		session.TerminalSize().Width,
		session.TerminalSize().Height,
		resourceUsageBytes,
		metadataBytes,
	)

	if err != nil {
		return fmt.Errorf("failed to create game session: %w", err)
	}

	return nil
}

func (r *PostgreSQLSessionRepository) Update(ctx context.Context, session *domain.GameSession) error {
	// Convert domain types to database types
	var processID sql.NullInt32
	var podID sql.NullString
	var endedAt sql.NullTime

	if session.ProcessInfo().PID != 0 {
		processID = sql.NullInt32{Int32: int32(session.ProcessInfo().PID), Valid: true}
	}
	if session.ProcessInfo().PodName != "" {
		podID = sql.NullString{String: session.ProcessInfo().PodName, Valid: true}
	}
	if session.EndTime() != nil {
		endedAt = sql.NullTime{Time: *session.EndTime(), Valid: true}
	}

	// Create minimal metadata
	metadata := map[string]interface{}{
		"encoding": "utf-8",
		"features": map[string]bool{
			"recording":  session.RecordingInfo() != nil,
			"streaming":  session.StreamingInfo() != nil,
			"spectating": session.CanSpectate(),
		},
	}
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Create minimal resource usage tracking
	resourceUsage := map[string]interface{}{
		"spectator_count": session.SpectatorCount(),
		"duration":        session.Duration().Seconds(),
	}
	resourceUsageBytes, err := json.Marshal(resourceUsage)
	if err != nil {
		return fmt.Errorf("failed to marshal resource usage: %w", err)
	}

	query := `
		UPDATE game_sessions SET
			status = $2,
			process_id = $3,
			pod_id = $4,
			ended_at = $5,
			last_activity = $6,
			terminal_width = $7,
			terminal_height = $8,
			resource_usage = $9,
			metadata = $10
		WHERE id = $1
	`

	result, err := r.db.DB(database.QueryTypeWrite).ExecContext(ctx, query,
		session.ID().String(),
		string(session.Status()),
		processID,
		podID,
		endedAt,
		time.Now(), // Update last activity to now
		session.TerminalSize().Width,
		session.TerminalSize().Height,
		resourceUsageBytes,
		metadataBytes,
	)

	if err != nil {
		return fmt.Errorf("failed to update game session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("game session not found: %s", session.ID().String())
	}

	return nil
}

func (r *PostgreSQLSessionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM game_sessions WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id.String())
	if err != nil {
		return fmt.Errorf("failed to delete game session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("game session not found: %s", id.String())
	}

	return nil
}

func (r *PostgreSQLSessionRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.GameSession, error) {
	query := `
		SELECT id, game_id, user_id, status, process_id, pod_id,
			started_at, ended_at, last_activity, terminal_width, terminal_height,
			resource_usage, metadata
		FROM game_sessions
		WHERE id = $1
	`

	row := r.db.DB(database.QueryTypeRead).QueryRowContext(ctx, query, id.String())
	return r.scanSession(row)
}

func (r *PostgreSQLSessionRepository) FindByUserID(ctx context.Context, userID int) ([]*domain.GameSession, error) {
	query := `
		SELECT id, game_id, user_id, status, process_id, pod_id,
			started_at, ended_at, last_activity, terminal_width, terminal_height,
			resource_usage, metadata
		FROM game_sessions
		WHERE user_id = $1
		ORDER BY started_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*domain.GameSession
	for rows.Next() {
		session, err := r.scanSessionFromRows(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return sessions, nil
}

func (r *PostgreSQLSessionRepository) FindActiveByUserID(ctx context.Context, userID int) ([]*domain.GameSession, error) {
	query := `
		SELECT id, game_id, user_id, status, process_id, pod_id,
			started_at, ended_at, last_activity, terminal_width, terminal_height,
			resource_usage, metadata
		FROM game_sessions
		WHERE user_id = $1 AND status IN ('starting', 'running', 'paused')
		ORDER BY last_activity DESC
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query active sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*domain.GameSession
	for rows.Next() {
		session, err := r.scanSessionFromRows(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return sessions, nil
}

func (r *PostgreSQLSessionRepository) FindByStatus(ctx context.Context, status string) ([]*domain.GameSession, error) {
	query := `
		SELECT id, game_id, user_id, status, process_id, pod_id,
			started_at, ended_at, last_activity, terminal_width, terminal_height,
			resource_usage, metadata
		FROM game_sessions
		WHERE status = $1
		ORDER BY last_activity DESC
	`

	rows, err := r.db.QueryContext(ctx, query, status)
	if err != nil {
		return nil, fmt.Errorf("failed to query sessions by status: %w", err)
	}
	defer rows.Close()

	var sessions []*domain.GameSession
	for rows.Next() {
		session, err := r.scanSessionFromRows(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return sessions, nil
}

func (r *PostgreSQLSessionRepository) UpdateActivity(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE game_sessions SET last_activity = $2 WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id.String(), time.Now())
	if err != nil {
		return fmt.Errorf("failed to update session activity: %w", err)
	}
	return nil
}

func (r *PostgreSQLSessionRepository) DeleteExpiredSessions(ctx context.Context, maxAge time.Duration) (int, error) {
	expirationTime := time.Now().Add(-maxAge)
	query := `
		DELETE FROM game_sessions 
		WHERE status IN ('stopped', 'crashed', 'terminated') 
		AND ended_at < $1
	`

	result, err := r.db.ExecContext(ctx, query, expirationTime)
	if err != nil {
		return 0, fmt.Errorf("failed to delete expired sessions: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return int(rowsAffected), nil
}

func (r *PostgreSQLSessionRepository) scanSession(row *sql.Row) (*domain.GameSession, error) {
	var sessionID, gameID string
	var userID int
	var status string
	var processID sql.NullInt32
	var podID sql.NullString
	var startedAt, lastActivity time.Time
	var endedAt sql.NullTime
	var terminalWidth, terminalHeight int
	var resourceUsage, metadata []byte

	err := row.Scan(
		&sessionID,
		&gameID,
		&userID,
		&status,
		&processID,
		&podID,
		&startedAt,
		&endedAt,
		&lastActivity,
		&terminalWidth,
		&terminalHeight,
		&resourceUsage,
		&metadata,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("game session not found")
		}
		return nil, fmt.Errorf("failed to scan game session: %w", err)
	}

	return r.buildDomainSession(sessionID, gameID, userID, status, processID, podID, startedAt, endedAt, terminalWidth, terminalHeight)
}

func (r *PostgreSQLSessionRepository) scanSessionFromRows(rows *sql.Rows) (*domain.GameSession, error) {
	var sessionID, gameID string
	var userID int
	var status string
	var processID sql.NullInt32
	var podID sql.NullString
	var startedAt, lastActivity time.Time
	var endedAt sql.NullTime
	var terminalWidth, terminalHeight int
	var resourceUsage, metadata []byte

	err := rows.Scan(
		&sessionID,
		&gameID,
		&userID,
		&status,
		&processID,
		&podID,
		&startedAt,
		&endedAt,
		&lastActivity,
		&terminalWidth,
		&terminalHeight,
		&resourceUsage,
		&metadata,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to scan game session: %w", err)
	}

	return r.buildDomainSession(sessionID, gameID, userID, status, processID, podID, startedAt, endedAt, terminalWidth, terminalHeight)
}

func (r *PostgreSQLSessionRepository) buildDomainSession(sessionID, gameID string, userID int, status string, processID sql.NullInt32, podID sql.NullString, startedAt time.Time, endedAt sql.NullTime, terminalWidth, terminalHeight int) (*domain.GameSession, error) {
	// Build domain session - this is a simplified reconstruction
	// In a real implementation, you'd need to store more domain data or reconstruct it properly
	session := domain.NewGameSession(
		domain.NewSessionID(sessionID),
		domain.NewUserID(userID),
		"username", // This would need to be retrieved from user service
		domain.NewGameID(gameID),
		domain.GameConfig{}, // This would need to be retrieved
		domain.TerminalSize{Width: terminalWidth, Height: terminalHeight},
	)

	// Apply process info if available
	if processID.Valid {
		pid := int(processID.Int32)
		pod := ""
		if podID.Valid {
			pod = podID.String
		}
		session.Start(domain.ProcessInfo{
			PID:     pid,
			PodName: pod,
		})
	}

	// Apply end time if available
	if endedAt.Valid {
		session.End(nil, nil)
	}

	return session, nil
}
