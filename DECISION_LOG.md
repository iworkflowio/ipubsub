# Async Output Service - Decision Log

This document tracks all major architectural and design decisions made during the development of the async output service.

## Decision Entry Format
Each decision should include:
- **Date**: When the decision was made
- **Context**: Problem being solved or situation requiring a decision
- **Decision**: What was decided
- **Rationale**: Why this decision was made
- **Alternatives**: Other options that were considered
- **Impact**: Expected consequences or implications

**Ordering**: from latest to olderst. 

---

## Decisions

### [Date: 2025-08-01] API Unification: Merged send and sendAndStore into Single Send Endpoint

- **Context**: The original API design included separate `send` and `sendAndStore` endpoints with different behaviors: `send` used long polling to wait for matching clients (returning 424 on timeout), while `sendAndStore` immediately persisted outputs without waiting. This created confusion about when to use which endpoint and forced clients to make upfront decisions about storage strategy. Additionally, the long polling behavior on the send side coupled producer performance to consumer availability, creating potential bottlenecks.

- **Decision**: Consolidate both APIs into a single unified `POST /api/v1/streams/send` endpoint with the following changes:
  - **No Waiting/Matching**: Send operations never wait for consumers - they immediately store output and return 200
  - **Dual Storage Strategy**: Single API supports both in-memory and persistent storage via `writeToDB` parameter
  - **Storage Mode Selection**: 
    - `writeToDB: false` (default): Store in circular buffer with `inMemoryStreamSize` parameter
    - `writeToDB: true`: Persist to database with `dbTTLSeconds` retention policy
  - **Asymmetric Polling**: Only receive operations use long polling with 424 timeout responses
  - **Parameter Consolidation**: Merged `ttl` field from sendAndStore into `dbTTLSeconds` parameter

- **Rationale**: 
  - **Producer Performance Decoupling**: Removing send-side waiting eliminates producer blocking, enabling consistent high throughput regardless of consumer state
  - **Simplified Client Logic**: Single endpoint eliminates the need for clients to choose between different APIs based on persistence requirements
  - **Dynamic Storage Strategy**: Clients can decide storage mode per message rather than per stream, providing greater flexibility
  - **Reduced API Surface**: Fewer endpoints simplify documentation, testing, SDK development, and client integration
  - **Fire-and-Forget Semantics**: Send operations become predictable with guaranteed success (given valid parameters), improving system reliability
  - **Asymmetric Optimization**: Different polling behaviors optimize for different performance characteristics (producer throughput vs consumer latency)

- **Alternatives**: 
  - **Keep Separate APIs**: Rejected due to increased complexity and forced upfront storage decisions
  - **Send-side Long Polling**: Rejected due to producer performance coupling and timeout complexity
  - **Always Persist**: Rejected due to unnecessary storage overhead for ephemeral use cases
  - **Always In-Memory**: Rejected due to lack of durability guarantees for important outputs
  - **Queue-based Approach**: Rejected due to added infrastructure complexity and ordering guarantees

- **Impact**: 
  - **API Simplification**: Reduced from 3 endpoints to 2 endpoints with clearer responsibilities
  - **Producer Performance**: Guaranteed O(1) send latency regardless of consumer state or cluster load
  - **Storage Flexibility**: Dynamic per-message storage decisions enable mixed workloads on same streams
  - **Implementation Complexity**: Simplified matching logic since only receive side manages timeouts and polling
  - **Backward Compatibility**: Breaking change requiring client updates, but with clear migration path
  - **Operational Benefits**: Simplified monitoring, logging, and debugging with unified send path
  - **Memory Management**: Explicit circular buffer configuration provides predictable memory usage for in-memory streams

### [Date: 2025-07-31] Distributed System Architecture with Gossip Protocol and Consistent Hashing

- **Context**: The async output service needs to scale horizontally to handle thousands of concurrent streams and millions of requests. A single-node architecture cannot meet the performance and availability requirements. The system must route send and receive requests for the same streamId to the same node for efficient real-time matching, while providing fault tolerance and automatic load balancing across multiple nodes.

- **Decision**: Implement a distributed cluster architecture using HashiCorp's memberlist library for gossip-based membership management and consistent hashing for stream routing. Key components include:
  - **Gossip Protocol**: Memberlist library for node discovery, failure detection, and cluster membership
  - **Consistent Hashing Ring**: Virtual nodes (150-200 per physical node) for even load distribution
  - **Request Forwarding**: HTTP-based proxy between nodes based on stream ownership
  - **Sync Matching Engine**: In-memory FIFO matching within individual nodes
  - **Rebalance Threshold**: 10% deviation tolerance for load balancing decisions

- **Rationale**: 
  - **Gossip Protocol Choice**: HashiCorp's memberlist provides proven, production-ready gossip implementation with built-in failure detection, encryption support, and minimal configuration overhead
  - **Consistent Hashing Benefits**: Ensures same streamId always routes to same node (when topology stable), enables automatic load redistribution during node changes, and minimizes key movement during cluster topology changes
  - **Virtual Nodes Strategy**: 150-200 virtual nodes per physical node provides excellent load distribution while minimizing disruption during node joins/leaves (compared to single position per node)
  - **Request Forwarding Design**: HTTP-based forwarding is simple, debuggable, and leverages existing infrastructure; allows any node to accept requests and route appropriately
  - **10% Rebalance Threshold**: Balances load evenness (prevents significant hot spots) with stability (avoids excessive cluster churn from minor imbalances)
  - **Local Matching Optimization**: Processing within individual nodes eliminates cross-node coordination overhead for real-time matching

- **Alternatives**: 
  - **External Coordination**: Rejected due to added complexity and single point of failure (Zookeeper, Consul, etcd)
  - **Database-based Routing**: Rejected due to latency overhead and scalability bottlenecks
  - **Message Queue Distribution**: Rejected due to complexity and inability to guarantee real-time matching
  - **WebSocket Connections**: Rejected due to connection management complexity and proxy/firewall issues
  - **RAFT Consensus**: Rejected as overkill for eventual consistency requirements
  - **Simple Round-robin**: Rejected as it cannot guarantee stream affinity for matching
  - **Single Hash Position per Node**: Rejected due to poor load distribution and large key movements

- **Impact**: 
  - **Horizontal Scalability**: System can scale from 1 to 50+ nodes with linear performance improvement
  - **Fault Tolerance**: Automatic failure detection and traffic redistribution when nodes fail
  - **Real-time Performance**: Sub-10ms matching latency for local operations, +5-15ms overhead for forwarded requests
  - **Operational Simplicity**: Minimal configuration required, automatic cluster formation and rebalancing
  - **Development Complexity**: Added distributed systems complexity but using proven libraries minimizes risk
  - **Debugging Capability**: HTTP-based forwarding enables easy request tracing and debugging
  - **Future Scalability**: Architecture supports Phase 2 persistence and Phase 3 stream partitioning extensions

### [Date: 2025-07-31] Async Output Service API Design

- **Context**: Need to design a REST API for the async output service that enables real-time matching between applications generating outputs asynchronously and clients waiting to receive specific outputs. The service acts as a matching intermediary using stream-based identification. Two phases are planned: Phase 1 (in-memory real-time matching) and Phase 2 (persistent storage with replay capability). Key requirements include handling thousands of concurrent streams, sub-10ms matching latency, long polling support, and simple client integration.

- **Decision**: Implement a simplified REST API with three core endpoints:
  - `POST /streams/send` - Real-time output delivery with streamId and outputUuid in request body
  - `GET /streams/receive` - Client consumption with streamId as query parameter, optional resumeToken
  - `POST /streams/sendAndStore` - Persistent storage with TTL-based retention
  
  Key design choices:
  - **outputUuid**: Required UUID field in all operations for unique output identification
  - **JSON-only output**: Support only JSON objects to eliminate format complexity
  - **Simple responses**: Use HTTP status codes (200/201/424/400/500) without complex response bodies
  - **Long polling**: Configurable timeouts with 424 "Failed Dependency" for timeout scenarios
  - **TTL retention**: Simple time-based retention (e.g., "24h") for stored outputs
  - **Resume tokens**: Abstract position tracking for Phase 2 replay functionality

- **Rationale**: 
  - **Stream-based Matching**: streamId provides clear semantic connection between senders and receivers, enabling efficient routing and horizontal scaling
  - **Parameter Placement Consistency**: Request body for send operations allows complex payloads; query parameters for receive operations enable simple URL-based access and caching
  - **outputUuid for Deduplication**: Unique identification prevents duplicate processing and enables tracking across distributed systems
  - **JSON-only Simplification**: Eliminates format complexity (text/binary) while providing sufficient structured data capability for all use cases
  - **Simplified Response Design**: HTTP status codes provide sufficient success/failure information; complex response schemas add unnecessary serialization overhead
  - **424 Error Semantics**: "Failed Dependency" clearly indicates timeout scenarios where matching fails, distinguishing from client errors (400) and server errors (500)
  - **TTL over Metadata**: Simple time-based retention is easier to implement and understand than complex metadata with tags/priority
  - **Resume Token Abstraction**: Provides flexibility for different storage implementations while hiding internal position/offset details

- **Alternatives**: 
  - **Path parameters for streamId**: Rejected due to inconsistency across operations and URL encoding complexity
  - **Multiple output formats** (text/json/binary): Rejected due to unnecessary complexity and performance overhead
  - **Complex response schemas**: Rejected due to serialization overhead and bandwidth usage
  - **Separate APIs for phases**: Rejected in favor of unified approach with optional parameters
  - **Position-based tracking**: Rejected in favor of resume tokens which provide better abstraction
  - **Metadata-based retention**: Rejected in favor of simple TTL approach
  - **WebSocket connections**: Rejected due to complexity and firewall/proxy issues with long polling being sufficient

- **Impact**: 
  - **Development Simplicity**: Uniform request patterns and simple responses reduce implementation complexity
  - **Performance Optimization**: Minimal response overhead enables sub-10ms matching latency targets
  - **Horizontal Scalability**: Stream-based design supports partitioning across multiple service instances
  - **Client Integration**: Simple HTTP semantics reduce client SDK complexity and improve adoption
  - **Operational Benefits**: Unified API design reduces documentation, testing, and monitoring complexity
  - **Future Flexibility**: Resume token design allows evolution of storage backends without API changes
  - **Debugging Capability**: outputUuid enables end-to-end tracing of outputs through the system

---

*This log should be updated whenever significant decisions are made during the project development.* 