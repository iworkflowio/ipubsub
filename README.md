# iPubSub

A lightweight, scalable pub-sub service that supports both real-time message delivery and persistent storage with replay capabilities.

## Problem Statement

Modern applications need a pub-sub service that can:

1. **Handle millions of lightweight topics/streams** - Create topics dynamically on first message without explicit provisioning
2. **Support both real-time and persistent messaging** - Messages can be delivered in real-time to active subscribers and/or stored for later replay
3. **Use efficient long-polling** - Reduce network overhead compared to push-based systems
4. **Scale horizontally** - Handle high throughput with distributed architecture
5. **Simple operations** - Easy deployment without complex dependencies

## How iPubSub Differs from Existing Solutions

| Service | iPubSub | Difference |
|---------|---------|------------|
| **AWS SNS** | ✅ Long-poll based<br/>✅ In-memory + persistent modes | SNS: Push-based delivery, only in-memory |
| **AWS SQS** | ✅ Lightweight streams<br/>✅ Dynamic creation | SQS: Heavy queues, cannot support millions of topics |
| **Apache Kafka** | ✅ Lightweight topics<br/>✅ No restart required | Kafka: Heavy topics, requires consumer restart |
| **Redis Pub/Sub** | ✅ Horizontally scalable<br/>✅ Not memory-limited | Redis: Limited by memory size, not horizontally scalable |
| **Apache Pulsar** | ✅ Per-message TTL<br/>✅ Simple operations | Pulsar: Per-stream TTL, requires BookKeeper + ZooKeeper |

**Key Advantage**: iPubSub only requires database for persistence mode(e.g. Cassandra)

See more details in [requirements](./REQUIREMENTS.md)

## Core Concepts

- **Stream/Topic**: Lightweight message channel identified by `streamId`. Created automatically on first message.
- **Real-time Matching**: Senders(aka publishers) and Receivers (aka subscribers) are matched in real-time using long-polling
- **Message Persistence**: Optional storage with replay capability using resume tokens
- **Per-message TTL**: Individual message expiration (not stream-level)

## API Overview

**Send Message:**
```http
POST /api/v1/streams/send
{
  "streamId": "user-notifications",
  "messageUuid": "msg-123",
  "message": {"type": "welcome", "userId": 123},
  "writeToDB": false,
  "inMemoryStreamSize": 100
}
```

**Receive Messages (Long-poll):**
```http
GET /api/v1/streams/receive?streamId=user-notifications&timeoutSeconds=30
```

## Supported Databases

- **Cassandra** (recommended for production)
- **MongoDB**
- **PostgreSQL**
- **MySQL**

## Quick Start

1. **Single Node (Development)**:
   ```bash
   go run ./cmd --config config/single-node.yaml
   ```

2. **Multi-Node Cluster**:
   ```bash
   # Node 1
   go run ./cmd --config config/multi-node-1.yaml
   
   # Node 2  
   go run ./cmd --config config/multi-node-2.yaml
   
   # Node 3
   go run ./cmd --config config/multi-node-3.yaml
   ```

## Architecture

iPubSub uses a distributed hash ring to route streams to specific nodes, ensuring scalability and fault tolerance. Each node can handle both publishing and subscribing, with automatic request forwarding to the owning node.

See more details in [design doc](./docs/system-design.md)

## Use Cases

- **Real-time notifications** - User alerts, system events
- **Event streaming** - Application event distribution  
- **Chat systems** - Message delivery with optional persistence
- **IoT data flows** - Sensor data routing and storage
- **Microservice communication** - Async service-to-service messaging
