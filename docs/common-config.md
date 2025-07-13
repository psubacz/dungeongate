# Common Configuration Architecture

This document describes the shared configuration architecture implemented in DungeonGate's development environment.

## Overview

To reduce configuration duplication and ensure consistency across services, common configuration settings have been extracted into `common.yaml`. Individual service configuration files inherit from this common base and only specify service-specific overrides.

## Configuration Structure

### common.yaml
Contains shared configuration used across all services:

- **Database Configuration**: Complete SQLite/PostgreSQL configuration shared by all services
- **Logging Configuration**: Base logging settings with service-specific overrides
- **Health Check Configuration**: Common health check settings
- **Security Configuration**: Base security settings for development
- **Server Configuration**: Default timeouts and server settings
- **Environment Metadata**: Development environment information

### Service-Specific Configs
Each service config (`auth-service.yaml`, `game-service.yaml`, `session-service.yaml`) includes:

- **inherit_from**: Reference to `common.yaml` (documentation only - actual implementation in config loader)
- **Service-specific overrides**: Only settings that differ from common defaults
- **Unique functionality**: Features specific to each service

## Configuration Inheritance

### Example: Logging Configuration

**common.yaml** provides the base:
```yaml
logging:
  level: "debug"
  format: "text" 
  output: "stdout"
  file:
    directory: "./logs"
    filename: "service.log"  # Default, overridden by services
    max_size: "100MB"
    max_files: 10
    max_age: "30d"
    compress: true
  journald:
    identifier: "dungeongate-service"  # Default, overridden by services
    fields:
      service: "generic-service"      # Default, overridden by services
```

**auth-service.yaml** overrides only service-specific values:
```yaml
logging:
  file:
    filename: "auth-service.log"
  journald:
    identifier: "dungeongate-auth"
    fields:
      service: "auth-service"
```

**Final result** (after inheritance) for auth service:
```yaml
logging:
  level: "debug"           # From common.yaml
  format: "text"           # From common.yaml
  output: "stdout"         # From common.yaml
  file:
    directory: "./logs"    # From common.yaml
    filename: "auth-service.log"  # Overridden in auth-service.yaml
    max_size: "100MB"      # From common.yaml
    max_files: 10          # From common.yaml
    max_age: "30d"         # From common.yaml
    compress: true         # From common.yaml
  journald:
    identifier: "dungeongate-auth"   # Overridden in auth-service.yaml
    fields:
      service: "auth-service"        # Overridden in auth-service.yaml
```

## Benefits

### Reduced Duplication
- **Before**: ~450 lines of configuration per service (database + logging sections)
- **After**: ~15 lines of overrides per service 
- **Reduction**: ~97% reduction in duplicated configuration

### Consistency
- All services share identical database configuration
- Logging behavior is consistent with only service identification differences
- Security and server settings are uniform across services

### Maintainability
- Database connection changes only need to be made in one place
- Logging improvements benefit all services automatically
- Environment-wide settings can be updated centrally

## Implementation Notes

### Configuration Loading
The `inherit_from: "common.yaml"` directive in service configs is documentation only. The actual inheritance mechanism needs to be implemented in the configuration loading code to:

1. Load `common.yaml` first
2. Load service-specific config
3. Deep merge configurations with service config taking precedence
4. Apply environment variable substitutions to the merged result

### Service-Specific Sections

Services maintain their unique functionality in separate sections:

- **Auth Service**: JWT settings, encryption, email configuration, user management
- **Game Service**: Game engine settings, game definitions, metrics, resource management  
- **Session Service**: SSH server settings, menu configuration, session management, spectating

### Environment Variables
Environment variable substitution should work on the final merged configuration, allowing production deployments to override both common and service-specific settings.

## Configuration Sections Extracted

### Database Configuration (100% shared)
- Complete embedded SQLite configuration
- Complete external PostgreSQL configuration  
- Connection pooling and failover settings
- Query timeouts and retry logic
- Migration paths and schema settings

### Logging Configuration (95% shared)
- Log levels, formats, and outputs
- File rotation and compression settings
- Journald configuration structure
- Only service-specific: filenames and identifiers

### Security Configuration (100% shared in dev)
- Rate limiting settings (disabled in development)
- Brute force protection (disabled in development)
- Common security timeouts and thresholds

### Health Check Configuration (100% shared)
- Health check endpoints and timeouts
- Monitoring intervals and paths

## File Structure

```
configs/development/
├── common.yaml           # Shared configuration base
├── auth-service.yaml     # Auth service overrides + unique config
├── game-service.yaml     # Game service overrides + unique config
├── session-service.yaml  # Session service overrides + unique config
└── COMMON_CONFIG.md     # This documentation file
```

## Migration Impact

### Backward Compatibility
- All existing configuration paths still work
- No changes required to service startup commands
- Configuration loading logic needs to be updated to support inheritance

### Testing
- Services should continue to work with the refactored configurations
- Configuration merging logic should be thoroughly tested
- Environment variable substitution must work on merged configs

## Future Enhancements

### Production Configuration
Consider extracting additional common settings for production:
- Monitoring and metrics configuration
- Security hardening settings  
- Performance tuning parameters

### Validation
- Add schema validation for common.yaml
- Validate that service overrides don't conflict with required common settings
- Ensure all required configuration keys are present after merging

### Documentation
- Auto-generate configuration documentation from schemas
- Provide examples of common override patterns
- Document environment-specific differences