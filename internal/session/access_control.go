package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"
)

// AccessControlManagerImpl implements the AccessControlManager interface
type AccessControlManagerImpl struct {
	config        *ServerAccessConfig
	inviteKeys    map[string]*InviteKey
	preloadedKeys map[string]*PreloadedKey
	accessLogs    []*AccessLog
	mutex         sync.RWMutex

	// Statistics
	stats *ServerAccessStats
}

// AccessLog represents an access attempt log entry
type AccessLog struct {
	ID        string    `json:"id"`
	IPAddress string    `json:"ip_address"`
	Username  string    `json:"username"`
	Action    string    `json:"action"` // "login", "register", "key_validation"
	Success   bool      `json:"success"`
	Reason    string    `json:"reason"`
	Timestamp time.Time `json:"timestamp"`
	UserAgent string    `json:"user_agent,omitempty"`
}

// NewAccessControlManager creates a new access control manager
func NewAccessControlManager(config *ServerAccessConfig) *AccessControlManagerImpl {
	return &AccessControlManagerImpl{
		config:        config,
		inviteKeys:    make(map[string]*InviteKey),
		preloadedKeys: make(map[string]*PreloadedKey),
		accessLogs:    make([]*AccessLog, 0),
		stats: &ServerAccessStats{
			Mode: config.Mode,
		},
	}
}

// CheckAccess validates access based on server configuration
func (acm *AccessControlManagerImpl) CheckAccess(ctx context.Context, req *AccessControlRequest) (*AccessControlResponse, error) {
	acm.mutex.Lock()
	defer acm.mutex.Unlock()

	// Log the access attempt
	logEntry := &AccessLog{
		ID:        generateAccessLogID(),
		IPAddress: req.IPAddress,
		Username:  req.Username,
		Action:    "access_check",
		Timestamp: time.Now(),
		UserAgent: req.UserAgent,
	}

	response := &AccessControlResponse{
		MaxUsers:     acm.config.MaxUsers,
		CurrentUsers: acm.stats.ActiveUsers,
	}

	// Check user limits
	if acm.stats.ActiveUsers >= acm.config.MaxUsers {
		response.Allowed = false
		response.Reason = "Server at maximum capacity"
		logEntry.Success = false
		logEntry.Reason = response.Reason
		acm.accessLogs = append(acm.accessLogs, logEntry)
		return response, nil
	}

	switch acm.config.Mode {
	case AccessModePublic:
		return acm.checkPublicAccess(req, response, logEntry)
	case AccessModeSemiPublic:
		return acm.checkSemiPublicAccess(req, response, logEntry)
	case AccessModePrivate:
		return acm.checkPrivateAccess(req, response, logEntry)
	default:
		response.Allowed = false
		response.Reason = "Invalid server access mode"
		logEntry.Success = false
		logEntry.Reason = response.Reason
		acm.accessLogs = append(acm.accessLogs, logEntry)
		return response, nil
	}
}

// checkPublicAccess handles public server access
func (acm *AccessControlManagerImpl) checkPublicAccess(req *AccessControlRequest, response *AccessControlResponse, logEntry *AccessLog) (*AccessControlResponse, error) {
	// Public servers allow anyone to register
	if acm.config.AllowAnonymous {
		// Check anonymous user limits
		if acm.stats.AnonymousUsers >= acm.config.MaxAnonymousUsers {
			response.Allowed = false
			response.Reason = "Maximum anonymous users reached"
			logEntry.Success = false
			logEntry.Reason = response.Reason
			acm.accessLogs = append(acm.accessLogs, logEntry)
			return response, nil
		}
	}

	response.Allowed = true
	logEntry.Success = true
	acm.accessLogs = append(acm.accessLogs, logEntry)
	return response, nil
}

// checkSemiPublicAccess handles semi-public server access
func (acm *AccessControlManagerImpl) checkSemiPublicAccess(req *AccessControlRequest, response *AccessControlResponse, logEntry *AccessLog) (*AccessControlResponse, error) {
	// Semi-public servers require invite keys
	if req.InviteKey == "" {
		response.Allowed = false
		response.Reason = "Invite key required"
		logEntry.Success = false
		logEntry.Reason = response.Reason
		acm.accessLogs = append(acm.accessLogs, logEntry)
		return response, nil
	}

	// Validate invite key
	inviteKey, err := acm.validateInviteKeyInternal(req.InviteKey)
	if err != nil {
		response.Allowed = false
		response.Reason = "Invalid invite key"
		logEntry.Success = false
		logEntry.Reason = response.Reason
		acm.accessLogs = append(acm.accessLogs, logEntry)
		return response, nil
	}

	// Check if key is still valid
	if !inviteKey.IsActive {
		response.Allowed = false
		response.Reason = "Invite key is no longer active"
		logEntry.Success = false
		logEntry.Reason = response.Reason
		acm.accessLogs = append(acm.accessLogs, logEntry)
		return response, nil
	}

	// Check expiration
	if inviteKey.ExpiresAt != nil && time.Now().After(*inviteKey.ExpiresAt) {
		response.Allowed = false
		response.Reason = "Invite key has expired"
		logEntry.Success = false
		logEntry.Reason = response.Reason
		acm.accessLogs = append(acm.accessLogs, logEntry)
		return response, nil
	}

	// Check usage limits
	if inviteKey.MaxUses > 0 && inviteKey.CurrentUses >= inviteKey.MaxUses {
		response.Allowed = false
		response.Reason = "Invite key usage limit reached"
		logEntry.Success = false
		logEntry.Reason = response.Reason
		acm.accessLogs = append(acm.accessLogs, logEntry)
		return response, nil
	}

	response.Allowed = true
	logEntry.Success = true
	acm.accessLogs = append(acm.accessLogs, logEntry)
	return response, nil
}

// checkPrivateAccess handles private server access
func (acm *AccessControlManagerImpl) checkPrivateAccess(req *AccessControlRequest, response *AccessControlResponse, logEntry *AccessLog) (*AccessControlResponse, error) {
	// Private servers require preloaded keys
	if req.AccessKey == "" {
		response.Allowed = false
		response.Reason = "Access key required"
		logEntry.Success = false
		logEntry.Reason = response.Reason
		acm.accessLogs = append(acm.accessLogs, logEntry)
		return response, nil
	}

	// Validate preloaded key
	preloadedKey, err := acm.validatePreloadedKeyInternal(req.AccessKey)
	if err != nil {
		response.Allowed = false
		response.Reason = "Invalid access key"
		logEntry.Success = false
		logEntry.Reason = response.Reason
		acm.accessLogs = append(acm.accessLogs, logEntry)
		return response, nil
	}

	// Check if key is still valid
	if !preloadedKey.IsActive {
		response.Allowed = false
		response.Reason = "Access key is no longer active"
		logEntry.Success = false
		logEntry.Reason = response.Reason
		acm.accessLogs = append(acm.accessLogs, logEntry)
		return response, nil
	}

	// Check expiration
	if preloadedKey.ExpiresAt != nil && time.Now().After(*preloadedKey.ExpiresAt) {
		response.Allowed = false
		response.Reason = "Access key has expired"
		logEntry.Success = false
		logEntry.Reason = response.Reason
		acm.accessLogs = append(acm.accessLogs, logEntry)
		return response, nil
	}

	// Check username match
	if preloadedKey.Username != req.Username {
		response.Allowed = false
		response.Reason = "Username does not match access key"
		logEntry.Success = false
		logEntry.Reason = response.Reason
		acm.accessLogs = append(acm.accessLogs, logEntry)
		return response, nil
	}

	response.Allowed = true
	response.RequiredRole = preloadedKey.Role
	logEntry.Success = true
	acm.accessLogs = append(acm.accessLogs, logEntry)
	return response, nil
}

// ValidateInviteKey validates an invite key
func (acm *AccessControlManagerImpl) ValidateInviteKey(ctx context.Context, key string) (*InviteKey, error) {
	acm.mutex.RLock()
	defer acm.mutex.RUnlock()

	return acm.validateInviteKeyInternal(key)
}

// validateInviteKeyInternal validates an invite key (internal method)
func (acm *AccessControlManagerImpl) validateInviteKeyInternal(key string) (*InviteKey, error) {
	inviteKey, exists := acm.inviteKeys[key]
	if !exists {
		return nil, fmt.Errorf("invite key not found")
	}

	return inviteKey, nil
}

// ValidatePreloadedKey validates a preloaded key
func (acm *AccessControlManagerImpl) ValidatePreloadedKey(ctx context.Context, key string) (*PreloadedKey, error) {
	acm.mutex.RLock()
	defer acm.mutex.RUnlock()

	return acm.validatePreloadedKeyInternal(key)
}

// validatePreloadedKeyInternal validates a preloaded key (internal method)
func (acm *AccessControlManagerImpl) validatePreloadedKeyInternal(key string) (*PreloadedKey, error) {
	preloadedKey, exists := acm.preloadedKeys[key]
	if !exists {
		return nil, fmt.Errorf("preloaded key not found")
	}

	return preloadedKey, nil
}

// CreateInviteKey creates a new invite key
func (acm *AccessControlManagerImpl) CreateInviteKey(ctx context.Context, createdBy string, opts *InviteKeyOptions) (*InviteKey, error) {
	acm.mutex.Lock()
	defer acm.mutex.Unlock()

	key := generateInviteKey()
	inviteKey := &InviteKey{
		ID:          generateKeyID(),
		Key:         key,
		CreatedBy:   createdBy,
		CreatedAt:   time.Now(),
		IsActive:    true,
		MaxUses:     opts.MaxUses,
		CurrentUses: 0,
		Notes:       opts.Notes,
	}

	if opts.ExpiresAt != nil {
		inviteKey.ExpiresAt = opts.ExpiresAt
	}

	acm.inviteKeys[key] = inviteKey
	acm.stats.ActiveInviteKeys++

	log.Printf("Created invite key: %s by %s", inviteKey.ID, createdBy)
	return inviteKey, nil
}

// CreatePreloadedKey creates a new preloaded key
func (acm *AccessControlManagerImpl) CreatePreloadedKey(ctx context.Context, createdBy string, opts *PreloadedKeyOptions) (*PreloadedKey, error) {
	acm.mutex.Lock()
	defer acm.mutex.Unlock()

	key := generateAccessKey()
	preloadedKey := &PreloadedKey{
		ID:        generateKeyID(),
		Key:       key,
		Username:  opts.Username,
		Email:     opts.Email,
		CreatedBy: createdBy,
		CreatedAt: time.Now(),
		IsActive:  true,
		Role:      opts.Role,
		Notes:     opts.Notes,
	}

	if opts.ExpiresAt != nil {
		preloadedKey.ExpiresAt = opts.ExpiresAt
	}

	acm.preloadedKeys[key] = preloadedKey
	acm.stats.ActivePreloadedKeys++

	log.Printf("Created preloaded key: %s for user %s by %s", preloadedKey.ID, opts.Username, createdBy)
	return preloadedKey, nil
}

// RevokeInviteKey revokes an invite key
func (acm *AccessControlManagerImpl) RevokeInviteKey(ctx context.Context, keyID string) error {
	acm.mutex.Lock()
	defer acm.mutex.Unlock()

	// Find key by ID
	for _, inviteKey := range acm.inviteKeys {
		if inviteKey.ID == keyID {
			inviteKey.IsActive = false
			acm.stats.ActiveInviteKeys--
			log.Printf("Revoked invite key: %s", keyID)
			return nil
		}
	}

	return fmt.Errorf("invite key not found: %s", keyID)
}

// RevokePreloadedKey revokes a preloaded key
func (acm *AccessControlManagerImpl) RevokePreloadedKey(ctx context.Context, keyID string) error {
	acm.mutex.Lock()
	defer acm.mutex.Unlock()

	// Find key by ID
	for _, preloadedKey := range acm.preloadedKeys {
		if preloadedKey.ID == keyID {
			preloadedKey.IsActive = false
			acm.stats.ActivePreloadedKeys--
			log.Printf("Revoked preloaded key: %s", keyID)
			return nil
		}
	}

	return fmt.Errorf("preloaded key not found: %s", keyID)
}

// ListInviteKeys returns all invite keys
func (acm *AccessControlManagerImpl) ListInviteKeys(ctx context.Context, activeOnly bool) ([]*InviteKey, error) {
	acm.mutex.RLock()
	defer acm.mutex.RUnlock()

	keys := make([]*InviteKey, 0)
	for _, inviteKey := range acm.inviteKeys {
		if !activeOnly || inviteKey.IsActive {
			keys = append(keys, inviteKey)
		}
	}

	return keys, nil
}

// ListPreloadedKeys returns all preloaded keys
func (acm *AccessControlManagerImpl) ListPreloadedKeys(ctx context.Context, activeOnly bool) ([]*PreloadedKey, error) {
	acm.mutex.RLock()
	defer acm.mutex.RUnlock()

	keys := make([]*PreloadedKey, 0)
	for _, preloadedKey := range acm.preloadedKeys {
		if !activeOnly || preloadedKey.IsActive {
			keys = append(keys, preloadedKey)
		}
	}

	return keys, nil
}

// GetServerStats returns server access statistics
func (acm *AccessControlManagerImpl) GetServerStats(ctx context.Context) (*ServerAccessStats, error) {
	acm.mutex.RLock()
	defer acm.mutex.RUnlock()

	// Update dynamic stats
	stats := *acm.stats
	stats.MaxUsers = acm.config.MaxUsers

	return &stats, nil
}

// UseInviteKey marks an invite key as used
func (acm *AccessControlManagerImpl) UseInviteKey(ctx context.Context, key string, usedBy string) error {
	acm.mutex.Lock()
	defer acm.mutex.Unlock()

	inviteKey, exists := acm.inviteKeys[key]
	if !exists {
		return fmt.Errorf("invite key not found")
	}

	inviteKey.CurrentUses++
	now := time.Now()
	inviteKey.UsedAt = &now
	inviteKey.UsedBy = &usedBy

	// Deactivate if max uses reached
	if inviteKey.MaxUses > 0 && inviteKey.CurrentUses >= inviteKey.MaxUses {
		inviteKey.IsActive = false
		acm.stats.ActiveInviteKeys--
		acm.stats.UsedInviteKeys++
	}

	log.Printf("Invite key %s used by %s (usage: %d/%d)", inviteKey.ID, usedBy, inviteKey.CurrentUses, inviteKey.MaxUses)
	return nil
}

// UsePreloadedKey marks a preloaded key as used
func (acm *AccessControlManagerImpl) UsePreloadedKey(ctx context.Context, key string) error {
	acm.mutex.Lock()
	defer acm.mutex.Unlock()

	preloadedKey, exists := acm.preloadedKeys[key]
	if !exists {
		return fmt.Errorf("preloaded key not found")
	}

	now := time.Now()
	preloadedKey.UsedAt = &now

	// Mark as used (private keys are typically single-use)
	preloadedKey.IsActive = false
	acm.stats.ActivePreloadedKeys--
	acm.stats.UsedPreloadedKeys++

	log.Printf("Preloaded key %s used for user %s", preloadedKey.ID, preloadedKey.Username)
	return nil
}

// CleanupExpiredKeys removes expired keys
func (acm *AccessControlManagerImpl) CleanupExpiredKeys(ctx context.Context) error {
	acm.mutex.Lock()
	defer acm.mutex.Unlock()

	now := time.Now()
	cleanedInvite := 0
	cleanedPreloaded := 0

	// Clean up expired invite keys
	for key, inviteKey := range acm.inviteKeys {
		if inviteKey.ExpiresAt != nil && now.After(*inviteKey.ExpiresAt) {
			if inviteKey.IsActive {
				inviteKey.IsActive = false
				acm.stats.ActiveInviteKeys--
			}
			delete(acm.inviteKeys, key)
			cleanedInvite++
		}
	}

	// Clean up expired preloaded keys
	for key, preloadedKey := range acm.preloadedKeys {
		if preloadedKey.ExpiresAt != nil && now.After(*preloadedKey.ExpiresAt) {
			if preloadedKey.IsActive {
				preloadedKey.IsActive = false
				acm.stats.ActivePreloadedKeys--
			}
			delete(acm.preloadedKeys, key)
			cleanedPreloaded++
		}
	}

	if cleanedInvite > 0 || cleanedPreloaded > 0 {
		log.Printf("Cleaned up %d expired invite keys and %d expired preloaded keys", cleanedInvite, cleanedPreloaded)
	}

	return nil
}

// UpdateUserStats updates user statistics
func (acm *AccessControlManagerImpl) UpdateUserStats(activeUsers, anonymousUsers, registeredUsers int) {
	acm.mutex.Lock()
	defer acm.mutex.Unlock()

	acm.stats.ActiveUsers = activeUsers
	acm.stats.AnonymousUsers = anonymousUsers
	acm.stats.RegisteredUsers = registeredUsers
	acm.stats.TotalUsers = anonymousUsers + registeredUsers
}

// Helper functions for key generation

// generateInviteKey generates a random invite key
func generateInviteKey() string {
	return "inv_" + generateRandomString(16)
}

// generateAccessKey generates a random access key
func generateAccessKey() string {
	return "acc_" + generateRandomString(32)
}

// generateKeyID generates a random key ID
func generateKeyID() string {
	return "key_" + generateRandomString(12)
}

// generateAccessLogID generates a random access log ID
func generateAccessLogID() string {
	return "log_" + generateRandomString(12)
}

// generateRandomString generates a random hexadecimal string
func generateRandomString(length int) string {
	bytes := make([]byte, length/2)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
