package domain

import (
	"crypto/sha256"
	"fmt"
	"time"
)

// GameSave aggregate root represents a game save file
type GameSave struct {
	// Identity
	id     SaveID
	userID UserID
	gameID GameID

	// Save data
	data     []byte
	metadata SaveMetadata
	checksum string

	// File information
	filePath string
	fileSize int64

	// State
	status SaveStatus

	// Backup information
	backups []SaveBackup

	// Audit
	createdAt time.Time
	updatedAt time.Time
}

// SaveID represents a unique save identifier
type SaveID struct {
	value string
}

// NewSaveID creates a new save ID
func NewSaveID(value string) SaveID {
	return SaveID{value: value}
}

// String returns the string representation of the save ID
func (id SaveID) String() string {
	return id.value
}

// SaveMetadata contains metadata about a save file
type SaveMetadata struct {
	GameVersion  string
	Character    string
	Level        int
	Score        int
	PlayTime     time.Duration
	Location     string
	CustomFields map[string]string
}

// SaveStatus represents the status of a save file
type SaveStatus string

const (
	SaveStatusActive   SaveStatus = "active"
	SaveStatusCorrupt  SaveStatus = "corrupt"
	SaveStatusArchived SaveStatus = "archived"
	SaveStatusDeleted  SaveStatus = "deleted"
)

// SaveBackup represents a backup of a save file
type SaveBackup struct {
	ID        string
	FilePath  string
	CreatedAt time.Time
	FileSize  int64
	Checksum  string
}

// NewGameSave creates a new game save
func NewGameSave(
	id SaveID,
	userID UserID,
	gameID GameID,
	data []byte,
	filePath string,
	metadata SaveMetadata,
) *GameSave {
	now := time.Now()
	checksum := calculateChecksum(data)

	return &GameSave{
		id:        id,
		userID:    userID,
		gameID:    gameID,
		data:      data,
		metadata:  metadata,
		checksum:  checksum,
		filePath:  filePath,
		fileSize:  int64(len(data)),
		status:    SaveStatusActive,
		backups:   make([]SaveBackup, 0),
		createdAt: now,
		updatedAt: now,
	}
}

// ID returns the save's ID
func (s *GameSave) ID() SaveID {
	return s.id
}

// UserID returns the save's user ID
func (s *GameSave) UserID() UserID {
	return s.userID
}

// GameID returns the save's game ID
func (s *GameSave) GameID() GameID {
	return s.gameID
}

// Data returns the save data
func (s *GameSave) Data() []byte {
	return s.data
}

// Metadata returns the save metadata
func (s *GameSave) Metadata() SaveMetadata {
	return s.metadata
}

// Checksum returns the save checksum
func (s *GameSave) Checksum() string {
	return s.checksum
}

// FilePath returns the save file path
func (s *GameSave) FilePath() string {
	return s.filePath
}

// FileSize returns the save file size
func (s *GameSave) FileSize() int64 {
	return s.fileSize
}

// Status returns the save status
func (s *GameSave) Status() SaveStatus {
	return s.status
}

// IsActive returns true if the save is active
func (s *GameSave) IsActive() bool {
	return s.status == SaveStatusActive
}

// UpdateData updates the save data
func (s *GameSave) UpdateData(data []byte, metadata SaveMetadata) error {
	if s.status != SaveStatusActive {
		return fmt.Errorf("cannot update save in status: %s", s.status)
	}

	// Create backup before updating
	backup := SaveBackup{
		ID:        fmt.Sprintf("%s_%d", s.id.String(), time.Now().Unix()),
		FilePath:  fmt.Sprintf("%s.bak.%d", s.filePath, time.Now().Unix()),
		CreatedAt: time.Now(),
		FileSize:  s.fileSize,
		Checksum:  s.checksum,
	}
	s.backups = append(s.backups, backup)

	// Update save data
	s.data = data
	s.metadata = metadata
	s.checksum = calculateChecksum(data)
	s.fileSize = int64(len(data))
	s.updatedAt = time.Now()

	return nil
}

// UpdateMetadata updates only the metadata
func (s *GameSave) UpdateMetadata(metadata SaveMetadata) {
	s.metadata = metadata
	s.updatedAt = time.Now()
}

// MarkCorrupt marks the save as corrupt
func (s *GameSave) MarkCorrupt() {
	s.status = SaveStatusCorrupt
	s.updatedAt = time.Now()
}

// Archive archives the save
func (s *GameSave) Archive() {
	s.status = SaveStatusArchived
	s.updatedAt = time.Now()
}

// Delete marks the save as deleted
func (s *GameSave) Delete() {
	s.status = SaveStatusDeleted
	s.updatedAt = time.Now()
}

// Restore restores the save from a backup
func (s *GameSave) Restore(backupID string, data []byte) error {
	// Find the backup
	var backup *SaveBackup
	for i := range s.backups {
		if s.backups[i].ID == backupID {
			backup = &s.backups[i]
			break
		}
	}

	if backup == nil {
		return fmt.Errorf("backup not found: %s", backupID)
	}

	// Verify backup integrity
	if calculateChecksum(data) != backup.Checksum {
		return fmt.Errorf("backup checksum mismatch")
	}

	// Restore the data
	s.data = data
	s.checksum = backup.Checksum
	s.fileSize = backup.FileSize
	s.status = SaveStatusActive
	s.updatedAt = time.Now()

	return nil
}

// Verify verifies the integrity of the save
func (s *GameSave) Verify() bool {
	return calculateChecksum(s.data) == s.checksum
}

// CreateBackup creates a manual backup
func (s *GameSave) CreateBackup(backupID, filePath string) SaveBackup {
	backup := SaveBackup{
		ID:        backupID,
		FilePath:  filePath,
		CreatedAt: time.Now(),
		FileSize:  s.fileSize,
		Checksum:  s.checksum,
	}
	s.backups = append(s.backups, backup)
	s.updatedAt = time.Now()
	return backup
}

// Backups returns the list of backups
func (s *GameSave) Backups() []SaveBackup {
	return s.backups
}

// CleanupOldBackups removes backups older than the specified duration
func (s *GameSave) CleanupOldBackups(maxAge time.Duration) []SaveBackup {
	cutoff := time.Now().Add(-maxAge)
	var keptBackups []SaveBackup
	var removedBackups []SaveBackup

	for _, backup := range s.backups {
		if backup.CreatedAt.After(cutoff) {
			keptBackups = append(keptBackups, backup)
		} else {
			removedBackups = append(removedBackups, backup)
		}
	}

	s.backups = keptBackups
	if len(removedBackups) > 0 {
		s.updatedAt = time.Now()
	}

	return removedBackups
}

// CreatedAt returns when the save was created
func (s *GameSave) CreatedAt() time.Time {
	return s.createdAt
}

// UpdatedAt returns when the save was last updated
func (s *GameSave) UpdatedAt() time.Time {
	return s.updatedAt
}

// calculateChecksum calculates the SHA256 checksum of data
func calculateChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)[:16] // Use first 16 characters for display
}
