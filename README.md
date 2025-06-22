# **Go CQRS & Event Sourcing Framework**

This repository provides a lightweight yet robust Go framework for building event-driven, scalable, and resilient services using Command Query Responsibility Segregation (CQRS) and Event Sourcing. It comes with built-in support for common patterns required in distributed systems, such as the Transactional Outbox, Idempotent Consumers, Snapshotting, and configurable retry mechanisms.

## **Core Concepts & Problems Solved**

Modern distributed systems face several challenges: ensuring data consistency across services, handling transient failures gracefully, and preventing duplicate processing. This framework directly addresses these issues by providing production-ready implementations of key patterns:

1.  **CQRS**: Separates the write-side (Commands) from the read-side (Queries), allowing for independent scaling and optimization of each.
2.  **Event Sourcing**: Persists the full history of changes to an application's state as a sequence of events. Instead of storing the current state of an entity, we store every event that ever happened to it. This provides a robust audit log, makes it possible to debug past states, and allows for building new projections of data retroactively.
3.  **Snapshotting**: An optimization for Event Sourcing. For aggregates with very long event histories, replaying every event can be time-consuming. This framework supports periodically saving a full snapshot of an aggregate's state, significantly speeding up load times.
4.  **Transactional Outbox Pattern**: Guarantees "at-least-once" event delivery to read models and other services. It ensures that events are published if and only if the business transaction that created them is successfully committed. This prevents lost events, a common source of data inconsistency.
5.  **Idempotent Consumers**: Prevents the same event from being processed more than once. When combined with the Outbox pattern, this achieves "exactly-once" processing semantics, making your system resilient to message redeliveries.
6.  **Resiliency & Retries**: Provides a configurable exponential backoff strategy for handling transient failures when processing events, preventing temporary issues from causing permanent failures.

## **Framework Components**

The framework is organized into several core packages, each with a distinct responsibility.

| Package | Description |
| :---- | :---- |
| **event** | Defines the core `Event` interface, the `EventStore` interface for persistence, and base event structs. |
| **es** | Contains the core logic for event-sourced aggregates, including a base `Aggregate` interface, repository, and snapshotting components. |
| **outbox** | Contains the `Relay` worker implementation. The Relay polls the database outbox table, retrieves unpublished events, and sends them to a message broker. |
| **handler** | Provides the `IdempotentEventHandler`, a decorator that adds idempotency, transactional guarantees, and retry logic to any event handler. |
| **messagebus** | Defines the `Broker` interface, abstracting away the specific message broker implementation (e.g., NATS, Kafka). |
| **testutil** | Contains utilities for integration testing, including helpers to spin up ephemeral database instances using Testcontainers. |

## **Project Structure**

The repository is structured to separate the reusable framework from a sample implementation.

```
.
├── event/                \# Core framework: Event definitions
├── handler/              \# Core framework: Idempotent handler wrapper
├── messagebus/           \# Core framework: Broker interface
├── outbox/               \# Core framework: Outbox relay worker
├── testutil/             \# Core framework: Testing utilities
│
├── sample/               \# A complete application demonstrating the framework
│   ├── app/              \# Application layer (command/event handlers)
│   ├── domain/           \# Domain logic and aggregates (e.g., Product)
│   ├── infra/            \# Infrastructure implementations (Postgres, NATS)
│   └── main.go           \# Application entry point
│
├── Dockerfile
├── docker-compose.yml
└── README.md
```
