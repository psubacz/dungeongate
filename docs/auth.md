# DungeonGate Authentication System

This document describes the DungeonGate authentication system, including user management, admin accounts, and security features.

## Overview

DungeonGate uses a centralized authentication service that provides:
- User registration and login
- JWT token-based authentication
- Role-based access control (RBAC)
- Admin user management
- One-time passwords for initial setup
- Configurable security policies

## Architecture

### Authentication Service
- **gRPC Server**: Port 8082 (service-to-service authentication)
- **HTTP Server**: Port 8081 (health checks and admin endpoints)
- **Database**: Shared SQLite/PostgreSQL with other services
- **JWT Tokens**: Stateless authentication with configurable expiration

### User Roles
- **Regular Users**: Basic access to game sessions
- **Admin Users**: Full system administration capabilities
- **Root Admin**: Special admin user for initial system setup

## Configuration

### Admin User Configuration

Admin users are configured in `configs/auth-service.yaml`:

```yaml
auth:
  # Root admin user (created automatically if no admins exist)
  root_admin_user:
    enabled: true
    name: admin                    # Optional: defaults to "admin"
    one_time_password: "secure123" # Required: strong password
    recovery_email: admin@company.com
  
  # Additional admin users
  admin_users:
    - name: alice
      one_time_password: "temp456"
      recovery_email: alice@company.com
    - name: bob
      one_time_password: "temp789"
      recovery_email: bob@company.com
```

### Security Features

```yaml
auth:
  # JWT token settings
  access_token_expiration: "15m"
  refresh_token_expiration: "168h"  # 7 days
  
  # Account lockout protection
  max_login_attempts: 3
  lockout_duration: "15m"
  
  # Password requirements
  require_password_change: true  # Force change on first login
```

## Admin User Management

### Automatic Admin Creation

When the auth service starts:

1. **Check for existing admins**: If any admin users exist, skip creation
2. **Create root admin**: If `root_admin_user.enabled: true` and no admins exist
3. **Create additional admins**: Process all users in `admin_users` list
4. **Fallback**: Create default `admin/admin123` if no config provided

### One-Time Passwords

Admin users created via configuration use **one-time passwords**:

-  **Secure**: Passwords are configured, not hardcoded
-  **Temporary**: Users must change password on first login
-  **Traceable**: Password change requirement tracked in database
-   **Log Security**: One-time passwords appear in logs during creation

#### One-Time Password Flow

1. **Service Startup**: Admin user created with `require_password_change: true`
2. **First Login**: User logs in with one-time password
3. **Forced Change**: System requires immediate password change
4. **Normal Operation**: `require_password_change: false` after successful change

### Admin Capabilities

Admin users can perform the following operations via gRPC API:

- **User Management**:
  - `UnlockUserAccount`: Unlock locked user accounts
  - `DeleteUserAccount`: Delete user accounts (with safety checks)
  - `ResetUserPassword`: Reset user passwords
  - `PromoteUserToAdmin`: Grant admin privileges to users

- **System Monitoring**:
  - `GetServerStatistics`: View server metrics and statistics

## Manual Admin Password Reset

If you lose access to admin accounts, you can manually reset the root admin password:

### Method 1: Configuration Reset

1. **Stop the auth service**:
   ```bash
   pkill -f dungeongate-auth-service
   ```

2. **Delete existing admin user** from database:
   ```sql
   -- Connect to the database
   sqlite3 data/sqlite/dungeongate-dev.db
   
   -- Remove admin user (forces recreation)
   DELETE FROM users WHERE username = 'admin';
   
   -- Or remove admin privileges (keeps user data)
   UPDATE users SET flags = flags & ~1 WHERE username = 'admin';
   ```

3. **Update configuration** with new password:
   ```yaml
   # configs/auth-service.yaml
   auth:
     root_admin_user:
       enabled: true
       one_time_password: "new_secure_password"
   ```

4. **Restart auth service**:
   ```bash
   make run-auth
   ```

### Method 2: Direct Database Reset

1. **Stop the auth service**:
   ```bash
   pkill -f dungeongate-auth-service
   ```

2. **Generate password hash** (using Go):
   ```go
   package main
   import (
       "crypto/rand"
       "encoding/hex"
       "fmt"
       "golang.org/x/crypto/argon2"
   )
   
   func main() {
       password := "new_admin_password"
       salt := make([]byte, 16)
       rand.Read(salt)
       hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
       
       fmt.Printf("Password Hash: %s\n", hex.EncodeToString(hash))
       fmt.Printf("Salt: %s\n", hex.EncodeToString(salt))
   }
   ```

3. **Update database directly**:
   ```sql
   sqlite3 data/sqlite/dungeongate-dev.db
   
   UPDATE users SET 
       password_hash = 'generated_hash',
       salt = 'generated_salt',
       require_password_change = 1,
       flags = flags | 1,
       updated_at = CURRENT_TIMESTAMP
   WHERE username = 'admin';
   ```

4. **Restart auth service**:
   ```bash
   make run-auth
   ```

### Method 3: Emergency Admin Creation

1. **Backup existing database**:
   ```bash
   cp data/sqlite/dungeongate-dev.db data/sqlite/dungeongate-dev.db.backup
   ```

2. **Create temporary config** with emergency admin:
   ```yaml
   # configs/emergency-admin.yaml
   inherit_from: "auth-service.yaml"
   auth:
     admin_users:
       - name: emergency_admin
         one_time_password: "emergency_temp_pass"
         recovery_email: emergency@company.com
   ```

3. **Start with emergency config**:
   ```bash
   ./build/dungeongate-auth-service -config=configs/emergency-admin.yaml
   ```

4. **Login as emergency admin** and reset other accounts

## Security Best Practices

### Password Security
-  Use strong one-time passwords (12+ characters)
-  Include special characters, numbers, uppercase/lowercase
-  Change passwords immediately after first login
-  Use unique passwords for each admin user
- L Never reuse one-time passwords
- L Never share passwords in plain text

### Operational Security
- = **Secure log files**: One-time passwords appear in startup logs
- = **Limit admin accounts**: Only create necessary admin users
- = **Monitor admin activity**: All admin actions are logged
- = **Regular password rotation**: Change admin passwords periodically
- = **Backup configurations**: Keep secure backups of auth configs

### Database Security
- = **Encrypt database**: Use full-disk encryption in production
- = **Restrict access**: Limit database file permissions to service user
- = **Regular backups**: Automated backup with encryption
- = **Access logging**: Monitor database access patterns

## API Reference

### Admin gRPC Endpoints

All admin endpoints require a valid admin JWT token in the `admin_token` field.

#### UnlockUserAccount
```protobuf
rpc UnlockUserAccount(AdminActionRequest) returns (AdminActionResponse);

message AdminActionRequest {
  string admin_token = 1;
  string target_username = 2;
}
```

#### DeleteUserAccount
```protobuf
rpc DeleteUserAccount(AdminActionRequest) returns (AdminActionResponse);
```
**Safety**: Prevents deletion of the last admin user.

#### ResetUserPassword
```protobuf
rpc ResetUserPassword(ResetPasswordAdminRequest) returns (AdminActionResponse);

message ResetPasswordAdminRequest {
  string admin_token = 1;
  string target_username = 2;
  string new_password = 3;
}
```

#### PromoteUserToAdmin
```protobuf
rpc PromoteUserToAdmin(AdminActionRequest) returns (AdminActionResponse);
```

#### GetServerStatistics
```protobuf
rpc GetServerStatistics(ServerStatsRequest) returns (ServerStatsResponse);

message ServerStatsRequest {
  string admin_token = 1;
}

message ServerStatsResponse {
  bool success = 1;
  string error = 2;
  map<string, string> stats = 3;  // total_users, active_users, etc.
}
```

## Troubleshooting

### Common Issues

#### "No admin users exist"
- **Cause**: Configuration not loaded properly
- **Solution**: Check YAML syntax and field names in config

#### "One-time password visible in logs"
- **Cause**: Normal behavior during admin creation
- **Solution**: Secure log files and rotate logs after setup

#### "Cannot delete last admin user"
- **Cause**: Safety mechanism prevents system lockout
- **Solution**: Create another admin before deleting

#### "User requires password change"
- **Cause**: User still has one-time password
- **Solution**: Force password change on next login

### Debug Commands

```bash
# Check admin users in database
sqlite3 data/sqlite/dungeongate-dev.db "SELECT username, flags, require_password_change FROM users WHERE (flags & 1) != 0;"

# Check service logs
tail -f logs/auth.log

# Test gRPC connection
grpcurl -plaintext localhost:8082 dungeongate.auth.v1.AuthService/Health

# Check HTTP health
curl http://localhost:8081/health
```

## Migration Guide

### Upgrading from Hardcoded Passwords

If you're upgrading from a version with hardcoded admin passwords:

1. **Backup current database**
2. **Add configuration** for existing admin users
3. **Set one-time passwords** for security
4. **Force password changes** on next login
5. **Remove old hardcoded passwords** from logs

### Configuration Migration

Old hardcoded approach:
```go
// Old: Hardcoded in code
defaultPassword := "admin123"
```

New configurable approach:
```yaml
# New: Configured in YAML
auth:
  root_admin_user:
    enabled: true
    one_time_password: "configured_secure_password"
```

## Related Documentation

- [Session Service Configuration](session.md)
- [Security Policies](../monitoring/security.md)
- [Database Setup](../deployments/database.md)
- [Production Deployment](../deployments/kubernetes.md)