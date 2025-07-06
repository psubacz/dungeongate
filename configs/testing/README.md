# DungeonGate Database Testing Guide

This directory contains configuration files for testing DungeonGate with different database setups, including the new read/write endpoint separation feature.

## Available Configurations

### 1. SQLite Embedded (`sqlite-embedded.yaml`)
**Best for:** Local development, quick testing, CI/CD
- **Database:** Single SQLite file
- **Read/Write:** Same connection for both
- **Setup:** No external dependencies
- **Performance:** Good for development, not for production

### 2. PostgreSQL Single Instance (`postgresql-single.yaml`)
**Best for:** Small deployments, staging environments
- **Database:** Single PostgreSQL instance
- **Read/Write:** Same instance (`reader_use_writer: true`)
- **Setup:** Requires PostgreSQL server
- **Performance:** Good for small to medium loads

### 3. PostgreSQL with Replica (`postgresql-replica.yaml`)
**Best for:** Production environments with read scaling needs
- **Database:** PostgreSQL primary + read replica
- **Read/Write:** Separate endpoints (`reader_use_writer: false`)
- **Setup:** Requires PostgreSQL primary and replica
- **Performance:** Excellent read scaling

### 4. AWS Aurora Simulation (`aws-aurora-simulation.yaml`)
**Best for:** Testing Aurora-like configurations locally
- **Database:** Simulated Aurora endpoints
- **Read/Write:** Separate endpoints with failover
- **Setup:** Two PostgreSQL instances on different ports
- **Performance:** Tests production-like scenarios

### 5. MySQL Test (`mysql-test.yaml`)
**Best for:** MySQL/MariaDB environments
- **Database:** Single MySQL instance
- **Read/Write:** Same instance
- **Setup:** Requires MySQL server
- **Performance:** Good for MySQL-specific testing

## Quick Start

### 1. Automatic Setup (Recommended)
```bash
# Set up all databases
python3 scripts/setup-databases.py all --test

# Or set up specific database
python3 scripts/setup-databases.py postgresql --test
```

### 2. Manual Testing
```bash
# Test all configurations
./scripts/test-database-configs.sh

# Test specific configuration
./scripts/test-database-configs.sh sqlite
```

### 3. Run with Specific Config
```bash
# SQLite (no dependencies)
go run test-build.go -config configs/testing/sqlite-embedded.yaml

# PostgreSQL (requires setup)
go run test-build.go -config configs/testing/postgresql-single.yaml
```

## Database Setup Requirements

### SQLite
- **Dependencies:** None (built into Go)
- **Setup:** Automatic directory creation

### PostgreSQL
```bash
# macOS
brew install postgresql
brew services start postgresql

# Ubuntu/Debian
sudo apt-get install postgresql postgresql-client
sudo systemctl start postgresql

# Create test database
createuser dungeongate_test
createdb -O dungeongate_test dungeongate_test
psql -c "ALTER USER dungeongate_test PASSWORD 'testpass123';"
```

### MySQL
```bash
# macOS
brew install mysql
brew services start mysql

# Ubuntu/Debian
sudo apt-get install mysql-server mysql-client
sudo systemctl start mysql

# Create test database
mysql -e "CREATE DATABASE dungeongate_mysql_test;"
mysql -e "CREATE USER 'dungeongate_mysql'@'localhost' IDENTIFIED BY 'mysql_test_pass';"
mysql -e "GRANT ALL PRIVILEGES ON dungeongate_mysql_test.* TO 'dungeongate_mysql'@'localhost';"
```

## Configuration Features Tested

### Database Read/Write Separation
- **Writer Endpoint:** All write operations (INSERT, UPDATE, DELETE)
- **Reader Endpoint:** All read operations (SELECT)
- **Failover:** Automatic fallback from reader to writer
- **Health Monitoring:** Continuous health checks

### User Registration Features
- **Validation:** Username, password, email validation
- **Rate Limiting:** Registration attempt limiting
- **Audit Logging:** All registration attempts logged
- **Security:** Argon2 password hashing

### Connection Pool Management
- **Writer Pool:** Optimized for write operations
- **Reader Pool:** Separate pool for read operations
- **Connection Limits:** Configurable max connections
- **Lifecycle Management:** Connection timeouts and cleanup

## Environment Variables

Create a `.env` file based on `.env.example`:

```bash
# PostgreSQL
DB_PASSWORD=testpass123

# MySQL  
MYSQL_PASSWORD=mysql_test_pass

# Aurora Simulation
AURORA_DB_PASSWORD=aurora_test_pass
```

## Testing Different Scenarios

### 1. Basic Registration Flow
```bash
# Start with SQLite for simplicity
go run test-build.go -config configs/testing/sqlite-embedded.yaml

# SSH to port 2222 and test registration
ssh localhost -p 2222
```

### 2. Read/Write Separation
```bash
# Use PostgreSQL replica config
go run test-build.go -config configs/testing/postgresql-replica.yaml

# Monitor logs to see read/write query routing
```

### 3. Failover Testing
```bash
# Use Aurora simulation config
go run test-build.go -config configs/testing/aws-aurora-simulation.yaml

# Stop replica to test failover
# Reads should automatically route to writer
```

### 4. Performance Testing
```bash
# Use different configs and compare
for config in configs/testing/*.yaml; do
    echo "Testing $config"
    go run test-build.go -config "$config" -benchmark
done
```

## Database Schema

All configurations use the same schema with these tables:
- `users` - Core user information
- `user_profiles` - Extended user profiles  
- `user_preferences` - Key-value preferences
- `user_roles` - Role assignments
- `registration_log` - Registration audit trail

## Troubleshooting

### Common Issues

1. **Database Connection Failed**
   ```bash
   # Check if database is running
   pg_isready -h localhost -p 5432  # PostgreSQL
   mysqladmin ping -h localhost     # MySQL
   ```

2. **Permission Denied**
   ```bash
   # Ensure test user has proper permissions
   psql -c "GRANT ALL PRIVILEGES ON DATABASE dungeongate_test TO dungeongate_test;"
   ```

3. **Port Already in Use**
   ```bash
   # Check what's using the port
   lsof -i :2222  # SSH port
   lsof -i :5432  # PostgreSQL
   ```

### Debug Logging

Enable debug logging in any config:
```yaml
logging:
  level: "debug"
  format: "text"
  output: "stdout"

database:
  settings:
    log_queries: true
```

## Production Deployment

For production deployment with AWS Aurora or similar:

1. **Use External Mode:**
   ```yaml
   database:
     mode: "external"
     external:
       writer_endpoint: "aurora-cluster.cluster-xyz.region.rds.amazonaws.com:5432"
       reader_use_writer: false
       reader_endpoint: "aurora-cluster.cluster-ro-xyz.region.rds.amazonaws.com:5432"
   ```

2. **Enable Security:**
   ```yaml
   database:
     external:
       ssl_mode: "require"
       options:
         sslcert: "/path/to/client-cert.pem"
         sslkey: "/path/to/client-key.pem"
         sslrootcert: "/path/to/ca-cert.pem"
   ```

3. **Configure Failover:**
   ```yaml
   database:
     external:
       failover:
         enabled: true
         health_check_interval: "15s"
         reader_to_writer_fallback: true
   ```

This testing setup ensures your DungeonGate deployment will work correctly with any database configuration from development to production.
