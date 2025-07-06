#!/usr/bin/env python3
"""
Database Setup Helper Script
Helps set up different database environments for testing DungeonGate
"""

import argparse
import subprocess
import sys
import os
import time

def run_command(cmd, check=True, capture_output=False):
    """Run a shell command"""
    print(f"Running: {cmd}")
    result = subprocess.run(cmd, shell=True, check=check, 
                          capture_output=capture_output, text=True)
    if capture_output:
        return result.stdout.strip()
    return result.returncode == 0

def setup_sqlite():
    """Set up SQLite for testing"""
    print("Setting up SQLite for testing...")
    
    # Create directories
    os.makedirs("test-data/sqlite", exist_ok=True)
    
    # SQLite doesn't need a server, just file permissions
    print("SQLite setup complete - no server required")
    return True

def setup_postgresql():
    """Set up PostgreSQL for testing"""
    print("Setting up PostgreSQL for testing...")
    
    # Check if PostgreSQL is installed
    if not run_command("which psql", check=False):
        print("PostgreSQL not found. Install with:")
        print("  macOS: brew install postgresql")
        print("  Ubuntu: sudo apt-get install postgresql postgresql-client")
        print("  CentOS: sudo yum install postgresql postgresql-server")
        return False
    
    # Check if PostgreSQL is running
    if not run_command("pg_isready -h localhost -p 5432", check=False):
        print("Starting PostgreSQL...")
        if sys.platform == "darwin":  # macOS
            run_command("brew services start postgresql", check=False)
        else:  # Linux
            run_command("sudo systemctl start postgresql", check=False)
        
        # Wait for PostgreSQL to start
        for _ in range(30):
            if run_command("pg_isready -h localhost -p 5432", check=False):
                break
            time.sleep(1)
        else:
            print("PostgreSQL failed to start")
            return False
    
    # Create test database and user
    print("Creating test database and user...")
    
    # Create user and database
    run_command('createuser -h localhost dungeongate_test', check=False)
    run_command('createdb -h localhost -O dungeongate_test dungeongate_test', check=False)
    
    # Set password
    run_command("""psql -h localhost -c "ALTER USER dungeongate_test PASSWORD 'testpass123';" """, check=False)
    
    print("PostgreSQL setup complete")
    return True

def setup_postgresql_replica():
    """Set up PostgreSQL with replica for testing"""
    print("Setting up PostgreSQL with replica...")
    
    if not setup_postgresql():
        return False
    
    print("Note: Setting up a real read replica requires additional configuration.")
    print("For testing purposes, you can:")
    print("1. Run a second PostgreSQL instance on port 5433")
    print("2. Or modify the config to use reader_use_writer: true")
    
    # For simplicity, we'll just document the process
    print("To set up a real replica:")
    print("1. Configure postgresql.conf for replication")
    print("2. Create a replication user")
    print("3. Set up pg_hba.conf for replication")
    print("4. Create a standby server")
    
    return True

def setup_mysql():
    """Set up MySQL for testing"""
    print("Setting up MySQL for testing...")
    
    # Check if MySQL is installed
    if not run_command("which mysql", check=False):
        print("MySQL not found. Install with:")
        print("  macOS: brew install mysql")
        print("  Ubuntu: sudo apt-get install mysql-server mysql-client")
        print("  CentOS: sudo yum install mysql-server mysql")
        return False
    
    # Check if MySQL is running
    if not run_command("mysqladmin ping -h localhost", check=False):
        print("Starting MySQL...")
        if sys.platform == "darwin":  # macOS
            run_command("brew services start mysql", check=False)
        else:  # Linux
            run_command("sudo systemctl start mysql", check=False)
        
        # Wait for MySQL to start
        for _ in range(30):
            if run_command("mysqladmin ping -h localhost", check=False):
                break
            time.sleep(1)
        else:
            print("MySQL failed to start")
            return False
    
    # Create test database and user
    print("Creating test database and user...")
    
    mysql_commands = [
        "CREATE DATABASE IF NOT EXISTS dungeongate_mysql_test;",
        "CREATE USER IF NOT EXISTS 'dungeongate_mysql'@'localhost' IDENTIFIED BY 'mysql_test_pass';",
        "GRANT ALL PRIVILEGES ON dungeongate_mysql_test.* TO 'dungeongate_mysql'@'localhost';",
        "FLUSH PRIVILEGES;"
    ]
    
    for cmd in mysql_commands:
        run_command(f'mysql -h localhost -e "{cmd}"', check=False)
    
    print("MySQL setup complete")
    return True

def test_connection(db_type):
    """Test database connection"""
    print(f"Testing {db_type} connection...")
    
    if db_type == "sqlite":
        # SQLite just needs file access
        return os.access("test-data/sqlite", os.W_OK)
    
    elif db_type == "postgresql":
        return run_command(
            "psql -h localhost -U dungeongate_test -d dungeongate_test -c 'SELECT 1;'",
            check=False
        )
    
    elif db_type == "mysql":
        return run_command(
            "mysql -h localhost -u dungeongate_mysql -pmysql_test_pass dungeongate_mysql_test -e 'SELECT 1;'",
            check=False
        )
    
    return False

def create_env_file():
    """Create environment file for testing"""
    env_content = """# DungeonGate Testing Environment Variables
# Copy this to .env and modify as needed

# PostgreSQL
DB_PASSWORD=testpass123
POSTGRES_USER=dungeongate_test
POSTGRES_DB=dungeongate_test

# MySQL
MYSQL_PASSWORD=mysql_test_pass
MYSQL_USER=dungeongate_mysql
MYSQL_DATABASE=dungeongate_mysql_test

# Aurora Simulation
AURORA_DB_PASSWORD=aurora_test_pass

# Optional: Enable debug logging
DEBUG=true
LOG_LEVEL=debug
"""
    
    with open(".env.example", "w") as f:
        f.write(env_content)
    
    print("Created .env.example file with database credentials")

def main():
    parser = argparse.ArgumentParser(description="Set up databases for DungeonGate testing")
    parser.add_argument("database", choices=["sqlite", "postgresql", "mysql", "all"],
                       help="Database to set up")
    parser.add_argument("--test", action="store_true",
                       help="Test connection after setup")
    parser.add_argument("--replica", action="store_true",
                       help="Set up read replica (PostgreSQL only)")
    
    args = parser.parse_args()
    
    # Create environment file
    create_env_file()
    
    success = True
    
    if args.database == "all":
        databases = ["sqlite", "postgresql", "mysql"]
    else:
        databases = [args.database]
    
    for db in databases:
        print(f"\n{'='*50}")
        print(f"Setting up {db.upper()}")
        print('='*50)
        
        if db == "sqlite":
            success &= setup_sqlite()
        elif db == "postgresql":
            if args.replica:
                success &= setup_postgresql_replica()
            else:
                success &= setup_postgresql()
        elif db == "mysql":
            success &= setup_mysql()
        
        if args.test and success:
            print(f"\nTesting {db} connection...")
            if test_connection(db):
                print(f"✅ {db} connection test passed")
            else:
                print(f"❌ {db} connection test failed")
                success = False
    
    print(f"\n{'='*50}")
    if success:
        print("✅ Database setup completed successfully!")
        print("\nNext steps:")
        print("1. Copy .env.example to .env and adjust if needed")
        print("2. Run: ./scripts/test-database-configs.sh")
        print("3. Or test specific database: go run test-build.go -config configs/testing/sqlite-embedded.yaml")
    else:
        print("❌ Some database setups failed")
        sys.exit(1)

if __name__ == "__main__":
    main()
