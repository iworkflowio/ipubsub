# iPubSub API Design

## Overview

The iPubSub API is designed as a lightweight, scalable pub-sub service that enables real-time message delivery and optional persistent storage. The API uses simple HTTP endpoints with long-polling to efficiently connect publishers and subscribers through lightweight topics/streams.

## Core Concepts

### 1. Streams/Topics
- **Stream ID**: Unique identifier for a topic/stream (e.g., `user-notifications`, `chat-room-123`)
- **Lightweight**: Topics are created automatically on first message without provisioning
- **Dynamic**: No pre-configuration required, topics exist as long as they have messages or active subscribers

### 2. Publishers and Subscribers
- **Publishers**: Applications that send messages to streams using `POST /api/v1/streams/send`
- **Subscribers**: Applications that receive messages from streams using `GET /api/v1/streams/receive`
- **Asymmetric**: Publishers can send immediately or blockingSend(for in-memoery matching), subscribers use long-polling for efficiency

### 3. Dual Storage Strategy
- **In-Memory**: Real-time message delivery using configurable buffers
- **Persistent**: Optional database storage with per-message TTL for replay
- **Dynamic**: Publishers can choose storage mode per message using `writeToDB` parameter

## API Endpoints

### Send Message (Publisher)

```http
POST /api/v1/streams/send
```

Publishers use this endpoint to send messages to a specific streamId (topic).

**Request Body:**
```json
{
  "streamId": "user-notifications",
  "messageUuid": "123e4567-e89b-12d3-a456-426614174000", 
  "message": {"type": "welcome", "userId": 123},
  "inMemoryStreamSize": 100,
  "blockingSendTimeoutSeconds": 30,
  "writeToDB": false,
  "dbTTLSeconds": 86400
}
```

**Parameters:**
- `streamId` (required): Unique identifier for the stream/topic
- `messageUuid` (required): Unique identifier for this specific message
- `message` (required): Message data (any JSON value - object, array, string, number, boolean, null)
- `inMemoryStreamSize` (optional): Size of in-memory buffer for the stream (default: 100)
- `blockingSendTimeoutSeconds` (optional): Enables blocking write behavior with timeout
- `writeToDB` (optional): Whether to persist message to database (default: false)
- `dbTTLSeconds` (optional): Message TTL in seconds when persisted (default: 86400)

**Responses:**
- `200 OK`: Message published successfully
- `424 Failed Dependency`: No space available (for blocking writes with timeout)
- `400 Bad Request`: Invalid parameters
- `500 Internal Server Error`: Server error

### Receive Messages (Subscriber)

```http
GET /api/v1/streams/receive?streamId=user-notifications&timeoutSeconds=30
```

Subscribers use this endpoint to receive messages for a specific streamId (topic) using long-polling.

**Query Parameters:**
- `streamId` (required): Stream/topic to subscribe to
- `timeoutSeconds` (optional): Maximum wait time for messages (default: 30)
- `readFromDB` (optional): Read from persistent storage (Phase 2, default: false)
- `dbResumeToken` (optional): Resume from specific position (Phase 2)

**Response (200 OK):**
```json
{
  "messageUuid": "123e4567-e89b-12d3-a456-426614174000",
  "message": {"type": "welcome", "userId": 123},
  "timestamp": "2025-08-03T10:00:00Z",
  "dbResumeToken": "abc123def456"
}
```

**Other Responses:**
- `424 Failed Dependency`: No messages available within timeout
- `400 Bad Request`: Invalid parameters  
- `500 Internal Server Error`: Server error

## In-Memory Stream Modes

iPubSub supports three different in-memory stream behaviors controlled by the `inMemoryStreamSize` and `blockingSendTimeoutSeconds` parameters:

### 1. Circular Buffer Mode (Default)
- **Configuration**: `inMemoryStreamSize > 0`, `blockingSendTimeoutSeconds` not specified
- **Behavior**: Overwrites oldest messages when buffer is full
- **Use Case**: High-throughput scenarios where recent messages are most important
- **Example**: Real-time metrics, live updates

### 2. Blocking Queue Mode  
- **Configuration**: `inMemoryStreamSize > 0`, `blockingSendTimeoutSeconds > 0`
- **Behavior**: Blocks publisher when buffer is full, returns 424 after timeout
- **Use Case**: Scenarios requiring backpressure and guaranteed delivery
- **Example**: Critical notifications, payment processing events

### 3. Sync Match Queue Mode
- **Configuration**: `inMemoryStreamSize = 0`, `blockingSendTimeoutSeconds > 0`  
- **Behavior**: Zero-capacity queue requiring immediate subscriber availability
- **Use Case**: Real-time synchronous communication without buffering
- **Example**: Chat messages, live collaboration

## Message Processing Flow

### Publisher Flow
1. **Message Creation**: Publisher creates message with unique `messageUuid`
2. **Storage Decision**: Choose in-memory, persistent, or both via `writeToDB`
3. **Stream Configuration**: Set buffer size and blocking behavior
4. **Send Request**: POST to `/api/v1/streams/send`
5. **Immediate Return**: Receive 200 OK confirmation (no waiting for subscribers)

### Subscriber Flow  
1. **Long-Poll Request**: GET `/api/v1/streams/receive` with streamId and timeout
2. **Wait for Messages**: Service holds connection until message arrives or timeout
3. **Message Delivery**: Receive message with metadata when available
4. **Resume Token**: Use token for replay (Phase 2) or continue polling

### Distributed Routing
- **Hash Ring**: StreamId is hashed to determine owning node
- **Request Forwarding**: Non-owner nodes forward requests to owner
- **Local Processing**: Owner node handles in-memory matching and storage

## Error Handling

### HTTP Status Codes
- **200 OK**: Successful operation
- **424 Failed Dependency**: Timeout scenarios (no subscribers, buffer full)
- **400 Bad Request**: Invalid parameters, malformed JSON
- **500 Internal Server Error**: System errors, database failures

### Timeout Scenarios
- **Subscriber Timeout**: No messages available within `timeoutSeconds`  
- **Publisher Timeout**: Buffer full in blocking write mode
- **Network Timeout**: Standard HTTP client/server timeouts

### Error Recovery
- **Retry Logic**: Clients should implement exponential backoff for 424 errors
- **Circuit Breaker**: Publishers can fall back to async processing on repeated failures
- **Health Checks**: Monitor endpoint availability and cluster health

## Implementation Details

### Message Identification
- **messageUuid**: UUID format required for message deduplication
- **Timestamp**: Automatically assigned by service on message receipt
- **Stream Affinity**: Same streamId always routes to same node (when cluster stable)

### Performance Characteristics
- **Latency**: Sub-100ms for in-memory delivery within same node
- **Throughput**: >10K messages/second per node
- **Scalability**: Linear scaling with node count via hash ring
- **Memory**: Configurable per-stream buffer sizes prevent memory issues


For detailed implementation specifics on in-memory circular buffers, see the [circular buffer section in system design](./system-design.md#46-in-memory-circular-buffer-storage). 