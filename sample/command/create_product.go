package command

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/0m3kk/eventus/cqrs"
	"github.com/0m3kk/eventus/eventsrc"
	"github.com/0m3kk/eventus/sample/domain/aggregate"
	"github.com/0m3kk/eventus/sample/domain/event"
	"github.com/0m3kk/eventus/sample/domain/repository"
)

// CreateProductCommand is the command for creating a new product.
type CreateProductCommand struct {
	ID    uuid.UUID
	Name  string
	Price float64
}

// CreateProductHandler handles the CreateProductCommand.
type CreateProductHandler struct {
	repo       *repository.ProductRepository
	transactor cqrs.Transactor
}

func NewCreateProductHandler(repo *repository.ProductRepository, transactor cqrs.Transactor) *CreateProductHandler {
	return &CreateProductHandler{
		repo:       repo,
		transactor: transactor,
	}
}

func (h *CreateProductHandler) Handle(ctx context.Context, cmd CreateProductCommand) error {
	slog.InfoContext(ctx, "Handling CreateProductCommand", "name", cmd.Name)

	// The entire operation is wrapped in a transaction to ensure atomicity.
	// If any step fails, all database changes will be rolled back.
	err := h.transactor.WithTransaction(ctx, func(txCtx context.Context) error {
		// 1. Create the new product aggregate.
		p := aggregate.NewProductAggregateEmpty()

		// 2. Apply a change to the aggregate. This creates a `ProductCreated` event
		// and adds it to the aggregate's list of uncommitted events.
		if err := p.TrackChange(txCtx, event.ProductCreated{
			BaseEvent: eventsrc.BaseEvent{
				ID:      uuid.New(),
				AggID:   cmd.ID,
				AggType: aggregate.ProductAggregateType,
				Ver:     p.Version() + 1,
				Ts:      time.Now().UTC(),
			},
			Name:  cmd.Name,
			Price: cmd.Price,
		}); err != nil {
			return fmt.Errorf("trace create product failed. %w", err)
		}

		// 3. Save the aggregate. This is the key step where the outbox pattern is implemented.
		// The `repo.Save` method calls the underlying `EventStore`, which is configured
		// to perform two actions atomically within the current transaction (txCtx):
		//   a) It saves the new event(s) to the `event_store` table.
		//   b) It saves the same event(s) to the `outbox` table for later publishing.
		if err := h.repo.Save(txCtx, p); err != nil {
			return fmt.Errorf("failed to save new product: %w", err)
		}

		slog.InfoContext(txCtx, "Product aggregate and outbox events saved successfully", "productID", p.ID())
		return nil
	})
	if err != nil {
		slog.ErrorContext(ctx, "Failed to handle CreateProductCommand", "error", err)
		return err
	}

	return nil
}
