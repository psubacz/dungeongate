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

type PostgreSQLSaveRepository struct {
	db *database.Connection
}

func NewPostgreSQLSaveRepository(db *database.Connection) *PostgreSQLSaveRepository {
	return &PostgreSQLSaveRepository{db: db}
}

func (r *PostgreSQLSaveRepository) Create(ctx context.Context, save *domain.GameSave) error {
	metadata, err := json.Marshal(save.Metadata())
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO game_saves (
			id, user_id, game_id, session_id, name, description,
			file_path, file_size, checksum, version, is_active, metadata,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	// Build save name from metadata
	saveName := fmt.Sprintf("save_%s", save.ID().String())
	if save.Metadata().Character != "" {
		saveName = save.Metadata().Character
	}

	_, err = r.db.ExecContext(ctx, query,
		save.ID().String(),
		save.UserID().Int(),
		save.GameID().String(),
		nil, // session_id - would need to be passed in or derived
		saveName,
		"", // description - would need to be added to domain
		save.FilePath(),
		save.FileSize(),
		save.Checksum(),
		1, // version - would need to be added to domain
		save.IsActive(),
		metadata,
		save.CreatedAt(),
		save.UpdatedAt(),
	)

	if err != nil {
		return fmt.Errorf("failed to create game save: %w", err)
	}

	return nil
}

func (r *PostgreSQLSaveRepository) Update(ctx context.Context, save *domain.GameSave) error {
	metadata, err := json.Marshal(save.Metadata())
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Build save name from metadata
	saveName := fmt.Sprintf("save_%s", save.ID().String())
	if save.Metadata().Character != "" {
		saveName = save.Metadata().Character
	}

	query := `
		UPDATE game_saves SET
			name = $2,
			description = $3,
			file_path = $4,
			file_size = $5,
			checksum = $6,
			version = $7,
			is_active = $8,
			metadata = $9,
			updated_at = $10
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		save.ID().String(),
		saveName,
		"", // description - would need to be added to domain
		save.FilePath(),
		save.FileSize(),
		save.Checksum(),
		1, // version - would need to be added to domain
		save.IsActive(),
		metadata,
		time.Now(),
	)

	if err != nil {
		return fmt.Errorf("failed to update game save: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("game save not found: %s", save.ID().String())
	}

	return nil
}

func (r *PostgreSQLSaveRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM game_saves WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id.String())
	if err != nil {
		return fmt.Errorf("failed to delete game save: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("game save not found: %s", id.String())
	}

	return nil
}

func (r *PostgreSQLSaveRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.GameSave, error) {
	query := `
		SELECT id, user_id, game_id, session_id, name, description,
			file_path, file_size, checksum, version, is_active, metadata,
			created_at, updated_at
		FROM game_saves
		WHERE id = $1
	`

	row := r.db.QueryRowContext(ctx, query, id.String())
	return r.scanSave(row)
}

func (r *PostgreSQLSaveRepository) FindByUserAndGame(ctx context.Context, userID int, gameID int) ([]*domain.GameSave, error) {
	query := `
		SELECT id, user_id, game_id, session_id, name, description,
			file_path, file_size, checksum, version, is_active, metadata,
			created_at, updated_at
		FROM game_saves
		WHERE user_id = $1 AND game_id = $2
		ORDER BY updated_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, userID, gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to query saves: %w", err)
	}
	defer rows.Close()

	var saves []*domain.GameSave
	for rows.Next() {
		save, err := r.scanSaveFromRows(rows)
		if err != nil {
			return nil, err
		}
		saves = append(saves, save)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return saves, nil
}

func (r *PostgreSQLSaveRepository) FindActiveByUserAndGame(ctx context.Context, userID int, gameID int) (*domain.GameSave, error) {
	query := `
		SELECT id, user_id, game_id, session_id, name, description,
			file_path, file_size, checksum, version, is_active, metadata,
			created_at, updated_at
		FROM game_saves
		WHERE user_id = $1 AND game_id = $2 AND is_active = true
		ORDER BY updated_at DESC
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, userID, gameID)
	return r.scanSave(row)
}

func (r *PostgreSQLSaveRepository) FindBySession(ctx context.Context, sessionID uuid.UUID) ([]*domain.GameSave, error) {
	query := `
		SELECT id, user_id, game_id, session_id, name, description,
			file_path, file_size, checksum, version, is_active, metadata,
			created_at, updated_at
		FROM game_saves
		WHERE session_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, sessionID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to query saves by session: %w", err)
	}
	defer rows.Close()

	var saves []*domain.GameSave
	for rows.Next() {
		save, err := r.scanSaveFromRows(rows)
		if err != nil {
			return nil, err
		}
		saves = append(saves, save)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return saves, nil
}

func (r *PostgreSQLSaveRepository) CreateBackup(ctx context.Context, saveID uuid.UUID, backupPath string, fileSize int64, checksum string) error {
	// First, get the next backup number for this save
	var maxBackupNumber sql.NullInt32
	query := `SELECT MAX(backup_number) FROM game_save_backups WHERE save_id = $1`
	err := r.db.QueryRowContext(ctx, query, saveID.String()).Scan(&maxBackupNumber)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to get max backup number: %w", err)
	}

	backupNumber := 1
	if maxBackupNumber.Valid {
		backupNumber = int(maxBackupNumber.Int32) + 1
	}

	// Create the backup entry
	insertQuery := `
		INSERT INTO game_save_backups (
			id, save_id, backup_number, file_path, file_size, checksum, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err = r.db.ExecContext(ctx, insertQuery,
		uuid.New().String(),
		saveID.String(),
		backupNumber,
		backupPath,
		fileSize,
		checksum,
		time.Now(),
	)

	if err != nil {
		return fmt.Errorf("failed to create save backup: %w", err)
	}

	return nil
}

// SaveBackupData represents backup data from the database
type SaveBackupData struct {
	ID           string
	SaveID       string
	BackupNumber int
	FilePath     string
	FileSize     int64
	Checksum     string
	CreatedAt    time.Time
}

func (r *PostgreSQLSaveRepository) ListBackups(ctx context.Context, saveID uuid.UUID) ([]*SaveBackupData, error) {
	query := `
		SELECT id, save_id, backup_number, file_path, file_size, checksum, created_at
		FROM game_save_backups
		WHERE save_id = $1
		ORDER BY backup_number DESC
	`

	rows, err := r.db.QueryContext(ctx, query, saveID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to query save backups: %w", err)
	}
	defer rows.Close()

	var backups []*SaveBackupData
	for rows.Next() {
		var backup SaveBackupData
		err := rows.Scan(
			&backup.ID,
			&backup.SaveID,
			&backup.BackupNumber,
			&backup.FilePath,
			&backup.FileSize,
			&backup.Checksum,
			&backup.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan backup: %w", err)
		}
		backups = append(backups, &backup)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return backups, nil
}

func (r *PostgreSQLSaveRepository) DeleteOldBackups(ctx context.Context, saveID uuid.UUID, keepCount int) error {
	// Delete all but the most recent N backups
	query := `
		DELETE FROM game_save_backups
		WHERE save_id = $1 AND backup_number NOT IN (
			SELECT backup_number FROM game_save_backups
			WHERE save_id = $1
			ORDER BY backup_number DESC
			LIMIT $2
		)
	`

	_, err := r.db.ExecContext(ctx, query, saveID.String(), keepCount)
	if err != nil {
		return fmt.Errorf("failed to delete old backups: %w", err)
	}

	return nil
}

func (r *PostgreSQLSaveRepository) scanSave(row *sql.Row) (*domain.GameSave, error) {
	var saveID, gameID string
	var userID int
	var sessionID, name, description sql.NullString
	var filePath, checksum string
	var fileSize int64
	var version int
	var isActive bool
	var metadata []byte
	var createdAt, updatedAt time.Time

	err := row.Scan(
		&saveID,
		&userID,
		&gameID,
		&sessionID,
		&name,
		&description,
		&filePath,
		&fileSize,
		&checksum,
		&version,
		&isActive,
		&metadata,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("game save not found")
		}
		return nil, fmt.Errorf("failed to scan game save: %w", err)
	}

	return r.buildDomainSave(saveID, userID, gameID, filePath, fileSize, checksum, metadata, createdAt, updatedAt)
}

func (r *PostgreSQLSaveRepository) scanSaveFromRows(rows *sql.Rows) (*domain.GameSave, error) {
	var saveID, gameID string
	var userID int
	var sessionID, name, description sql.NullString
	var filePath, checksum string
	var fileSize int64
	var version int
	var isActive bool
	var metadata []byte
	var createdAt, updatedAt time.Time

	err := rows.Scan(
		&saveID,
		&userID,
		&gameID,
		&sessionID,
		&name,
		&description,
		&filePath,
		&fileSize,
		&checksum,
		&version,
		&isActive,
		&metadata,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to scan game save: %w", err)
	}

	return r.buildDomainSave(saveID, userID, gameID, filePath, fileSize, checksum, metadata, createdAt, updatedAt)
}

func (r *PostgreSQLSaveRepository) buildDomainSave(saveID string, userID int, gameID, filePath string, fileSize int64, checksum string, metadataBytes []byte, createdAt, updatedAt time.Time) (*domain.GameSave, error) {
	// Parse metadata
	var metadata domain.SaveMetadata
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	// Note: This is a simplified reconstruction from database data
	// In a real implementation, you might need to read the actual file data
	// or store the data separately
	save := domain.NewGameSave(
		domain.NewSaveID(saveID),
		domain.NewUserID(userID),
		domain.NewGameID(gameID),
		[]byte{}, // Empty data - would need to read from file
		filePath,
		metadata,
	)

	return save, nil
}
