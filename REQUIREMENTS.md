# iPubSub Requirements

## Overview

iPubSub is a lightweight, scalable pub-sub service that enables real-time message delivery and optional persistent storage with replay capabilities. The service is designed to handle millions of lightweight topics/streams with dynamic creation and efficient long-polling.

## Problem Statement

Modern applications need a pub-sub service that addresses the limitations of existing solutions:

### Current Pub-Sub Limitations

| **AWS SNS** | **AWS SQS** | **Apache Kafka** | **Redis Pub/Sub** | **Apache Pulsar** |
|-------------|-------------|------------------|-------------------|------------------|
| ❌ Push-based delivery | ❌ Heavy queues | ❌ Heavy topics | ❌ Memory-limited | ❌ Complex operations |
| ❌ Only in-memory | ❌ Limited topic count | ❌ Requires consumer restart | ❌ Not horizontally scalable | ❌ Requires BookKeeper + ZooKeeper |
| | | | | ❌ Per-stream TTL only |

### iPubSub Solution

✅ **Long-poll based** - Efficient network usage vs push-based systems  
✅ **Lightweight streams** - Support millions of topics without heavy provisioning  
✅ **Dynamic creation** - Topics created automatically on first message  
✅ **Dual storage modes** - In-memory + persistent with replay  
✅ **Horizontally scalable** - Distributed hash ring architecture  
✅ **Per-message TTL** - Individual message expiration control  
✅ **Simple operations** - Only requires database for persistence  

## Core Requirements

### Functional Requirements

#### FR1: Dynamic Topic Management
- **Lightweight Topics**: Support millions of streams/topics without explicit provisioning
- **Auto-creation**: Topics created automatically on first message publish
- **No Pre-configuration**: No need to declare topics before use

#### FR2: Dual Delivery Modes
- **Real-time Matching**: Publishers and subscribers matched in real-time via long-polling
- **Persistent Storage**: Optional message persistence with configurable per-message TTL
- **Replay Capability**: Resume consumption from any point using resume tokens

#### FR3: Long-Polling APIs
- **Publisher API**: `POST /api/v1/streams/send` for message publishing (only long poll for in-memoery delivery with blockingSendTimeout enabled)
- **Subscriber API**: `GET /api/v1/streams/receive` with configurable timeout
- **Timeout Handling**: Return 424 (Failed Dependency) when no match within timeout

#### FR4: Flexible Message Format
- **Any JSON Value**: Support object, array, string, number, boolean, null
- **Message Identification**: Unique `messageUuid` per message
- **Timestamp Tracking**: Automatic timestamp assignment

#### FR5: Stream Configuration
- **In-memory Buffer**: Configurable circular buffer size per stream
- **Blocking Modes**: Support circular buffer, blocking queue, and sync match modes
- **Flow Control**: Configurable blocking write timeouts

### Non-Functional Requirements

#### NFR1: Scalability
- **Horizontal Scaling**: Distributed cluster with hash ring routing
- **High Throughput**: Handle thousands of concurrent publishers/subscribers
- **Million Topics**: Support millions of lightweight streams simultaneously

#### NFR2: Availability
- **Fault Tolerance**: Automatic node failure detection and recovery
- **Request Forwarding**: Automatic routing to appropriate node
- **Graceful Degradation**: Continue operation with reduced cluster size

#### NFR3: Performance
- **Low Latency**: Sub-second message delivery for real-time matching
- **Efficient Polling**: Long-polling reduces network overhead vs short polling
- **Memory Efficiency**: Configurable in-memory buffer sizes

#### NFR4: Operational Simplicity
- **Minimal Dependencies**: Only Cassandra required for persistence (optional)
- **Easy Deployment**: Simple cluster configuration
- **Self-Healing**: Automatic cluster membership management

## API Specification

### Send Message (Publisher)
```http
POST /api/v1/streams/send
Content-Type: application/json

{
  "streamId": "user-notifications",
  "messageUuid": "msg-123", 
  "message": {"type": "welcome", "userId": 123},
  "inMemoryStreamSize": 100,
  "blockingSendTimeoutSeconds": 30,
  "writeToDB": false,
  "dbTTLSeconds": 86400
}
```

### Receive Messages (Subscriber)
```http
GET /api/v1/streams/receive?streamId=user-notifications&timeoutSeconds=30
```

**Response:**
```json
{
  "messageUuid": "msg-123",
  "message": {"type": "welcome", "userId": 123},
  "timestamp": "2025-08-03T10:00:00Z",
  "dbResumeToken": "abc123def456"
}
```





## Use Cases

### UC1: Real-time Notifications
**Scenario**: Mobile app sending push notifications to users
- Publisher: Notification service
- Subscriber: Mobile clients via long-polling
- Stream ID: `notifications:{userId}`
- Mode: In-memory real-time delivery

### UC2: Chat Systems
**Scenario**: Multi-user chat application
- Publisher: Chat service
- Subscriber: Connected users
- Stream ID: `chat:{roomId}`
- Mode: In-memory + persistent for message history

### UC3: IoT Data Streams
**Scenario**: Sensor data collection and processing
- Publisher: IoT devices
- Subscriber: Data processing services
- Stream ID: `sensor:{deviceId}`
- Mode: Persistent with TTL for time-series data

### UC4: Event Streaming
**Scenario**: Microservice event distribution
- Publisher: Source microservices
- Subscriber: Dependent services
- Stream ID: `events:{serviceId}`
- Mode: Persistent for guaranteed delivery

