package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/dungeongate/internal/games/domain"
	"github.com/dungeongate/pkg/database"
)

// PostgreSQLGameRepository implements GameRepository using PostgreSQL
type PostgreSQLGameRepository struct {
	db *database.Connection
}

// NewPostgreSQLGameRepository creates a new PostgreSQL game repository
func NewPostgreSQLGameRepository(db *database.Connection) *PostgreSQLGameRepository {
	return &PostgreSQLGameRepository{db: db}
}

// Save persists a game to the database
func (r *PostgreSQLGameRepository) Save(ctx context.Context, game *domain.Game) error {
	query := `
		INSERT INTO games (id, name, short_name, description, category, tags, version, difficulty,
			binary_path, binary_args, binary_working_dir, environment, cpu_limit, memory_limit, 
			disk_limit, timeout_seconds, run_as_user, run_as_group, read_only_root_filesystem,
			allow_privilege_escalation, capabilities, network_isolated, allowed_ports,
			allowed_domains, block_internet, status, total_sessions, active_sessions,
			total_play_time_seconds, average_session_time_seconds, unique_users, last_played,
			popularity_rank, rating, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18,
			$19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $34, $35, $36)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			short_name = EXCLUDED.short_name,
			description = EXCLUDED.description,
			category = EXCLUDED.category,
			tags = EXCLUDED.tags,
			version = EXCLUDED.version,
			difficulty = EXCLUDED.difficulty,
			binary_path = EXCLUDED.binary_path,
			binary_args = EXCLUDED.binary_args,
			binary_working_dir = EXCLUDED.binary_working_dir,
			environment = EXCLUDED.environment,
			cpu_limit = EXCLUDED.cpu_limit,
			memory_limit = EXCLUDED.memory_limit,
			disk_limit = EXCLUDED.disk_limit,
			timeout_seconds = EXCLUDED.timeout_seconds,
			run_as_user = EXCLUDED.run_as_user,
			run_as_group = EXCLUDED.run_as_group,
			read_only_root_filesystem = EXCLUDED.read_only_root_filesystem,
			allow_privilege_escalation = EXCLUDED.allow_privilege_escalation,
			capabilities = EXCLUDED.capabilities,
			network_isolated = EXCLUDED.network_isolated,
			allowed_ports = EXCLUDED.allowed_ports,
			allowed_domains = EXCLUDED.allowed_domains,
			block_internet = EXCLUDED.block_internet,
			status = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at
	`

	metadata := game.Metadata()
	config := game.Config()
	stats := game.Statistics()

	var lastPlayed *time.Time
	if stats.LastPlayed != nil {
		lastPlayed = stats.LastPlayed
	}

	_, err := r.db.ExecContext(ctx, query,
		game.ID().String(),
		metadata.Name,
		metadata.ShortName,
		metadata.Description,
		metadata.Category,
		metadata.Tags,
		metadata.Version,
		metadata.Difficulty,
		config.Binary.Path,
		config.Binary.Args,
		config.Binary.WorkingDirectory,
		config.Environment,
		config.Resources.CPULimit,
		config.Resources.MemoryLimit,
		config.Resources.DiskLimit,
		int(config.Resources.Timeout.Seconds()),
		config.Security.RunAsUser,
		config.Security.RunAsGroup,
		config.Security.ReadOnlyRootFilesystem,
		config.Security.AllowPrivilegeEscalation,
		config.Security.Capabilities,
		config.Networking.Isolated,
		config.Networking.AllowedPorts,
		config.Networking.AllowedDomains,
		config.Networking.BlockInternet,
		string(game.Status()),
		stats.TotalSessions,
		stats.ActiveSessions,
		int(stats.TotalPlayTime.Seconds()),
		int(stats.AverageSessionTime.Seconds()),
		stats.UniqueUsers,
		lastPlayed,
		stats.PopularityRank,
		stats.Rating,
		game.CreatedAt(),
		game.UpdatedAt(),
	)

	return err
}

// FindByID retrieves a game by its ID
func (r *PostgreSQLGameRepository) FindByID(ctx context.Context, id domain.GameID) (*domain.Game, error) {
	query := `
		SELECT id, name, short_name, description, category, tags, version, difficulty,
			binary_path, binary_args, binary_working_dir, environment, cpu_limit, memory_limit,
			disk_limit, timeout_seconds, run_as_user, run_as_group, read_only_root_filesystem,
			allow_privilege_escalation, capabilities, network_isolated, allowed_ports,
			allowed_domains, block_internet, status, total_sessions, active_sessions,
			total_play_time_seconds, average_session_time_seconds, unique_users, last_played,
			popularity_rank, rating, created_at, updated_at
		FROM games WHERE id = $1
	`

	row := r.db.QueryRowContext(ctx, query, id.String())
	return r.scanGame(row)
}

// FindByName retrieves a game by its name
func (r *PostgreSQLGameRepository) FindByName(ctx context.Context, name string) (*domain.Game, error) {
	query := `
		SELECT id, name, short_name, description, category, tags, version, difficulty,
			binary_path, binary_args, binary_working_dir, environment, cpu_limit, memory_limit,
			disk_limit, timeout_seconds, run_as_user, run_as_group, read_only_root_filesystem,
			allow_privilege_escalation, capabilities, network_isolated, allowed_ports,
			allowed_domains, block_internet, status, total_sessions, active_sessions,
			total_play_time_seconds, average_session_time_seconds, unique_users, last_played,
			popularity_rank, rating, created_at, updated_at
		FROM games WHERE name = $1
	`

	row := r.db.QueryRowContext(ctx, query, name)
	return r.scanGame(row)
}

// FindAll retrieves all games
func (r *PostgreSQLGameRepository) FindAll(ctx context.Context) ([]*domain.Game, error) {
	query := `
		SELECT id, name, short_name, description, category, tags, version, difficulty,
			binary_path, binary_args, binary_working_dir, environment, cpu_limit, memory_limit,
			disk_limit, timeout_seconds, run_as_user, run_as_group, read_only_root_filesystem,
			allow_privilege_escalation, capabilities, network_isolated, allowed_ports,
			allowed_domains, block_internet, status, total_sessions, active_sessions,
			total_play_time_seconds, average_session_time_seconds, unique_users, last_played,
			popularity_rank, rating, created_at, updated_at
		FROM games ORDER BY name
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var games []*domain.Game
	for rows.Next() {
		game, err := r.scanGame(rows)
		if err != nil {
			return nil, err
		}
		games = append(games, game)
	}

	return games, rows.Err()
}

// FindEnabled retrieves all enabled games
func (r *PostgreSQLGameRepository) FindEnabled(ctx context.Context) ([]*domain.Game, error) {
	query := `
		SELECT id, name, short_name, description, category, tags, version, difficulty,
			binary_path, binary_args, binary_working_dir, environment, cpu_limit, memory_limit,
			disk_limit, timeout_seconds, run_as_user, run_as_group, read_only_root_filesystem,
			allow_privilege_escalation, capabilities, network_isolated, allowed_ports,
			allowed_domains, block_internet, status, total_sessions, active_sessions,
			total_play_time_seconds, average_session_time_seconds, unique_users, last_played,
			popularity_rank, rating, created_at, updated_at
		FROM games WHERE status = 'enabled' ORDER BY name
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var games []*domain.Game
	for rows.Next() {
		game, err := r.scanGame(rows)
		if err != nil {
			return nil, err
		}
		games = append(games, game)
	}

	return games, rows.Err()
}

// Delete removes a game from the database
func (r *PostgreSQLGameRepository) Delete(ctx context.Context, id domain.GameID) error {
	query := `DELETE FROM games WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id.String())
	return err
}

// FindByCategory retrieves games by category
func (r *PostgreSQLGameRepository) FindByCategory(ctx context.Context, category string) ([]*domain.Game, error) {
	// Implementation would be similar to FindAll but with WHERE clause
	return nil, fmt.Errorf("not implemented")
}

// FindByTag retrieves games by tag
func (r *PostgreSQLGameRepository) FindByTag(ctx context.Context, tag string) ([]*domain.Game, error) {
	// Implementation would use array operations for tags
	return nil, fmt.Errorf("not implemented")
}

// SearchByName searches games by name pattern
func (r *PostgreSQLGameRepository) SearchByName(ctx context.Context, query string) ([]*domain.Game, error) {
	// Implementation would use ILIKE or full-text search
	return nil, fmt.Errorf("not implemented")
}

// CountByStatus counts games by status
func (r *PostgreSQLGameRepository) CountByStatus(ctx context.Context, status domain.GameStatus) (int, error) {
	query := `SELECT COUNT(*) FROM games WHERE status = $1`
	var count int
	err := r.db.QueryRowContext(ctx, query, string(status)).Scan(&count)
	return count, err
}

// UpdateStatistics updates game statistics
func (r *PostgreSQLGameRepository) UpdateStatistics(ctx context.Context, id domain.GameID, stats domain.GameStatistics) error {
	query := `
		UPDATE games SET
			total_sessions = $2,
			active_sessions = $3,
			total_play_time_seconds = $4,
			average_session_time_seconds = $5,
			unique_users = $6,
			last_played = $7,
			popularity_rank = $8,
			rating = $9,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`

	var lastPlayed *time.Time
	if stats.LastPlayed != nil {
		lastPlayed = stats.LastPlayed
	}

	_, err := r.db.ExecContext(ctx, query,
		id.String(),
		stats.TotalSessions,
		stats.ActiveSessions,
		int(stats.TotalPlayTime.Seconds()),
		int(stats.AverageSessionTime.Seconds()),
		stats.UniqueUsers,
		lastPlayed,
		stats.PopularityRank,
		stats.Rating,
	)

	return err
}

// GetMostPopular retrieves the most popular games
func (r *PostgreSQLGameRepository) GetMostPopular(ctx context.Context, limit int) ([]*domain.Game, error) {
	// Implementation would order by popularity metrics
	return nil, fmt.Errorf("not implemented")
}

// scanGame scans a database row into a Game domain object
func (r *PostgreSQLGameRepository) scanGame(scanner interface{}) (*domain.Game, error) {
	// This is a simplified implementation - would need to handle all the complex scanning
	// of arrays, JSON fields, etc. in a real implementation
	return nil, fmt.Errorf("scanGame not fully implemented")
}
