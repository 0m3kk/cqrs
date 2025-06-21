# **Production-Ready Go CQRS Framework**

This repository provides a lightweight yet robust Go framework for building event-driven, scalable, and resilient services using the Command Query Responsibility Segregation (CQRS) pattern. It comes with built-in support for common patterns required in distributed systems, such as the Transactional Outbox, Idempotent Consumers, and configurable retry mechanisms.

## **Core Concepts & Problems Solved**

Modern distributed systems face several challenges: ensuring data consistency across services, handling transient failures gracefully, and preventing duplicate processing. This framework directly addresses these issues by providing production-ready implementations of key patterns:

1. **CQRS**: Separates the write-side (Commands) from the read-side (Queries), allowing for independent scaling and optimization of each.
2. **Transactional Outbox Pattern**: Guarantees "at-least-once" event delivery. It ensures that events are published if and only if the business transaction that created them is successfully committed. This prevents lost events, a common source of data inconsistency.
3. **Idempotent Consumers**: Prevents the same event from being processed more than once. When combined with the Outbox pattern, this achieves "exactly-once" processing semantics, making your system resilient to message redeliveries.
4. **Resiliency & Retries**: Provides a configurable exponential backoff strategy for handling transient failures when processing events, preventing temporary issues from causing permanent failures.

## **Framework Components**

The framework is organized into several core packages, each with a distinct responsibility.

| Package | Description |
| :---- | :---- |
| **event** | Defines the core Event interface and base structs. All domain events in your application should conform to this interface. |
| **outbox** | Contains the Relay worker implementation. The Relay polls the database outbox table, retrieves unpublished events, and sends them to a message broker. It is designed to run concurrently with multiple instances for high throughput. |
| **handler** | Provides the IdempotentEventHandler, a decorator (wrapper) that adds idempotency, transactional guarantees, and retry logic to any event handler. |
| **messagebus** | Defines the Broker interface, abstracting away the specific message broker implementation (e.g., NATS, Kafka). |
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

## **Getting Started: Running the Sample Application**

The sample/ directory contains a fully functional application that demonstrates how to use the framework. It simulates creating a "Product" and publishing a ProductCreated event.

**Prerequisites:**

* Docker
* Docker Compose

**To run the sample:**

1. Navigate to the sample/ directory.
2. Run the following command:
   docker-compose up \--build

This command will:

* Build the Go application binary inside a Docker container.
* Start three services: a PostgreSQL database, a NATS message broker, and the application itself.
* The application will automatically connect to the database and broker.
* After a few seconds, it will simulate a CreateProductCommand.
* You can observe the logs to see the entire flow:
  1. The command handler creates an event and saves it to the outbox table.
  2. An outbox.Relay worker picks up the event.
  3. The Relay publishes the event to NATS.
  4. The ProductViewProjection subscriber receives the event.
  5. The IdempotentEventHandler wrapper ensures it's processed exactly once.

## **Usage in Your Own Project**

To use this framework in a new project:

1. **Copy the Core Packages**: Copy the event, handler, messagebus, outbox, and testutil directories into your new project.
2. **Define Your Domain**: Create your own aggregates and domain events, ensuring your events implement the event.Event interface.
3. **Implement Infrastructure**:
   * Create persistence layers for your repositories and the outbox.Store and handler.IdempotencyStore interfaces. You can use the implementations in sample/infra/postgres as a reference.
   * Implement the messagebus.Broker interface for your chosen message broker. You can use sample/infra/nats as a reference.
4. **Wire Everything Together**:
   * In your main.go, instantiate your infrastructure components (DB, Broker).
   * Create an outbox.Relay and provide it with a TopicMapper function to route events to the correct topics.
   * For each subscriber, wrap its business logic handler with handler.NewIdempotentEventHandler.
   * Use the Broker to subscribe your idempotent handlers to the appropriate topics.

## **Configuration**

The sample application is configured via environment variables, which is a production-ready approach.

* APP\_DSN: The Data Source Name for the PostgreSQL connection.
  * Example: postgres://user:password@localhost:5432/cqrs\_db
* APP\_NATS\_URL: The URL for the NATS message broker.
  * Example: nats://localhost:4222
