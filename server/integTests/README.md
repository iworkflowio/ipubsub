# iPubSub Integration Tests

This directory contains comprehensive end-to-end integration tests for the iPubSub service.

## Overview

The integration tests start actual iPubSub service instances and test the full request/response flow via HTTP APIs. They validate:

- Single node basic send/receive functionality
- Circular buffer mode with message overwriting
- Blocking queue mode with timeout behavior
- Sync match queue (zero buffer) functionality
- Multi-node cluster with consistent hashing
- Concurrent send/receive scenarios
- Different message types (string, number, boolean, array, object, null)
- Error scenarios and edge cases
- Performance benchmarks

## Test Structure

### Test Utilities (`test_utils.go`)

- **TestCluster**: Manages multiple iPubSub service instances
- **TestNode**: Represents a single running service instance
- **Configuration**: Creates test configurations programmatically (no YAML files)
- **HTTP Helpers**: Send/receive message utilities
- **Port Management**: Automatic port assignment for isolation

### Key Features

1. **In-Memory Configuration**: All test configurations are created in Go code
2. **Service Integration**: Directly starts the iPubSub service via Go imports
3. **Port Isolation**: Each test uses different ports to avoid conflicts
4. **Automatic Cleanup**: Services are properly stopped after tests
5. **Comprehensive Coverage**: Tests all major iPubSub features

## Running Tests

### Run All Integration Tests
```bash
go test ./integTests -v
```

### Run Specific Test
```bash
go test ./integTests -v -run TestSingleNodeBasicSendReceive
```

### Run Performance Benchmark
```bash
go test ./integTests -bench=.
```

### Run with Race Detection
```bash
go test ./integTests -v -race
```

## Test Cases

### Single Node Tests
- `TestSingleNodeBasicSendReceive`: Basic send/receive flow
- `TestSingleNodeCircularBufferMode`: Circular buffer overwrite behavior
- `TestSingleNodeBlockingQueueMode`: Blocking queue timeout behavior
- `TestSingleNodeSyncMatchQueue`: Zero-buffer sync matching

### Multi-Node Tests
- `TestMultiNodeCluster`: Cluster formation and message routing
- `TestConcurrentSendReceive`: Concurrent operations stress test

### Message Type Tests
- `TestMessageWithDifferentTypes`: Various JSON data types

### API Tests
- `TestHealthEndpoint`: Health check functionality
- `TestRootEndpoint`: Root endpoint identification
- `TestErrorScenarios`: Error handling validation

### Performance Tests
- `BenchmarkSendReceive`: Performance benchmarking

## Configuration

Test configurations are created programmatically using the `CreateTestConfig` function:

```go
cfg := CreateTestConfig("test-node-1", 18001, 17001, []int{17001})
```

Parameters:
- Node name
- HTTP port
- Gossip port  
- Bootstrap ports (for cluster formation)

## Port Ranges

Tests use the following port ranges to avoid conflicts:
- HTTP: 18001-18099
- Gossip: 17001-17099

Each test uses different ports to enable parallel test execution.

## Error Handling

The tests include comprehensive error handling and validation:
- Service startup verification
- Health check validation
- HTTP status code verification
- Message content validation
- Timeout behavior validation
- Cluster stability verification

## Logging

Service logs are captured during test execution. In case of test failures, relevant logs are displayed for debugging.

## Dependencies

The integration tests use:
- Standard iPubSub service components
- Testify for assertions
- Go's built-in testing framework
- UUID generation for unique message IDs
- Zap logger for service logging

## Notes

- Tests create real service instances with full functionality
- Network communication happens over localhost
- Each test is isolated with unique ports
- Services are automatically cleaned up after tests
- Multi-node tests verify cluster membership and message routing
- Performance tests provide benchmarking data for optimization