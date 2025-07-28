# NetHack Profile Management Implementation Plan

## Overview
Implement dgamelaunch-style profile editing for NetHack, allowing users to edit their `.nethackrc` configuration files through the `[e] Edit profile` menu option using nano.

## Architecture

### Workflow
```
User selects [e] Edit profile 
    ↓
Session Service launches nano with temp file containing current profile
    ↓
User edits profile with nano
    ↓
Session Service validates and sends profile to Game Service
    ↓
Game Service stores profile in database and file system
    ↓
Profile available for next game session
```

## Implementation Tasks

### 1. API Extensions (Games API v2)

**New gRPC Methods:**
```protobuf
service GameService {
  // Profile management  
  rpc GetUserProfile(GetUserProfileRequest) returns (GetUserProfileResponse);
  rpc SaveUserProfile(SaveUserProfileRequest) returns (SaveUserProfileResponse);
  rpc ValidateProfile(ValidateProfileRequest) returns (ValidateProfileResponse);
}

message UserProfile {
  int32 user_id = 1;
  string game_id = 2;
  string content = 3;
  string format = 4; // "nethackrc", "config", etc.
  google.protobuf.Timestamp created_at = 5;
  google.protobuf.Timestamp updated_at = 6;
}

message GetUserProfileRequest {
  int32 user_id = 1;
  string game_id = 2;
  string format = 3; // optional, defaults to "nethackrc"
}

message GetUserProfileResponse {
  UserProfile profile = 1;
  bool exists = 2;
}

message SaveUserProfileRequest {
  UserProfile profile = 1;
}

message SaveUserProfileResponse {
  bool success = 1;
  string error = 2;
  UserProfile profile = 3;
}

message ValidateProfileRequest {
  string content = 1;
  string game_id = 2;
  string format = 3;
}

message ValidateProfileResponse {
  bool valid = 1;
  repeated string errors = 2;
  repeated string warnings = 3;
  string sanitized_content = 4;
}
```

### 2. Database Schema

**Migration: `add_user_profiles_table.sql`**
```sql
CREATE TABLE user_profiles (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    game_id VARCHAR(50) NOT NULL,
    content TEXT NOT NULL,
    format VARCHAR(20) DEFAULT 'nethackrc',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, game_id, format)
);

CREATE INDEX idx_user_profiles_user_game ON user_profiles(user_id, game_id);
```

### 3. Security Implementation

**Profile Validation Rules:**
- Maximum profile size: 64KB
- Forbidden options: `HACKDIR`, `SAVE`, `BONES`, `PANICLOG`, `SYSCF`
- Block shell escapes: `!`, `|`, `$()`, backticks
- Whitelist allowed NetHack options
- Sanitize dangerous patterns

**Nano Editor Security:**
- Run in restricted mode: `nano -R`
- Set `SHELL=/bin/false`
- Use temporary files with 600 permissions
- Automatic cleanup on session end

### 4. File Structure

**New Files to Create:**

#### Game Service
```
internal/games/profiles/
├── repository.go          # Database operations
├── validator.go           # Security validation
├── service.go            # Business logic
└── handler.go            # gRPC handlers

pkg/profiles/
└── security.go           # Shared validation utilities
```

#### Session Service
```
internal/session/profiles/
├── editor.go             # Nano integration
└── client.go            # Game Service client calls
```

#### Database
```
migrations/
└── 007_add_user_profiles_table.sql
```

### 5. Implementation Details

#### Session Service Changes

**File: `internal/session/connection/menuchoice.go`**
- Replace "not yet implemented" with profile editor handler
- Create temp file with current profile content
- Launch nano with security restrictions
- Validate edited content
- Send to Game Service for storage

**File: `internal/session/profiles/editor.go`**
```go
type ProfileEditor struct {
    gameClient *client.GameClient
    logger     *slog.Logger
}

func (pe *ProfileEditor) EditProfile(ctx context.Context, channel ssh.Channel, user *authv1.User, gameID string) error {
    // 1. Get current profile from Game Service
    // 2. Create temp file with content
    // 3. Launch nano with restrictions
    // 4. Validate edited content
    // 5. Save to Game Service
    // 6. Clean up temp files
}
```

#### Game Service Changes

**File: `internal/games/profiles/repository.go`**
```go
type ProfileRepository interface {
    GetProfile(ctx context.Context, userID int32, gameID, format string) (*UserProfile, error)
    SaveProfile(ctx context.Context, profile *UserProfile) error
    DeleteProfile(ctx context.Context, userID int32, gameID, format string) error
}
```

**File: `internal/games/profiles/validator.go`**
```go
type ProfileValidator struct {
    allowedOptions    map[string]bool
    forbiddenOptions  []string
    dangerousPatterns []*regexp.Regexp
    maxSize          int64
}

func (pv *ProfileValidator) ValidateContent(content, gameID, format string) (*ValidationResult, error)
```

**File: `internal/games/profiles/service.go`**
```go
type ProfileService struct {
    repo      ProfileRepository
    validator *ProfileValidator
    logger    *slog.Logger
}
```

#### Integration with Game Sessions

**File: `internal/games/saves/manager.go`**
- Update `SetupUserEnvironment` to load user profile
- Replace default .nethackrc creation with profile content
- Set NETHACKOPTIONS to user's custom config

### 6. Security Considerations

**Input Validation:**
```go
var forbiddenOptions = []string{
    "HACKDIR", "SAVE", "BONES", "PANICLOG", "SYSCF",
}

var dangerousPatterns = []*regexp.Regexp{
    regexp.MustCompile(`!\s*[^#]`),    // Shell escapes
    regexp.MustCompile(`\|\s*[^#]`),   // Pipe commands
    regexp.MustCompile(`\$\([^)]+\)`), // Command substitution
    regexp.MustCompile("`[^`]+`"),     // Backtick execution
}

var allowedOptions = map[string]bool{
    "OPTIONS": true, "PICKUP": true, "AUTOPICKUP": true,
    "MSGTYPE": true, "SOUND": true, "GRAPHICS": true,
    "SYMBOLS": true, "FRUIT": true, "CATNAME": true,
    "DOGNAME": true, "HORSENAME": true, "MENUCOLOR": true,
    "STATUSCOLOR": true,
}
```

**File System Security:**
- Temp files: `/tmp/dungeongate_profile_${user_id}_${session_id}.nethackrc`
- Permissions: 600 (user read/write only)
- Automatic cleanup on session end or timeout
- Mount temp directory with `noexec` flag

### 7. Testing Strategy

**Unit Tests:**
- Profile validation logic
- Security pattern detection
- Database operations
- gRPC handlers

**Integration Tests:**
- End-to-end profile editing workflow
- Nano editor integration
- Game session profile loading
- Error handling and cleanup

**Security Tests:**
- Attempt code injection through profiles
- Test forbidden option blocking
- Validate file system isolation
- Test cleanup on abnormal termination

### 8. Deployment Considerations

**Database Migration:**
- Add migration to existing migration system
- Ensure backward compatibility
- Plan for rollback if needed

**Configuration:**
- Add profile validation settings to game-service.yaml
- Configure temp file cleanup intervals
- Set security policy parameters

**Monitoring:**
- Log profile edit attempts
- Monitor for security violations
- Track profile validation failures
- Alert on suspicious activity

### 9. Default NetHack Profile

**Default .nethackrc content:**
```
# DungeonGate Default NetHack Configuration
OPTIONS=color,boulder:0,pickup_types:$"=/!?+
OPTIONS=hilite_pet,showexp,time,showscore
OPTIONS=autodig,autopickup,safe_pet
OPTIONS=menucolors,statuscolors
PICKUP_TYPES=$"=/!?+
AUTOPICKUP_EXCEPTION=">.*cursed.*"
AUTOPICKUP_EXCEPTION=">.*blessed.*"
MSGTYPE=stop "You feel hungry"
MSGTYPE=stop "You are beginning to feel hungry"
```

### 10. Future Enhancements

- Profile versioning and history
- Profile templates and sharing
- Game-specific profile validation
- Profile import/export functionality
- Web-based profile editor
- Profile backup and restore

## Implementation Priority

1. **High Priority:**
   - API extensions and proto generation
   - Security validation framework
   - Basic profile storage and retrieval

2. **Medium Priority:**
   - Session service nano integration
   - Database migration
   - Game session profile loading

3. **Low Priority:**
   - Advanced validation features
   - Monitoring and alerting
   - Profile versioning

## Success Criteria

- Users can edit NetHack profiles via `[e] Edit profile`
- Profiles are securely validated and stored
- No code execution vulnerabilities
- Profiles automatically load in game sessions
- Graceful error handling and user feedback
- Complete audit trail of profile operations