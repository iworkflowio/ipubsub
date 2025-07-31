# Async Output Service - Requirements Specification

## 1. Overview

The Async Output Service solves the core problem of connecting asynchronous output generation with real-time client consumption. When applications generate outputs asynchronously (e.g., in background processes), they need a way to deliver these outputs to specific clients who are actively waiting for them in real-time.

The service acts as a matching intermediary between:
- **Applications** generating outputs asynchronously without direct client connections
- **Clients** waiting to receive specific outputs immediately as they become available

## 2. Core Problem Statement

### 2.1 The Challenge
- Applications run background processes that generate outputs asynchronously
- Clients need to receive these outputs in real-time as they become available
- There's no direct connection between the async application and the waiting client
- Thousands of output instances may be generated concurrently
- Thousands of clients may be waiting for different specific outputs simultaneously

### 2.2 Key Use Cases
- **AI Agent Progress Updates**: AI agents running background processes (LLM interactions, tool executions) need to stream progress and intermediate results to users
- **Long-running Task Results**: Applications processing lengthy operations need to deliver results as they become available
- **Real-time Notifications**: Background services generating events that specific clients are waiting to receive

## 3. Solution Architecture

### 3.1 Core Concept: Stream-based Matching
- **Stream ID**: Unique identifier that connects output senders with receivers
- **Real-time Matching**: Service matches `send` operations with `receive` operations in real-time
- **Long Polling**: Both send and receive operations use long polling to wait for matching
- **Timeout Handling**: Configurable timeouts with 424 error responses when no match occurs

### 3.2 Two-Phase Evolution
- **Phase 1**: Real-time in-memory matching (current focus)
- **Phase 2**: Persistent storage with replay capability

## 4. Functional Requirements - Phase 1 (Real-time Matching)

### 4.1 Send API
- **FR-1.1**: Applications can send output with a streamID via long polling API
- **FR-1.2**: Send operation waits for a matching client receiver (configurable timeout)
- **FR-1.3**: Returns 424 error if no client is waiting within timeout period
- **FR-1.4**: Returns success when output is successfully delivered to waiting client
- **FR-1.5**: Support multiple output formats (text, JSON, binary)

### 4.2 Receive API
- **FR-2.1**: Clients can receive output for a specific streamID via long polling API
- **FR-2.2**: Receive operation waits for matching output (configurable timeout)
- **FR-2.3**: Returns 424 error if no output arrives within timeout period
- **FR-2.4**: Returns output immediately when available
- **FR-2.5**: Support streaming multiple outputs on the same streamID

### 4.3 Stream Management
- **FR-3.1**: StreamID-based output routing and matching
- **FR-3.2**: Support thousands of concurrent streams
- **FR-3.3**: Efficient matching algorithm for real-time performance
- **FR-3.4**: Stream lifecycle management (creation, active, cleanup)
- **FR-3.5**: Multiple clients can wait for the same streamID (broadcast)

### 4.4 Error Handling
- **FR-4.1**: 424 (Failed Dependency) for timeout scenarios
- **FR-4.2**: Graceful handling of client disconnections
- **FR-4.3**: Retry mechanisms for failed deliveries
- **FR-4.4**: Connection health monitoring

## 5. Functional Requirements - Phase 2 (Persistence & Replay)

### 5.1 Send and Store API
- **FR-5.1**: `sendAndStore` API that persists output without waiting for clients
- **FR-5.2**: Immediate return after successful storage
- **FR-5.3**: Output versioning and ordering within streams
- **FR-5.4**: Configurable retention policies per stream

### 5.2 Enhanced Receive API
- **FR-6.1**: Resume token support for reading from specific points in output history
- **FR-6.2**: Replay capability from any point in the stream
- **FR-6.3**: Historical output retrieval
- **FR-6.4**: Stream position tracking and management

### 5.3 Persistence Layer
- **FR-7.1**: Durable storage of outputs across multiple database types
- **FR-7.2**: Stream-based partitioning for scalability
- **FR-7.3**: Efficient querying by streamID and position
- **FR-7.4**: Data compression for large outputs

## 6. API Specification

### 6.1 Send API (Phase 1)
```
POST /api/v1/streams/send
{
  "streamId": "string",
  "outputUuid": "<UUID>",
  "output": object,
  "timeout": "30s"
}

Responses:
200 - Output delivered to waiting client
424 - No client waiting (timeout exceeded)
400 - Invalid request
500 - Server error
```

### 6.2 Receive API
```
GET /api/v1/streams/receive?streamId=value&timeout=30s&resumeToken=token

Responses:
200 - Output received
{
  "outputUuid": "<UUID>",
  "output": object,
  "timestamp": "2024-01-01T10:00:00Z",
  "resumeToken": "def456"
}

424 - No output available (timeout exceeded)
400 - Invalid request
500 - Server error
```

### 6.3 Send and Store API (Phase 2)
```
POST /api/v1/streams/sendAndStore
{
  "outputUuid": "<UUID>",    
  "streamId": "string",
  "output": object,
  "ttl": "24h"
}

Response:
201 - Output stored successfully
400 - Invalid request
500 - Server error
```

## 7. Non-Functional Requirements

### 7.1 Performance
- **NFR-1.1**: Support 10,000+ concurrent streams
- **NFR-1.2**: Sub-10ms matching latency for real-time operations
- **NFR-1.3**: Handle 1,000+ send/receive operations per second
- **NFR-1.4**: Support outputs up to 10MB in size
- **NFR-1.5**: Memory-efficient handling of waiting connections

### 7.2 Scalability
- **NFR-2.1**: Horizontal scaling through stream partitioning
- **NFR-2.2**: Support for multiple service instances
- **NFR-2.3**: Efficient load distribution across instances
- **NFR-2.4**: Linear scaling with additional resources

### 7.3 Reliability
- **NFR-3.1**: 99.9% availability for matching operations
- **NFR-3.2**: Graceful degradation under high load
- **NFR-3.3**: Connection failover and recovery
- **NFR-3.4**: Data integrity for stored outputs (Phase 2)

### 7.4 Real-time Characteristics
- **NFR-4.1**: Real-time matching with minimal buffering
- **NFR-4.2**: Configurable timeout ranges (1s to 300s)
- **NFR-4.3**: Efficient connection management for long polling
- **NFR-4.4**: Low resource overhead for idle connections

## 8. Database Requirements (Phase 2)

### 8.1 Supported Databases
- **DR-1.1**: Cassandra - For high-write throughput and time-series data
- **DR-1.2**: MongoDB - For flexible document storage and indexing
- **DR-1.3**: DynamoDB - For managed scaling and serverless deployment
- **DR-1.4**: MySQL - For ACID compliance and structured data
- **DR-1.5**: PostgreSQL - For advanced features and JSON support

### 8.2 Schema Design
- **DR-2.1**: Stream-based partitioning for horizontal scaling
- **DR-2.2**: Position-based ordering within streams
- **DR-2.3**: Efficient querying by streamID and position range
- **DR-2.4**: Metadata indexing for search and filtering

## 9. Implementation Strategy

### 9.1 Phase 1: In-Memory Matching
- **IS-1.1**: Goroutine-based connection handling
- **IS-1.2**: Channel-based message passing for matching
- **IS-1.3**: Memory-efficient data structures for waiting connections
- **IS-1.4**: Timeout management with context cancellation

### 9.2 Phase 2: Persistent Storage
- **IS-2.1**: Database abstraction layer for multi-database support
- **IS-2.2**: Streaming write operations for high throughput
- **IS-2.3**: Resume token generation and validation
- **IS-2.4**: Background cleanup and retention management

## 10. Success Criteria

### 10.1 Phase 1 Success Metrics
- Real-time matching latency < 10ms
- Support 10,000+ concurrent streams
- 99.9% successful matching rate
- Handle 1,000+ operations/second

### 10.2 Phase 2 Success Metrics
- Successful replay from any stream position
- Support for streams with 1M+ outputs
- Efficient storage utilization
- Sub-100ms historical query response

## 11. Development Phases

### Phase 1: Real-time Matching (Current Priority)
- [x] Basic in-memory stream matching
- [x] Long polling API endpoints
- [ ] Timeout and error handling
- [ ] Connection management and cleanup
- [ ] Performance optimization
- [ ] Load testing and validation

### Phase 2: Persistence and Replay (Future)
- [ ] Database integration layer
- [ ] sendAndStore API implementation
- [ ] Resume token system
- [ ] Historical data querying
- [ ] Retention policy management
- [ ] Migration from Phase 1

## 12. Constraints and Assumptions

### 12.1 Technical Constraints
- Must handle high concurrency with efficient resource usage
- Should minimize latency for real-time matching
- Must gracefully handle network disconnections
- Should support horizontal scaling

### 12.2 Assumptions
- Clients can handle 424 errors and implement retry logic
- Network connectivity is generally stable for long polling
- StreamIDs are unique and meaningful to applications
- Applications can tolerate delivery timeouts 