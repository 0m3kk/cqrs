# **Sample CQRS & Event Sourcing Application**

This application is a complete, working example that demonstrates the concepts of **Command Query Responsibility Segregation (CQRS)** and **Event Sourcing** using the provided framework. It builds a small service for managing "Products".

## **Architecture**

The sample application is structured to clearly separate different concerns, following the principles of Clean Architecture and CQRS.

* **main.go**: This is the application's entry point. It is responsible for initializing and wiring together all components, including the database connection (PostgreSQL), message broker (NATS), repositories, handlers, and background workers (outbox relay).
* **command/**: Contains Command definitions (e.g., CreateProductCommand) and the Command Handlers (CreateProductHandler) responsible for executing state-changing requests.
* **domain/**: The core of the business logic.
  * aggregate/: Defines the ProductAggregate, which is the aggregate root that processes commands and generates domain events.
  * domain/: Contains core domain objects (e.g., Product).
  * event/: Defines events (e.g., ProductCreated) that represent changes that have occurred.
  * repository/: Provides a repository for saving and loading the ProductAggregate from the event store.
* **query/**: Responsible for the "read" side of the system.
  * projection/: Contains event handlers (ProductProjectionHandler) that listen to events and build denormalized read models.
  * repository/: A repository (ProductViewRepository) for querying these read models.
  * view/: Defines the data structures for the read models (ProductView).
  * query/: Contains Query definitions and Query Handlers (GetProductByIDHandler) for retrieving read data.

## **Workflow**

The data flow, from the time a command is dispatched until the read model is updated, clearly demonstrates the architectural patterns in use.

1. **Command Processing (Write Side)**:
   * A CreateProductCommand is sent to the CreateProductHandler.
   * The handler creates a new ProductAggregate and calls a method on it, passing in data from the command.
   * The aggregate validates the business logic and, if successful, produces a ProductCreated event. This event is applied to change the aggregate's state and is added to a list of uncommitted events.
   * The ProductRepository saves the aggregate. This action writes the event to two tables within the same database transaction:
     1. **event\_store**: An append-only table that is the source of truth for the aggregate's state.
     2. **outbox**: A table used to ensure reliable event delivery to the message broker (Transactional Outbox Pattern).
2. **Event Publishing**:
   * A background worker (outbox.Relay) periodically polls the outbox table for unpublished events.
   * It publishes these events to a topic on NATS JetStream and then marks them as published in the database.
3. **Projection Building (Read Side)**:
   * A subscriber (ProductProjectionHandler) listens for ProductCreated events on the NATS topic.
   * Upon receiving an event, the handler unmarshals it and updates a separate read table (product\_views) with denormalized data from the event.
   * This entire subscriber process is wrapped with logic to ensure idempotency (not processing the same event twice) and fault tolerance.
4. **Querying**:
   * Client applications can now query the optimized read data via the GetProductByIDHandler.
   * This handler reads directly from the product\_views table, which is highly performant as it avoids rebuilding state from the event store.

## **How to Run the Application**

The application and its dependencies (PostgreSQL, NATS) are managed with Docker.

### **Prerequisites**

* Docker
* Docker Compose

### **Startup**

1. Ensure you are in the project's root directory.
2. Run the following command to build and start the containers:
   docker-compose up

3. On startup, the docker-compose.yml file will:
   * Start a PostgreSQL container and automatically create the necessary database schema from infra/postgres/schema.sql.
   * Start a NATS container with JetStream enabled.
   * Build and run the Go application, configuring environment variables (APP\_DSN, APP\_NATS\_URL) to connect to Postgres and NATS.

When the application starts, it will automatically simulate sending a CreateProductCommand after 3 seconds, allowing you to observe the entire processing flow in the logs.
