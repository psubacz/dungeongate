# DungeonGate Monitoring Guide

This guide describes the Prometheus metrics exposed by DungeonGate services for monitoring and alerting.

## Metrics Endpoint

All services expose Prometheus metrics at:
- **HTTP Endpoint**: `/metrics`
- **Default Port**: 8083 (configurable)

## SSH Server Metrics

### Connection Metrics

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `dungeongate_ssh_connections_total` | Counter | Total number of SSH connections | None |
| `dungeongate_ssh_connections_active` | Gauge | Number of currently active SSH connections | None |
| `dungeongate_ssh_connections_failed_total` | Counter | Total number of failed SSH connections | None |
| `dungeongate_ssh_connection_duration_seconds` | Histogram | SSH connection duration in seconds | None |

### Session Metrics

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `dungeongate_ssh_sessions_total` | Counter | Total number of SSH sessions | None |
| `dungeongate_ssh_sessions_active` | Gauge | Number of currently active SSH sessions | None |
| `dungeongate_ssh_session_duration_seconds` | Histogram | SSH session duration in seconds | None |
| `dungeongate_ssh_session_bytes_read_total` | Counter | Total bytes read from SSH sessions | None |
| `dungeongate_ssh_session_bytes_written_total` | Counter | Total bytes written to SSH sessions | None |

### Authentication Metrics

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `dungeongate_ssh_auth_attempts_total` | Counter | Total number of authentication attempts | `method`, `username` |
| `dungeongate_ssh_auth_failures_total` | Counter | Total number of authentication failures | `method`, `reason` |
| `dungeongate_ssh_auth_duration_seconds` | Histogram | Authentication duration in seconds | None |

### Game Metrics

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `dungeongate_ssh_games_started_total` | Counter | Total number of games started | `game_id`, `game_name` |
| `dungeongate_ssh_games_active` | Gauge | Number of currently active game sessions | `game_id`, `game_name` |
| `dungeongate_ssh_game_duration_seconds` | Histogram | Game session duration in seconds | `game_id`, `game_name` |
| `dungeongate_ssh_game_session_errors_total` | Counter | Total number of game session errors | `game_id`, `error_type` |

### Terminal Metrics

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `dungeongate_ssh_terminal_size_changes_total` | Counter | Total number of terminal size changes | None |
| `dungeongate_ssh_terminal_types_total` | Counter | Terminal types used for connections | `type` |

## Service Health Metrics

### Build Information

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `dungeongate_build_info` | Gauge | Build information (always 1) | `version`, `commit`, `build_time` |
| `dungeongate_start_time_seconds` | Gauge | Unix timestamp of service start time | None |

### HTTP Metrics

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `dungeongate_http_requests_total` | Counter | Total number of HTTP requests | `method`, `path`, `status` |
| `dungeongate_http_request_duration_seconds` | Histogram | HTTP request duration in seconds | `method`, `path` |
| `dungeongate_http_response_size_bytes` | Histogram | HTTP response size in bytes | `method`, `path` |

### Database Metrics

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `dungeongate_database_connections_active` | Gauge | Number of active database connections | None |
| `dungeongate_database_queries_total` | Counter | Total number of database queries | `query_type`, `table` |
| `dungeongate_database_query_duration_seconds` | Histogram | Database query duration in seconds | `query_type` |
| `dungeongate_database_errors_total` | Counter | Total number of database errors | `error_type` |

## Example Prometheus Configuration

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'dungeongate'
    static_configs:
      - targets: ['localhost:8083']
```

## Example Alerting Rules

```yaml
groups:
  - name: dungeongate_alerts
    rules:
      # Alert when SSH connections are failing
      - alert: HighSSHConnectionFailureRate
        expr: rate(dungeongate_ssh_connections_failed_total[5m]) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High SSH connection failure rate"
          description: "SSH connection failure rate is {{ $value }} per second"

      # Alert when too many active game sessions
      - alert: HighActiveGameSessions
        expr: sum(dungeongate_ssh_games_active) > 100
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High number of active game sessions"
          description: "{{ $value }} active game sessions"

      # Alert on authentication failures
      - alert: HighAuthenticationFailureRate
        expr: rate(dungeongate_ssh_auth_failures_total[5m]) > 0.5
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High authentication failure rate"
          description: "Authentication failure rate is {{ $value }} per second"

      # Alert on database connection issues
      - alert: DatabaseConnectionPoolExhausted
        expr: dungeongate_database_connections_active > 20
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Database connection pool near exhaustion"
          description: "{{ $value }} active database connections"
```

## Grafana Dashboard

A sample Grafana dashboard JSON is available at `./dashboards/dungeongate.json` which includes:

- SSH connection overview
- Authentication success/failure rates
- Active games by type
- Terminal type distribution
- Database performance metrics
- HTTP endpoint performance

## Common Queries

### SSH Connection Rate
```promql
rate(dungeongate_ssh_connections_total[5m])
```

### Authentication Success Rate
```promql
rate(dungeongate_ssh_auth_attempts_total[5m]) - rate(dungeongate_ssh_auth_failures_total[5m])
```

### Average Game Session Duration
```promql
rate(dungeongate_ssh_game_duration_seconds_sum[5m]) / rate(dungeongate_ssh_game_duration_seconds_count[5m])
```

### Most Popular Games
```promql
topk(5, sum by (game_name) (dungeongate_ssh_games_started_total))
```

### Terminal Type Distribution
```promql
sum by (type) (dungeongate_ssh_terminal_types_total)
```

## Monitoring Best Practices

1. **Set up alerts** for critical metrics like authentication failures and connection errors
2. **Monitor trends** in game usage to plan capacity
3. **Track terminal types** to ensure compatibility
4. **Watch database performance** to prevent bottlenecks
5. **Use histograms** to identify performance outliers

## Integration with Cloud Providers

### AWS CloudWatch
Use the CloudWatch Prometheus exporter to send metrics to CloudWatch.

### Google Cloud Monitoring
Use the Stackdriver Prometheus sidecar to send metrics to Cloud Monitoring.

### Azure Monitor
Use the Azure Monitor for containers to collect Prometheus metrics.