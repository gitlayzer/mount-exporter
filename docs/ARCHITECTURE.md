# Architecture and Design Documentation

This document describes the architecture, design decisions, and internal components of Mount Exporter.

## Table of Contents

- [Overview](#overview)
- [System Architecture](#system-architecture)
- [Component Design](#component-design)
- [Data Flow](#data-flow)
- [Design Principles](#design-principles)
- [Configuration Architecture](#configuration-architecture)
- [Metrics System](#metrics-system)
- [Reliability Patterns](#reliability-patterns)
- [Security Architecture](#security-architecture)
- [Performance Considerations](#performance-considerations)
- [Extensibility](#extensibility)
- [Deployment Architecture](#deployment-architecture)

## Overview

Mount Exporter is a Prometheus exporter designed to monitor filesystem mount point availability. It follows a modular, microservices-inspired architecture with clear separation of concerns, comprehensive error handling, and production-ready reliability features.

### Key Goals

1. **Reliability**: Provide accurate mount point status monitoring
2. **Performance**: Minimal resource usage and fast response times
3. **Extensibility**: Easy to add new features and monitoring capabilities
4. **Maintainability**: Clean code structure and comprehensive testing
5. **Production-Ready**: Enterprise-grade reliability and security features

### Core Responsibilities

- Execute system commands to check mount point status
- Expose metrics in Prometheus-compatible format
- Provide health check endpoints
- Handle configuration management
- Ensure high availability and fault tolerance

## System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Mount Exporter                            │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────────┐  ┌─────────────────────────────────┐  │
│  │   HTTP Server   │  │      Resource Manager           │  │
│  │                 │  │                                 │  │
│  │ • /metrics      │  │ • Resource Registration        │  │
│  │ • /health       │  │ • Automatic Cleanup             │  │
│  │ • Security      │  │ • Memory Management             │  │
│  │ • Middleware    │  │ • Background GC                 │  │
│  └─────────────────┘  └─────────────────────────────────┘  │
│           │                       │                         │
│           ▼                       ▼                         │
│  ┌─────────────────┐  ┌─────────────────────────────────┐  │
│  │ Metrics Collector│  │       Panic Recovery            │  │
│  │                 │  │                                 │  │
│  │ • Prometheus    │  │ • Global Panic Handler          │  │
│  │ • Registry      │  │ • Goroutine Safety              │  │
│  │ • Scrape Logic  │  │ • Stack Trace Capture           │  │
│  │ • Error Handling│  │ • Statistics Tracking            │  │
│  └─────────────────┘  └─────────────────────────────────┘  │
│           │                       │                         │
│           ▼                       ▼                         │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │              System Integration Layer                   │  │
│  │                                                         │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌──────────────┐   │  │
│  │  │FindmntWrapper│  │Circuit Breaker│  │  Retry Logic │   │  │
│  │  │             │  │             │  │              │   │  │
│  │  │• Command Exec│  │• Failure    │  │• Backoff     │   │  │
│  │  │• Output Parse│  │  Detection  │  │• Jitter      │   │  │
│  │  │• Timeout     │  │• Auto-Reset │  │• Transient   │   │  │
│  │  │• Error Handle│  │• Statistics │  │  Detection   │   │  │
│  │  └─────────────┘  └─────────────┘  └──────────────┘   │  │
│  └─────────────────────────────────────────────────────────┘  │
│                             │                                 │
│                             ▼                                 │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │                   Configuration Layer                     │  │
│  │                                                         │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌──────────────┐   │  │
│  │  │   Config    │  │ConfigWatcher│  │   Validation  │   │  │
│  │  │   Manager   │  │             │  │              │   │  │
│  │  │             │  │• File Watch │  │• Type Checking│   │  │
│  │  │• YAML Parse │  │• Hot Reload │  │• Range Verify │   │  │
│  │  │• Env Override│  │• Callbacks  │  │• Error Report│   │  │
│  │  │• Defaults    │  │• Atomic     │  │              │   │  │
│  │  └─────────────┘  └─────────────┘  └──────────────┘   │  │
│  └─────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

## Component Design

### 1. Main Application (`main.go`)

**Responsibilities:**
- Application lifecycle management
- Signal handling and graceful shutdown
- Panic recovery coordination
- Dependency injection and component orchestration

**Key Features:**
- Global panic recovery with stack trace capture
- Graceful shutdown handling for SIGINT/SIGTERM
- Configuration loading and validation
- Component initialization and wiring

### 2. Configuration System (`config/`)

**Components:**
- `Config`: Main configuration structure
- `ConfigWatcher`: Hot-reload functionality
- Validation and default handling

**Design Patterns:**
- Builder pattern for configuration construction
- Observer pattern for configuration changes
- Strategy pattern for validation rules

### 3. HTTP Server (`server/`)

**Components:**
- `Server`: Main HTTP server implementation
- Route handlers for metrics and health endpoints
- Security middleware
- Resource management integration

**Architecture:**
- Middleware chain pattern for request processing
- Dependency injection for services
- Graceful shutdown with context cancellation

### 4. Metrics Collection (`metrics/`)

**Components:**
- `Collector`: Prometheus metrics collector
- Metric definitions and registry management
- Collection orchestration

**Design:**
- Prometheus client library integration
- Thread-safe metric updates
- Error handling and health monitoring

### 5. System Integration (`system/`)

**Components:**
- `FindmntWrapper`: System command execution
- Output parsing and error handling
- Reliability patterns integration

**Features:**
- Context-aware command execution
- Timeout management
- Concurrent processing capabilities

### 6. Reliability Package (`reliability/`)

**Components:**
- `CircuitBreaker`: Failure detection and prevention
- `Retry`: Transient error handling
- Backoff strategies

**Patterns:**
- Circuit breaker pattern for fault tolerance
- Retry pattern with exponential backoff
- Statistics tracking and monitoring

### 7. Recovery Package (`recovery/`)

**Components:**
- `PanicHandler`: Panic detection and recovery
- Stack trace capture
- Statistics tracking

**Features:**
- Global panic recovery
- Goroutine-safe operations
- Configurable panic handlers

### 8. Resource Management (`resources/`)

**Components:**
- `ResourceManager`: Resource lifecycle management
- Cleanup orchestration
- Memory monitoring

**Responsibilities:**
- Automatic resource cleanup
- Background garbage collection
- Resource usage tracking

## Data Flow

### 1. Startup Flow

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   main()    │───▶│ Load Config │───▶│ Validate    │───▶│ Initialize   │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
                                                                       │
                                                                       ▼
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│ Start Server │───▶│ Register    │───▶│ Begin       │───▶│ Wait for     │
│             │    │ Resources   │    │ Collection  │    │ Signals      │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
```

### 2. Metrics Collection Flow

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│ Prometheus   │───▶│ HTTP Server │───▶│ Metrics     │───▶│ Findmnt     │
│   Scrape     │    │   Request   │    │ Collector   │    │ Wrapper      │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
                                                                       │
                                                                       ▼
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Return    │◀───│   Update    │◀───│   Parse     │◀───│ Execute     │
│  Metrics    │    │ Prometheus  │    │   Output    │    │ findmnt      │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
     │                    │                    │                    │
     ▼                    ▼                    ▼                    ▼
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│ Circuit     │    │   Retry      │    │   Panic      │    │   Resource   │
│ Breaker     │    │   Logic      │    │ Recovery    │    │ Management   │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
```

### 3. Configuration Reload Flow

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│ Config File │───▶│ File Watcher │───▶│ Detect      │───▶│ Read New    │
│   Changed   │    │   Trigger    │    │ Change      │    │ Config       │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
                                                                       │
                                                                       ▼
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Apply     │───▶│ Notify      │───▶│ Update      │───▶│ Restart     │
│   New Config│    │ Callbacks   │    │ Components   │    │ Collection   │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
```

## Design Principles

### 1. Single Responsibility Principle

Each component has a clear, single responsibility:
- **Config**: Configuration management only
- **Server**: HTTP request handling only
- **Metrics**: Prometheus integration only
- **System**: External command execution only

### 2. Dependency Inversion

High-level modules don't depend on low-level modules:
- Server depends on Collector interface
- Collector depends on system interface abstraction
- Configuration uses interface-based callbacks

### 3. Interface Segregation

Small, focused interfaces:
```go
type MountChecker interface {
    CheckMountPoint(ctx context.Context, mountPoint string) *FindmntResult
}

type MetricsCollector interface {
    Collect() ([]byte, error)
}

type ResourceManager interface {
    Register(id string, cleanup func() error)
}
```

### 4. Open/Closed Principle

Open for extension, closed for modification:
- New metric types via interface implementation
- New reliability patterns via composition
- New configuration sources via interface extension

## Configuration Architecture

### Layered Configuration

```
┌─────────────────────────────────────────────────────────┐
│                Configuration Hierarchy                   │
├─────────────────────────────────────────────────────────┤
│ 1. Default Values (lowest priority)                      │
│ 2. Configuration File                                    │
│ 3. Environment Variables                                 │
│ 4. Command Line Flags (highest priority)                 │
└─────────────────────────────────────────────────────────┘
```

### Hot Reload Design

```go
type ConfigWatcher struct {
    config     *Config
    callbacks  []func(*Config)
    fileWatcher *fsnotify.Watcher
    mutex      sync.RWMutex
}

// Callback registration for configuration changes
watcher.AddCallback(func(newConfig *Config) {
    // React to configuration changes
    updateCollector(newConfig)
    restartCollection(newConfig)
})
```

### Validation Strategy

Multi-layer validation:
1. **Struct-level**: Type validation via Go type system
2. **Value-level**: Range and format validation
3. **Semantic-level**: Business logic validation
4. **Runtime-level**: Live configuration validation

## Metrics System Architecture

### Prometheus Integration

```go
type Collector struct {
    config     *config.Config
    findmnt    *system.FindmntWrapper

    // Prometheus metrics
    mountStatus *prometheus.Desc
    scrapeTime  *prometheus.Desc
    upMetric   *prometheus.Desc
    registry   *prometheus.Registry
}
```

### Metric Collection Strategy

1. **Eager Collection**: Collect all metrics on scrape
2. **Caching**: Short-term caching to prevent excessive system calls
3. **Parallel Processing**: Concurrent mount point checking
4. **Error Isolation**: Failed checks don't affect others

### Label Strategy

Consistent labeling across all metrics:
- `mount_point`: Mount point path
- `fs_type`: Filesystem type
- `source`: Mount source device
- `target`: Mount target
- `error`: Error information (when applicable)

## Reliability Patterns

### Circuit Breaker Implementation

```go
type CircuitBreaker struct {
    state         State
    failures      int
    maxFailures   int
    resetTimeout  time.Duration
    lastFailTime  time.Time
    mutex         sync.RWMutex
}
```

**States:**
- **CLOSED**: Normal operation, requests pass through
- **OPEN**: All requests fail immediately
- **HALF_OPEN**: Limited requests allowed to test recovery

### Retry Pattern

```go
type Retry struct {
    maxAttempts int
    backoff     BackoffStrategy
    jitter      bool
    shouldRetry func(error) bool
}
```

**Strategies:**
- **Linear**: Fixed delay between attempts
- **Exponential**: Increasing delay with multiplier
- **Fixed**: Constant delay

### Panic Recovery

```go
type PanicHandler struct {
    handlers   []PanicHandlerFunc
    logger     Logger
    enabled    bool
    statistics map[string]int64
}
```

**Features:**
- Global panic capture
- Stack trace preservation
- Non-blocking recovery handlers
- Statistics tracking

## Security Architecture

### Security Layers

1. **Application Level**
   - Input validation
   - Error message sanitization
   - Secure defaults

2. **Network Level**
   - Security headers
   - CORS considerations
   - Rate limiting readiness

3. **System Level**
   - Non-root execution
   - Privilege separation
   - Resource limits

### Defense in Depth

```
┌─────────────────────────────────────────────────────────┐
│                    Security Layers                      │
├─────────────────────────────────────────────────────────┤
│ Network Security (Firewall, Segmentation)               │
├─────────────────────────────────────────────────────────┤
│ Application Security (Input Validation, Headers)         │
├─────────────────────────────────────────────────────────┤
│ System Security (Non-root, Resource Limits)             │
├─────────────────────────────────────────────────────────┤
│ Container Security (Read-only, Minimal Base)            │
└─────────────────────────────────────────────────────────┘
```

## Performance Considerations

### Resource Usage Optimization

1. **Memory Management**
   - Minimal heap allocation
   - Regular garbage collection
   - Resource cleanup monitoring

2. **CPU Efficiency**
   - Concurrent mount point checking
   - Optimized command execution
   - Caching where appropriate

3. **I/O Optimization**
   - Non-blocking operations
   - Connection pooling (if applicable)
   - Efficient serialization

### Scalability Design

```go
// Concurrent mount point checking
func (f *FindmntWrapper) CheckMultipleMountPoints(ctx context.Context, mountPoints []string) []*FindmntResult {
    results := make([]*FindmntResult, len(mountPoints))

    var wg sync.WaitGroup
    for i, mountPoint := range mountPoints {
        wg.Add(1)
        go func(index int, mp string) {
            defer wg.Done()
            results[index] = f.CheckMountPoint(ctx, mp)
        }(i, mountPoint)
    }

    wg.Wait()
    return results
}
```

### Performance Monitoring

Built-in performance metrics:
- Scrape duration tracking
- Memory usage monitoring
- Goroutine count tracking
- Resource utilization statistics

## Extensibility

### Plugin Architecture

The system is designed for easy extension:

1. **New Metrics Types**
   ```go
   type CustomMetricsCollector interface {
       Collect() ([]prometheus.Metric, error)
   }
   ```

2. **New System Commands**
   ```go
   type SystemCommand interface {
       Execute(ctx context.Context, args ...string) ([]byte, error)
   }
   ```

3. **New Configuration Sources**
   ```go
   type ConfigLoader interface {
       Load() (*Config, error)
   }
   ```

### Extension Points

- **Custom Metric Collectors**: Implement additional monitoring
- **Alternative System Commands**: Support different platforms
- **Configuration Sources**: Environment variables, etcd, etc.
- **Output Formats**: Beyond Prometheus (if needed)

### Future Enhancements

Planned extension capabilities:
1. **Push-based Metrics**: Support for remote_write
2. **Advanced Filtering**: Label-based metric filtering
3. **Custom Alerting**: Built-in alert rule evaluation
4. **Plugin System**: Dynamic loading of extensions

## Deployment Architecture

### Deployment Patterns

1. **Single Instance**
   - Direct binary deployment
   - Systemd service management
   - Local configuration files

2. **Containerized**
   - Docker container with health checks
   - Kubernetes deployment with ConfigMaps
   - Sidecar pattern for logging

3. **Distributed**
   - Multiple instances for high availability
   - Load balancer distribution
   - Service discovery integration

### High Availability Design

```go
// Graceful shutdown with context cancellation
func (s *Server) Stop(ctx context.Context) error {
    shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    return s.httpServer.Shutdown(shutdownCtx)
}
```

### Configuration Management

**Environment-specific configurations:**
- Development: Local files, debug logging
- Staging: Container images, structured logging
- Production: Managed configs, security headers

### Monitoring Integration

**Prometheus Ecosystem:**
- Metrics exposition
- Alert rule integration
- Service discovery support

**Observability Stack:**
- Structured logging
- Distributed tracing readiness
- Metrics correlation

## Design Trade-offs

### Decisions Made

1. **Go Language Choice**
   - **Pros**: Strong concurrency, single binary deployment, rich ecosystem
   - **Cons**: Memory usage, GC pauses
   - **Decision**: Go provides the best balance for this use case

2. **Prometheus-Only Approach**
   - **Pros**: Standard integration, ecosystem support
   - **Cons**: Limited to pull-based monitoring
   - **Decision**: Focus on excellence in one monitoring system

3. **findmnt Dependency**
   - **Pros**: Standard Linux tool, reliable output
   - **Cons**: Platform dependency, external command overhead
   - **Decision**: Accept dependency for simplicity and reliability

4. **Configuration Hot-Reload**
   - **Pros**: Zero-downtime updates
   - **Cons**: Complexity, potential race conditions
   - **Decision**: Implement for production readiness

### Alternative Designs Considered

1. **In-Kernel Monitoring**
   - Direct filesystem monitoring
   - Rejected due to complexity and security concerns

2. **Multiple Output Formats**
   - Support for JSON, InfluxDB, etc.
   - Rejected to maintain focus and simplicity

3. **Built-in Web UI**
   - Dashboard and visualization
   - Rejected to leverage existing tools (Grafana)

## Quality Assurance

### Testing Strategy

```
┌─────────────────────────────────────────────────────────┐
│                    Testing Pyramid                       │
├─────────────────────────────────────────────────────────┤
│                 E2E Tests (Few)                          │
│               Integration Tests                          │
│              Component/Unit Tests (Many)                 │
└─────────────────────────────────────────────────────────┘
```

### Code Quality

- **Static Analysis**: golangci-lint configuration
- **Security Scanning**: Gosec integration
- **Coverage Requirements**: Minimum 80% test coverage
- **Documentation**: Comprehensive Go doc comments

### CI/CD Integration

Automated quality gates:
1. **Unit Tests**: All packages, high coverage
2. **Integration Tests**: Full workflow testing
3. **Security Scans**: Vulnerability detection
4. **Performance Tests**: Regression prevention
5. **Documentation**: API docs generation

This architecture provides a solid foundation for a production-ready, maintainable, and extensible mount monitoring solution.