# Eventus Implementation Guide

This guide outlines how to implement the key components of a service using the Eventus framework.

### Project Structure

A new project should follow the structure of the `sample` application to maintain a clean separation of concerns. This organization clearly divides the "write" side (Commands) from the "read" side (Queries).

```
/
├── command/              # Holds Command structs and their handlers.
│
├── domain/               # The core of your business logic.
│   ├── aggregate/        # Contains Aggregate Root definitions and their 'on' methods.
│   └── event/            # Contains all domain Event definitions.
│
├── query/                # The "read" side of the application.
│   ├── projection/       # Event handlers that build the read models.
│   ├── query/            # Holds Query structs and their handlers.
│   ├── repository/       # Generic repositories for saving/retrieving view models.
│   └── view/             # Defines the read model data structures (the views).
│
└── main.go               # The application entry point for setup and dependency injection.
```

-----

### 1\. How to Implement an Aggregate

The **Aggregate** validates state and applies events by routing them to specific handler functions.

#### Rules:

  * An aggregate struct should hold its state and embed the `eventsrc.AggregateRoot`.
  * A constructor function must return a new, empty instance and initialize the `AggregateRoot`.
  * The `Apply` method should act as a router, dispatching each event type to a dedicated `on<EventName>` function.
  * Each `on<EventName>` function is responsible for the actual state change for that specific event.
  * The `Validate` method checks if the aggregate's current state is valid.

#### Sample Code Snippet:

```go
// In domain/aggregate/product_aggregate.go
package aggregate

import "github.com/0m3kk/eventus/eventsrc"

type ProductAggregate struct {
	*eventsrc.AggregateRoot
	Product domain.Product
}

func NewProductAggregateEmpty() *ProductAggregate {
	p := &ProductAggregate{}
	p.AggregateRoot = eventsrc.NewAggregateRoot(ProductAggregateType, p.Apply, p.Validate)
	return p
}

// Apply routes events to the correct state change function.
func (p *ProductAggregate) Apply(ctx context.Context, evt eventsrc.Event) error {
	var err error
	switch e := evt.(type) {
	case *event.ProductCreated:
		err = p.onProductCreated(e)
	default:
		return fmt.Errorf("unknown event type: %s", evt.EventType())
	}
	if err != nil {
		return err
	}
	p.SetVersion(evt.Version())
	return nil
}

// onProductCreated handles the state change for the ProductCreated event.
func (p *ProductAggregate) onProductCreated(evt *event.ProductCreated) error {
	p.SetID(evt.AggregateID())
	p.Product.Name = evt.Name
	p.Product.Price = evt.Price
	return nil
}
```

### 2\. How to Implement a Command

A **Command** is a request to change state, handled by creating and tracking an event.

#### Rules:

  * Define a simple struct for the command that holds the necessary data.
  * The command handler is responsible for the entire workflow: creating the aggregate, creating the event, passing the event to the aggregate's `TrackChange` method, and saving the aggregate.

#### Sample Code Snippet:

```go
// In command/create_product_handler.go
package command

import (
	"context"

	"github.com/google/uuid"

	"github.com/0m3kk/eventus/cqrs"
	"github.com/0m3kk/eventus/eventsrc"
)

type CreateProductHandler struct {
	repo       *repository.ProductRepository
	transactor cqrs.Transactor
}

func (h *CreateProductHandler) Handle(ctx context.Context, cmd CreateProductCommand) error {
	return h.transactor.WithTransaction(ctx, func(txCtx context.Context) error {
		p := aggregate.NewProductAggregateEmpty()

		productCreatedEvent := event.ProductCreated{
			BaseEvent: eventsrc.BaseEvent{
				ID:      uuid.New(),
				AggID:   cmd.ID,
				AggType: aggregate.ProductAggregateType,
			},
			Name:  cmd.Name,
			Price: cmd.Price,
		}

		if err := p.TrackChange(txCtx, &productCreatedEvent); err != nil {
			return err
		}
		return h.repo.Save(txCtx, p)
	})
}
```

### 3\. How to Implement an Event

An **Event** is a record of something that happened.

#### Rules:

  * Define a struct for each event, preferably embedding `eventsrc.BaseEvent`.
  * Each event must have an `EventType()` method returning a unique string identifier.
  * All events must be registered with the framework using `eventsrc.RegisterEvent()`.

#### Sample Code Snippet:

```go
// In domain/event/product_created.go
package event

import "github.com/0m3kk/eventus/eventsrc"

const ProductCreatedEventType = "ProductCreated"

type ProductCreated struct {
	eventsrc.BaseEvent
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

func (e ProductCreated) EventType() string { return ProductCreatedEventType }

// In main.go or an init() function:
func init() {
    eventsrc.RegisterEvent(&event.ProductCreated{})
}
```

### 4\. How to Implement a Projection (The Read Side)

A **Projection** builds a fast read model by listening for events and dispatching them to specific handlers.

#### Rules:

  * Define the denormalized read model in the `query/view` package.
  * The main `ProjectionHandler` should have a `Handle` method that routes events to dedicated handler functions (e.g., `handleProductCreated`).
  * Each handler function is responsible for creating the view model and saving it using a generic view repository.

#### Sample Code Snippet:

```go
// In query/projection/product_projection.go
package projection

import (
	"github.com/0m3kk/eventus/eventsrc"
)

type ProductProjectionHandler struct {
	repo *repository.ProductViewRepository // The generic view repository
}

// Handle routes events to the correct logic.
func (p *ProductProjectionHandler) Handle(ctx context.Context, evt eventsrc.OutboxEvent) error {
	switch evt.EventType {
	case event.ProductCreatedEventType:
		return p.handleProductCreated(ctx, evt)
	}
	return nil
}

// handleProductCreated processes the ProductCreated event.
func (p *ProductProjectionHandler) handleProductCreated(ctx context.Context, evt eventsrc.OutboxEvent) error {
	var createdEvt event.ProductCreated
	if err := json.Unmarshal(evt.Payload, &createdEvt); err != nil {
		return err
	}

	productView := view.ProductView{
		ID:    createdEvt.AggregateID(),
		Name:  createdEvt.Name,
		Price: createdEvt.Price,
	}

	return p.repo.Save(ctx, productView) // Using the generic Save method
}
```

### 5\. How to Implement a View and its Repository

The **View Repository** provides methods to interact with read models.

#### Rules:

  * Define the view model struct in the `query/view` package.
  * The repository in `query/repository` should provide data access methods like `Save`, `Update`, `GetByID`, and `Delete`.
  * It should also provide a `List` method that can be extended to support advanced querying.

#### Sample Code Snippet:

```go
// In query/view/product_view.go
package view

type ProductView struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Price     float64   `json:"price"`
	Version   int       `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
}

// In query/repository/view_repository.go
package repository

// ProductViewRepository provides access to the product_views table.
type ProductViewRepository struct {
	pool *pgxpool.Pool
}

// Save inserts a new ProductView.
func (r *ProductViewRepository) Save(ctx context.Context, view view.ProductView) error {
	query := "INSERT INTO product_views (id, name, price, version, updated_at) VALUES ($1, $2, $3, $4, $5)"
	_, err := r.pool.Exec(ctx, query, view.ID, view.Name, view.Price, view.Version, time.Now())
	return err
}

// Update modifies an existing ProductView.
func (r *ProductViewRepository) Update(ctx context.Context, view view.ProductView) error {
	query := "UPDATE product_views SET name = $2, price = $3, version = $4, updated_at = $5 WHERE id = $1"
	_, err := r.pool.Exec(ctx, query, view.ID, view.Name, view.Price, view.Version, time.Now())
	return err
}

// GetByID retrieves a single ProductView by its ID.
func (r *ProductViewRepository) GetByID(ctx context.Context, id uuid.UUID) (*view.ProductView, error) {
	var view view.ProductView
	query := "SELECT id, name, price, version, updated_at FROM product_views WHERE id = $1"
	err := r.pool.QueryRow(ctx, query, id).Scan(&view.ID, &view.Name, &view.Price, &view.Version, &view.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &view, nil
}

// Delete removes a ProductView by its ID.
func (r *ProductViewRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := "DELETE FROM product_views WHERE id = $1"
	_, err := r.pool.Exec(ctx, query, id)
	return err
}


// List retrieves a list of views.
// IDEA: This method can be expanded to dynamically build a SQL query
// for pagination, filtering, sorting, and searching based on a params struct.
func (r *ProductViewRepository) List(ctx context.Context, params ListParams) ([]view.ProductView, int, error) {
	// 1. Build a SELECT query with WHERE clauses for filters/search.
	// 2. Build a SELECT count(*) query for pagination.
	// 3. Add ORDER BY for sorting.
	// 4. Add LIMIT and OFFSET for pagination.
	// 5. Execute queries and return results.
	return nil, 0, fmt.Errorf("not implemented")
}
```

### 6\. How to Implement the Query Side

The **Query** side retrieves data from read models using the repository's methods.

#### Rules:

  * Define a query struct that holds any necessary parameters for getting a single item or a list.
  * The query handler takes the query struct, calls the appropriate repository method (`GetByID` or `List`), and returns the view(s).

#### Sample Code Snippet:

```go
// In query/query/list_products.go
package query

// ListProductsQuery holds parameters for the list operation.
type ListProductsQuery struct {
	repository.ListParams
}

// ListProductsHandler processes the query.
type ListProductsHandler struct {
	repo repository.ProductViewRepository
}

// Query fetches a list of products using the repository's List method.
func (h *ListProductsHandler) Query(ctx context.Context, q ListProductsQuery) ([]view.ProductView, int, error) {
	return h.repo.List(ctx, q.ListParams)
}
```
