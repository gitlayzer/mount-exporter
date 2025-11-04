# Mount Exporter

A Prometheus exporter for monitoring filesystem mount point availability by calling the system `findmnt` command and exposing metrics indicating whether configured mount points are currently active.

## Overview

Mount Exporter provides critical monitoring for storage infrastructure by:
- Monitoring configured filesystem mount points
- Exposing Prometheus-compatible metrics
- Supporting automated alerting when mount points become unavailable
- Running as a single binary with minimal dependencies

## Features

- ✅ **Mount Point Monitoring**: Uses `findmnt` command to verify mount status
- ✅ **Prometheus Metrics**: Exposes metrics in standard Prometheus format
- ✅ **Configuration Management**: YAML-based configuration with validation
- ✅ **Health Checks**: Built-in health endpoint for monitoring
- ✅ **Structured Logging**: Configurable logging levels and formats
- ✅ **Graceful Shutdown**: Proper signal handling and cleanup
- ✅ **Container Ready**: Docker support with health checks
- ✅ **Production Ready**: systemd service template included

## Quick Start

### Binary Installation

1. **Download the latest release**
   ```bash
   wget https://github.com/mount-exporter/mount-exporter/releases/latest/download/mount-exporter-linux-amd64
   chmod +x mount-exporter-linux-amd64
   sudo mv mount-exporter-linux-amd64 /usr/local/bin/mount-exporter
   ```

2. **Create configuration**
   ```bash
   sudo mkdir -p /etc/mount-exporter
   sudo cp config.yaml /etc/mount-exporter/config.yaml
   sudo nano /etc/mount-exporter/config.yaml
   ```

3. **Run the exporter**
   ```bash
   mount-exporter -config /etc/mount-exporter/config.yaml
   ```

### Docker Installation

```bash
# Build the image
docker build -t mount-exporter:latest .

# Run with default configuration
docker run -d \
  --name mount-exporter \
  -p 8080:8080 \
  -v $(pwd)/examples/config.yaml:/etc/mount-exporter/config.yaml:ro \
  mount-exporter:latest

# Or with custom configuration
docker run -d \
  --name mount-exporter \
  -p 8080:8080 \
  -v /path/to/your/config.yaml:/etc/mount-exporter/config.yaml:ro \
  mount-exporter:latest
```

### Systemd Service

```bash
# Copy the service file
sudo cp examples/mount-exporter.service /etc/systemd/system/
sudo cp examples/config.yaml /etc/mount-exporter/

# Create mount-exporter user
sudo useradd -r -s /bin/false mount-exporter
sudo chown -R mount-exporter:mount-exporter /etc/mount-exporter

# Enable and start the service
sudo systemctl daemon-reload
sudo systemctl enable mount-exporter
sudo systemctl start mount-exporter
sudo systemctl status mount-exporter
```

## Configuration

The exporter uses YAML configuration files. Here's an example:

```yaml
# server configuration
server:
  host: "0.0.0.0"      # Bind address
  port: 8080           # Port to listen on
  path: "/metrics"      # Metrics endpoint path

# Mount points to monitor
mount_points:
  - "/data"            # Data directory
  - "/var/log"         # System logs
  - "/mnt/backups"     # Backup mount point
  - "/home"            # User home directories

# Collection interval
interval: 30s          # How often to check mount points

# Logging configuration
logging:
  level: "info"        # Log level: debug, info, warn, error, fatal
  format: "json"       # Log format: json, text
```

### Environment Variables

You can override configuration using environment variables:

- `MOUNT_EXPORTER_HOST` - Override server host
- `MOUNT_EXPORTER_PORT` - Override server port
- `MOUNT_EXPORTER_PATH` - Override metrics path
- `MOUNT_EXPORTER_INTERVAL` - Override collection interval
- `MOUNT_EXPORTER_LOG_LEVEL` - Override log level

Example:
```bash
export MOUNT_EXPORTER_PORT=9090
export MOUNT_EXPORTER_LOG_LEVEL=debug
mount-exporter
```

## Metrics

The exporter exposes the following metrics on `/metrics`:

### Mount Point Status
```
# HELP mount_exporter_mount_point_status Mount point availability status (1=mounted, 0=not mounted)
# TYPE mount_exporter_mount_point_status gauge
mount_exporter_mount_point_status{mount_point="/data"} 1
mount_exporter_mount_point_status{mount_point="/var/log"} 1
mount_exporter_mount_point_status{mount_point="/mnt/backups"} 0
```

### Scrape Duration
```
# HELP mount_exporter_scrape_duration_seconds Time spent scraping mount point status
# TYPE mount_exporter_scrape_duration_seconds gauge
mount_exporter_scrape_duration_seconds{mount_point="/data"} 0.045
mount_exporter_scrape_duration_seconds{mount_point="/var/log"} 0.032
```

### Application Health
```
# HELP mount_exporter_up Whether the mount exporter is healthy (1=healthy, 0=unhealthy)
# TYPE mount_exporter_up gauge
mount_exporter_up 1
```

### Scrape Success
```
# HELP mount_exporter_scrape_success_total Total number of successful scrapes
# TYPE mount_exporter_scrape_success_total counter
mount_exporter_scrape_success_total{mount_point="/data"} 125
mount_exporter_scrape_success_total{mount_point="/var/log"} 125
```

## Endpoints

- `/metrics` - Prometheus metrics endpoint
- `/health` - Health check endpoint (JSON format)
- `/healthz` - Alternative health check endpoint
- `/` - Basic information page

## Health Check

The `/health` endpoint returns JSON response:

```json
{"status": "healthy"}
```

If `findmnt` is not available or there are other issues:
```json
{"status": "unhealthy", "error": "findmnt command not available"}
```

## Command Line Options

```bash
mount-exporter [OPTIONS]

Options:
  -config string
        Path to configuration file (default: searches for config.yaml in common locations)
  -log-level string
        Override log level (debug, info, warn, error, fatal)
  -help
        Show help message
  -version
        Show version information
```

## Configuration File Locations

The exporter looks for configuration files in this order:

1. `config.yaml` (current directory)
2. `config.yml` (current directory)
3. `examples/config.yaml`
4. `examples/config.yml`
5. `/etc/mount-exporter/config.yaml`
6. `/etc/mount-exporter/config.yml`

## Troubleshooting

### Common Issues

1. **Permission Denied**
   ```bash
   # Ensure the binary has execute permissions
   chmod +x mount-exporter
   ```

2. **findmnt Command Not Found**
   ```bash
   # Install util-linux package (Ubuntu/Debian)
   sudo apt-get update && sudo apt-get install util-linux

   # Install util-linux package (CentOS/RHEL)
   sudo yum install util-linux
   ```

3. **Configuration Errors**
   ```bash
   # Validate configuration
   mount-exporter -config config.yaml -log-level debug
   ```

4. **Port Already in Use**
   ```bash
   # Check what's using the port
   sudo netstat -tulpn | grep :8080

   # Use different port
   mount-exporter -config config.yaml
   # Set MOUNT_EXPORTER_PORT=9090 in config.yaml or environment variable
   ```

### Logs

Check logs for detailed information:

```bash
# With systemd
sudo journalctl -u mount-exporter -f

# With Docker
docker logs -f mount-exporter

# Run in foreground for debugging
mount-exporter -log-level debug
```

## Development

### Building from Source

```bash
# Clone the repository
git clone https://github.com/mount-exporter/mount-exporter.git
cd mount-exporter

# Build the binary
make build

# Run tests
make test

# Run with coverage
make test-coverage
```

### Project Structure

```
mount-exporter/
├── main.go              # Application entry point
├── config/              # Configuration package
│   ├── config.go        # Configuration handling
│   └── config_test.go   # Configuration tests
├── metrics/             # Metrics package
│   ├── collector.go     # Prometheus collector
│   └── collector_test.go # Metrics tests
├── server/              # HTTP server package
│   ├── server.go       # HTTP server
│   └── server_test.go  # Server tests
├── system/              # System integration
│   ├── findmnt.go      # findmnt wrapper
│   └── findmnt_test.go # System tests
├── test/                # Integration tests
│   └── integration_test.go
├── examples/            # Example configurations
│   ├── config.yaml      # Example config
│   └── mount-exporter.service
├── Dockerfile           # Container definition
├── Makefile             # Build automation
└── README.md            # This file
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Support

- 📖 [Documentation](https://github.com/mount-exporter/mount-exporter/wiki)
- 🐛 [Issues](https://github.com/mount-exporter/mount-exporter/issues)
- 💬 [Discussions](https://github.com/mount-exporter/mount-exporter/discussions)