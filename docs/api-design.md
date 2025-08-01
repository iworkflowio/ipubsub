# Async Output Service API Design

## Overview

This document describes the design decisions and rationale behind the Async Output Service REST API. The API provides a stream-based matching system for real-time delivery of asynchronous outputs to waiting clients, with support for both in-memory and persistent storage.

## Core Design Principles

### 1. Stream-Based Matching
- **Decision**: Use `streamId` as the primary identifier for connecting output producers with consumers
- **Rationale**: Enables many-to-many relationships where multiple producers can send to the same stream and multiple consumers can receive from it
- **Implementation**: All operations are organized around stream identifiers rather than individual message IDs

### 2. Dual Storage Strategy
- **Decision**: Support both in-memory streams and persistent database storage within the same send API
- **Rationale**: Provides flexibility for different use cases - real-time ephemeral data vs. durable persistent data
- **Implementation**:
  - In-memory: Fast, low-latency, bounded-size circular buffers
  - Database: Persistent, TTL-based retention, resume token support

### 3. Asymmetric Polling Model  
- **Decision**: Send operations are non-blocking, receive operations use long polling
- **Rationale**: Producers shouldn't be blocked by consumer availability, but consumers benefit from real-time delivery
- **Implementation**:
  - Send API: Immediate return after storing output
  - Receive API: Long polling with configurable timeout

## API Endpoints

### Send Output (`POST /api/v1/streams/send`)

**Core Behavior**:
- **Immediate storage** (default) - stores output and returns immediately
- **Optional blocking** - can wait for stream space if `blockingWriteTimeoutSeconds` is specified
- Supports both in-memory and database storage modes
- Uses `writeToDB` parameter to determine storage strategy

**Request Schema**:
```json
{
  "outputUuid": "123e4567-e89b-12d3-a456-426614174000",
  "streamId": "ai-agent-123", 
  "output": {"message": "Processing step 1 completed", "step": 1},
  "writeToDB": false,
  "inMemoryStreamSize": 1000,
  "blockingWriteTimeoutSeconds": 30,
  "dbTTLSeconds": 3600
}
```

**Storage Mode Selection**:
- `writeToDB: false` (default): Store in bounded in-memory stream
- `writeToDB: true`: Persist to database with TTL-based retention

**In-Memory Stream Behavior**:
- **Circular Buffer Mode** (default): `inMemoryStreamSize` only, overwrites oldest data when full
- **Blocking Queue Mode**: `blockingWriteTimeoutSeconds` specified, waits for space and returns 424 on timeout
- **Sync Match Queue**: `inMemoryStreamSize: 0` + `blockingWriteTimeoutSeconds`, provides zero-loss matching

**Memory Management**:
- `inMemoryStreamSize`: Controls buffer/queue size (default: 100)
- `blockingWriteTimeoutSeconds`: Enables blocking behavior instead of circular overwrite
- Only applies when `writeToDB: false`
- Only effective when stream is empty (initial creation)
- **Implementation Details**: See [In-Memory Storage Systems](system-design.md#46-in-memory-circular-buffer-storage) in the system design document

### Receive Output (`GET /api/v1/streams/receive`)

**Core Behavior**:
- Uses long polling to wait for available output
- Returns HTTP 424 on timeout (no output available)
- Supports both real-time and historical playback modes

**Query Parameters**:
```
streamId: "ai-agent-123" (required)
timeoutSeconds: 60 (optional, default: 30)
readFromDB: true (optional, default: false)  
dbResumeToken: "abc123def456" (optional)
```

**Reading Modes**:
- `readFromDB: false`: Real-time consumption from in-memory streams
- `readFromDB: true`: Historical playback from persistent storage
- `dbResumeToken`: Enables replay from specific position

**Response Schema**:
```json
{
  "outputUuid": "123e4567-e89b-12d3-a456-426614174000",
  "output": {"message": "Processing step 1 completed", "step": 1},
  "timestamp": "2024-01-01T10:00:00Z",
  "dbResumeToken": "def456ghi789"
  }
  ```

## Key Design Decisions

### 1. Unified Send API
- **Previous**: Separate `send` (with matching) and `sendAndStore` APIs
- **Current**: Single `send` API with storage mode parameter
- **Rationale**: Simplifies client code and reduces API surface area
- **Benefit**: Producers can dynamically choose storage strategy per message

### 2. No Blocking on Send
- **Decision**: Send operations never wait for consumer availability
- **Rationale**: Decouples producer performance from consumer readiness
- **Implementation**: Output is always stored (memory or DB) regardless of consumer state
- **Benefit**: High producer throughput and predictable latency

### 3. Object-Only Output Type
- **Decision**: Support only JSON objects for output data
- **Rationale**: Simplifies serialization/deserialization and provides structured data
- **Implementation**: `output` field is always `type: object`
- **Benefit**: Consistent data handling across all operations

### 4. TTL-Based Retention
- **Decision**: Use `dbTTLSeconds` for automatic cleanup of persistent data
- **Rationale**: Prevents unbounded storage growth and provides predictable data lifecycle
- **Implementation**: Database automatically removes expired outputs
- **Default**: 24 hours (86400 seconds)

## Error Handling Strategy

### HTTP Status Code Usage

| Code | Usage | Description |
|------|--------|-------------|
| 200 | Success | Output sent/received successfully |
| 400 | Client Error | Invalid request parameters |
| 424 | Failed Dependency | No output available (receive timeout) |
| 500 | Server Error | Internal server error |

### Timeout Behavior
- **Send API**: Never times out - always succeeds if request is valid
- **Receive API**: Returns 424 after `timeoutSeconds` if no output available
- **Client Strategy**: Clients should retry receive operations on 424 responses

## In-Memory Stream Modes

### 1. Circular Buffer Mode (Default)
**Configuration**: Only `inMemoryStreamSize` specified
**Behavior**: Overwrites oldest data when buffer is full
**Use Case**: High-throughput scenarios where recent data is most important

```bash
POST /api/v1/streams/send
{
  "streamId": "metrics-stream",
  "outputUuid": "metric-001",
  "output": {"cpu": 75.2, "timestamp": "2024-01-01T10:00:00Z"},
  "inMemoryStreamSize": 1000,
  "writeToDB": false
}
# Returns 200 immediately, overwrites oldest data if buffer full
```

### 2. Blocking Queue Mode  
**Configuration**: Both `inMemoryStreamSize` and `blockingWriteTimeoutSeconds` specified
**Behavior**: Waits for space when buffer is full, returns 424 on timeout
**Use Case**: Scenarios requiring backpressure to prevent data loss

```bash
POST /api/v1/streams/send
{
  "streamId": "important-events",
  "outputUuid": "event-001", 
  "output": {"event": "user_signup", "userId": "123"},
  "inMemoryStreamSize": 100,
  "blockingWriteTimeoutSeconds": 30,
  "writeToDB": false
}
# Waits up to 30 seconds for buffer space, returns 424 if still full
```

### 3. Sync Match Queue Mode
**Configuration**: `inMemoryStreamSize: 0` + `blockingWriteTimeoutSeconds`
**Behavior**: Zero-capacity queue, requires immediate consumer availability
**Use Case**: Real-time sync matching with guaranteed no data loss

```bash
POST /api/v1/streams/send
{
  "streamId": "realtime-chat",
  "outputUuid": "msg-001",
  "output": {"message": "Hello!", "sender": "user123"},
  "inMemoryStreamSize": 0,
  "blockingWriteTimeoutSeconds": 10,
  "writeToDB": false
}
# Only succeeds if consumer is actively waiting, returns 424 otherwise
```

## Use Case Examples

### Real-Time AI Agent Progress
```bash
# Producer sends progress updates
POST /api/v1/streams/send
{
  "outputUuid": "step-1-uuid",
  "streamId": "ai-agent-session-123",
  "output": {"step": 1, "status": "processing", "progress": 25},
  "writeToDB": false,
  "inMemoryStreamSize": 50
}

# Consumer receives real-time updates
GET /api/v1/streams/receive?streamId=ai-agent-session-123&timeoutSeconds=30
```

### Durable Task Results
```bash
# Producer stores important results
POST /api/v1/streams/send  
{
  "outputUuid": "result-uuid",
  "streamId": "batch-job-456",
  "output": {"status": "completed", "resultUrl": "s3://..."},
  "writeToDB": true,
  "dbTTLSeconds": 86400
}

# Consumer can replay from any position
GET /api/v1/streams/receive?streamId=batch-job-456&readFromDB=true&dbResumeToken=abc123
```

### High-Throughput Event Streaming
```bash
# Multiple producers send to same stream
POST /api/v1/streams/send
{
  "outputUuid": "event-1-uuid", 
  "streamId": "metrics-stream",
  "output": {"metric": "cpu_usage", "value": 75.2, "timestamp": "2024-01-01T10:00:00Z"},
  "writeToDB": false,
  "inMemoryStreamSize": 10000
}

# Multiple consumers receive from same stream
GET /api/v1/streams/receive?streamId=metrics-stream&timeoutSeconds=5
```

---

*This design document reflects the current API specification and will be updated as the API evolves.* 