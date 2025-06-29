# **🚀 Eventus: A Go Framework for CQRS & Event Sourcing**

A lightweight yet robust Go framework for building event-driven, scalable, and resilient services using **Command Query Responsibility Segregation (CQRS)** and **Event Sourcing**. It comes with built-in support for common patterns required in distributed systems, such as the Transactional Outbox, Idempotent Consumers, and Snapshotting.

-----

## **Core Concepts 🧠**

This framework directly addresses the challenges of modern distributed systems by providing production-ready implementations of key patterns:

  * **CQRS (Command Query Responsibility Segregation)**: Separates the write-side (Commands) from the read-side (Queries), allowing for independent scaling and optimization of each.
  * **Event Sourcing**: Persists the full history of changes to an application's state as a sequence of events, providing a robust audit log and the ability to build new projections of data retroactively.
  * **Snapshotting** 📸: Optimizes Event Sourcing by periodically saving a full snapshot of an aggregate's state, significantly speeding up load times for aggregates with long event histories.
  * **Transactional Outbox Pattern** 🗳️: Guarantees "at-least-once" event delivery, ensuring that events are published if and only if the business transaction that created them is successfully committed.
  * **Idempotent Consumers** 🛡️: Prevents the same event from being processed more than once, achieving "exactly-once" processing semantics and making your system resilient to message redeliveries.
  * **Resiliency & Retries** 💪: Provides a configurable exponential backoff strategy for handling transient failures when processing events.

-----

## **Framework Components 📦**

The framework is organized into several core packages, each with a distinct responsibility:

| Package | Description |
| :--- | :--- |
| **event** | Defines the core `Event` interface, the `EventStore` interface for persistence, and base event structs. |
| **es** | Contains the core logic for event-sourced aggregates, including a base `Aggregate` interface, repository, and snapshotting components. |
| **outbox** | Contains the `Relay` worker implementation that polls the outbox table and sends unpublished events to a message broker. |
| **handler**| Provides the `IdempotentEventHandler`, a decorator that adds idempotency, transactional guarantees, and retry logic to any event handler. |
| **messagebus**| Defines the `Broker` interface, abstracting away the specific message broker implementation (e.g., NATS, Kafka). |
| **testutil**| Contains utilities for integration testing, including helpers to spin up ephemeral database instances using Testcontainers. |

-----

## **Sample Application 🧪**

The repository includes a sample application that demonstrates the framework's concepts by building a simple service for managing "Products".

### **Architecture & Workflow 🏗️**

The data flows from the time a command is dispatched until the read model is updated, showcasing the architectural patterns in use:

1.  **Command Processing (Write Side)** ✍️:

      * A `CreateProductCommand` is sent to the `CreateProductHandler`.
      * The handler uses the `ProductAggregate` to validate the business logic and produce a `ProductCreated` event.
      * The `ProductRepository` saves the aggregate, which atomically writes the event to two tables:
        1.  `event_store`: The append-only source of truth.
        2.  `outbox`: For reliable event delivery (Transactional Outbox Pattern).

2.  **Event Publishing** 📢:

      * A background worker (`outbox.Relay`) polls the outbox table for unpublished events.
      * It publishes these events to a NATS JetStream topic and marks them as published.

3.  **Projection Building (Read Side)** 🛠️:

      * The `ProductProjectionHandler` listens for `ProductCreated` events on the NATS topic.
      * Upon receiving an event, it updates a denormalized read table (`product_views`).
      * This process is wrapped with logic to ensure idempotency and fault tolerance.

4.  **Querying** 🔎:

      * Client applications can now query the optimized read data via the `GetProductByIDHandler`, which reads directly from the `product_views` table.

-----

## **How to Run the Application ⚙️**

The application and its dependencies (PostgreSQL, NATS) are managed with Docker.

### **Prerequisites**

  * Docker
  * Docker Compose

### **Startup**

1.  Ensure you are in the project's root directory.
2.  Run the following command to build and start the containers:
    ```bash
    docker-compose up
    ```
3.  On startup, `docker-compose.yml` will:
      * Start a PostgreSQL container and initialize the schema from `infra/postgres/schema.sql`.
      * Start a NATS container with JetStream enabled.
      * Build and run the Go application, connecting to Postgres and NATS.

When the application starts, it will automatically simulate sending a `CreateProductCommand`, allowing you to observe the entire processing flow in the logs.
