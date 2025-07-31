# Async Output Service 🚧WIP🚧
The Async Output Service solves the core problem of connecting asynchronous output generation with real-time client consumption. When applications generate outputs asynchronously (e.g., in background processes), they need a way to deliver these outputs to specific clients who are actively waiting for them in real-time.

The service acts as a matching intermediary between:
- **Applications** generating outputs asynchronously without direct client connections
- **Clients** waiting to receive specific outputs immediately as they become available

## Core Problem Statement

### The Challenge
- Applications run background processes that generate outputs asynchronously
- Clients need to receive these outputs in real-time as they become available
- There's no direct connection between the async application and the waiting client
- Thousands of output instances may be generated concurrently
- Thousands of clients may be waiting for different specific outputs simultaneously

### Key Use Cases
- **AI Agent Progress Updates**: AI agents running background processes (LLM interactions, tool executions) need to stream progress and intermediate results to users
- **Long-running Task Results**: Applications processing lengthy operations need to deliver results as they become available
- **Real-time Notifications**: Background services generating events that specific clients are waiting to receive
- **Durable Output Storage**: Applications need to persist outputs for durability, enabling clients to resume consumption from any point and replay historical outputs when needed

## System Design Overview

The Async Output Service is designed as a distributed system that enables real-time matching between applications generating outputs asynchronously and clients waiting to receive specific outputs. The system uses a gossip-based clustering approach with consistent hashing for horizontal scalability and fault tolerance.

**Inspiration**: This design is similar to [Temporal's matching service architecture](https://github.com/temporalio/temporal/blob/main/docs/architecture/matching-service.md), which provides proven patterns for distributed stream matching at scale.

### Distributed Cluster Architecture
```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│    Node A   │◄──►│    Node B   │◄──►│    Node C   │
│             │    │             │    │             │
│ Gossip+HTTP │    │ Gossip+HTTP │    │ Gossip+HTTP │
│             │    │             │    │             │
│ StreamIDs:  │    │ StreamIDs:  │    │ StreamIDs:  │
│ hash(0-33%) │    │hash(34-66%) │    │hash(67-99%) │
└─────────────┘    └─────────────┘    └─────────────┘
       ▲                   ▲                   ▲
       │                   │                   │
   ┌───▼───┐           ┌───▼───┐           ┌───▼───┐
   │Client │           │Client │           │Client │
   │Send/  │           │Send/  │           │Send/  │
   │Receive│           │Receive│           │Receive│
   └───────┘           └───────┘           └───────┘
```

See more details in [system design doc](./docs/system-design.md)

### Core Components

#### **Membership Management**
- **Gossip Protocol**: Uses HashiCorp's memberlist library for node discovery and failure detection
- **Cluster Formation**: Nodes join cluster via bootstrap node list
- **Event Handling**: Real-time notifications for node join/leave/failure events
- **Membership List**: Each node maintains complete view of cluster topology

#### **Consistent Hashing Ring**
- **Stream Routing**: Maps streamId to specific node using consistent hashing
- **Load Distribution**: Evenly distributes streams across available nodes
- **Fault Tolerance**: Automatic rebalancing when nodes join/leave cluster
- **Deterministic Routing**: Same streamId always routes to same node (when topology stable)

#### **Request Processing Engine**
- **Local Processing**: Handle streams owned by current node
- **Request Forwarding**: Proxy requests to appropriate node based on streamId ownership
- **Sync Matching**: Real-time pairing of concurrent send/receive operations
- **Long Polling**: Timeout-based waiting for asynchronous matching


## Documentations

* [Requirement docs](./REQUIREMENTS.md)
* [API Design docs](./docs/api-design.md)
* [System Design docss](./docs/system-design.md)
* [Repo layout](./docs/repo-layout.md)
* [Design decisoins](./DECISION_LOG.md)


## Development

* Phase1: In-memory output matching 🚧WIP🚧
* Phase2: Support persisting output into database 🚧Not Started🚧

## Supported databases
* [ ] Cassandra
* [ ] MongoDB
* [ ] DynamoDB
* [ ] MySQL
* [ ] PostgreSQL
