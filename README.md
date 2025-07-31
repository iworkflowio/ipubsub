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
