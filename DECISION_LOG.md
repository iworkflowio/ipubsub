# iPubSub Design Decisions

## Service Rename: AsyncOutputService → iPubSub (2025-08-03)

**Decision**: Rename the service from "Async Output Service" to "iPubSub" and reposition it as a pub-sub service.

**Context**: 
- The original "async output" terminology was confusing and didn't clearly communicate the pub-sub nature
- Need to clearly differentiate iPubSub from existing pub-sub solutions
- The service is fundamentally a publish-subscribe system with unique characteristics

**Key Differentiators from Other Pub-Sub Services**:

| Feature | iPubSub | AWS SNS | AWS SQS | Apache Kafka | Redis Pub/Sub | Apache Pulsar |
|---------|---------|---------|---------|--------------|---------------|---------------|
| **Delivery Model** | Long-poll | Push | Pull | Pull | Push | Pull |
| **Topic Creation** | Dynamic/Auto | Manual | Manual | Manual | Dynamic | Manual |
| **Topic Weight** | Lightweight | Medium | Heavy | Heavy | Lightweight | Heavy |
| **Scalability** | Horizontal | Vertical | Limited | Horizontal | Memory-limited | Horizontal |
| **TTL Granularity** | Per-message | N/A | Per-queue | Per-topic | N/A | Per-stream |
| **Dependencies** | Cassandra only | AWS | AWS | ZooKeeper | Redis | BookKeeper + ZooKeeper |
| **Topic Limit** | Millions | Thousands | Limited | Limited | Unlimited | Thousands |

**Rationale**:
- **Lightweight Topics**: Support millions of streams without heavy provisioning overhead
- **Dynamic Creation**: Topics created automatically on first message (no pre-configuration)
- **Long-polling Efficiency**: More network-efficient than push-based systems
- **Per-message TTL**: Fine-grained expiration control vs stream-level TTL
- **Operational Simplicity**: Only requires Cassandra for persistence (optional)

---

## In-Memory Stream Modes: Added Blocking Queue and Sync Match Capabilities (2025-08-01)

**Decision**: Introduce `blockingWriteTimeoutSeconds` parameter to enable different in-memory stream behaviors.

**Context**: The original circular buffer mode overwrites old messages, but some use cases need guaranteed delivery or immediate synchronous matching.

**Three Stream Modes**:
1. **Circular Buffer** (`blockingWriteTimeoutSeconds` = 0): Overwrites oldest messages when full
2. **Blocking Queue** (`blockingWriteTimeoutSeconds` > 0): Blocks publishers when full, returns 424 on timeout
3. **Sync Match Queue** (`inMemoryStreamSize` = 0, `blockingWriteTimeoutSeconds` > 0): Zero-capacity queue for immediate matching

**Implementation**: 
- Modified `InMemoryStreamImpl.Send()` to check `blockingWriteTimeoutSeconds`
- Used Go channels with different behaviors based on parameters
- Added `time.After()` for timeout handling in blocking mode

**Rationale**: Provides flexibility for different use cases while maintaining backward compatibility with circular buffer as default.

---

## API Unification: Merged send and sendAndStore into Single Send Endpoint (2025-08-01)

**Decision**: Consolidate `send` and `sendAndStore` APIs into a single `/api/v1/streams/send` endpoint with optional persistence.

**Context**: Having separate endpoints for in-memory vs persistent messaging created confusion and API complexity.

**Changes**:
- Single `send` endpoint with `writeToDB` boolean parameter
- Removed send-side waiting for matching by default
- Send operation writes to memory/DB based on parameters, doesn't wait for subscribers
- Subscribers still use long-polling via `receive` endpoint

**Benefits**:
- Simpler API surface with single publish endpoint
- Dual storage strategy (memory + DB) in single call
- Cleaner separation: publishers send, subscribers poll
- Backward compatible through parameter defaults

**API Design**:
```json
POST /api/v1/streams/send
{
  "streamId": "topic-name",
  "output": "message-data", 
  "writeToDB": false,
  "inMemoryStreamSize": 100
}
```

---

## System Design: Distributed Hash Ring with Gossip Protocol (2025-07-31)

**Decision**: Implement distributed architecture using consistent hashing ring and gossip-based cluster membership.

**Context**: Need horizontal scalability and fault tolerance for high-throughput pub-sub workloads.

**Architecture Components**:
1. **Gossip Protocol**: HashiCorp's memberlist for node discovery and failure detection
2. **Consistent Hashing**: Stream routing with virtual nodes for load balancing  
3. **Request Forwarding**: Automatic proxying to stream owner nodes
4. **Membership Management**: Real-time cluster topology tracking

**Stream Routing**: `hash(streamId) % ring_size` determines owning node
**Load Balancing**: Virtual nodes provide even distribution
**Fault Tolerance**: Ring rebalances automatically on node join/leave

**Benefits**:
- Linear scalability with node count
- Automatic failover and recovery
- Deterministic stream routing
- No single point of failure

**Similar to**: Temporal's matching service architecture (proven at scale)

---

## API Design: Object-Only Output with Simplified Schemas (2025-07-31)

**Decision**: Support only `object` type for output field and remove verbose response schemas.

**Context**: Initial API design supported multiple output types (string, binary, object) with complex response schemas.

**Simplifications**:
- **Output Type**: Only JSON objects (`map[string]interface{}` in Go)
- **Response Schemas**: Removed `SendResponse` and `ErrorResponse` 
- **Metadata**: Replaced complex metadata with simple `ttl` field
- **Position**: Removed `position` field (resumeToken sufficient for replay)

**Benefits**:
- Simpler client integration
- Reduced API surface area  
- JSON-native messaging
- Cleaner response handling

**Note**: Later updated to support `interface{}` (any JSON value) for maximum flexibility.

---

## Initial API Design: Stream-Based Matching with Long Polling (2025-07-31)

**Decision**: Design pub-sub API around streamID-based topic routing with long-polling for efficiency.

**Context**: Need to connect message publishers with subscribers in real-time without direct connections.

**Core Concepts**:
- **StreamID**: Topic identifier for message routing
- **Long Polling**: Efficient network usage vs short polling
- **Dual Storage**: In-memory real-time + optional persistence
- **Resume Tokens**: Replay capability for persistent messages

**API Endpoints**:
- `POST /api/v1/streams/send` - Publish messages
- `GET /api/v1/streams/receive` - Subscribe with long-polling

**Error Handling**: HTTP 424 (Failed Dependency) for timeout scenarios

**Rationale**: Long-polling provides better network efficiency than WebSocket for sporadic messaging patterns while maintaining simplicity. 