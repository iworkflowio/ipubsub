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

### [Date: 2025-01-20] Async Output Service API Design

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