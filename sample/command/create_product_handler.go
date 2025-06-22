package command

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/0m3kk/eventus/cqrs"
	"github.com/0m3kk/eventus/eventsrc"
	"github.com/0m3kk/eventus/sample/domain/aggregate"
	"github.com/0m3kk/eventus/sample/domain/event"
	"github.com/0m3kk/eventus/sample/domain/repository"
)

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

	err := h.transactor.WithTransaction(ctx, func(txCtx context.Context) error {
		// Create the new product aggregate.
		p := aggregate.NewProductAggregateEmpty()

		// Trace change
		if err := p.TrackChange(txCtx, event.ProductCreated{
			BaseEvent: eventsrc.BaseEvent{
				ID:      uuid.New(),
				AggID:   uuid.New(),
				AggType: aggregate.ProductAggregateType,
			},
			Name:  cmd.Name,
			Price: cmd.Price,
		}); err != nil {
			return fmt.Errorf("trace create product failed. %w", err)
		}

		// Save the aggregate. The repository will persist its uncommitted events.
		if err := h.repo.Save(txCtx, p); err != nil {
			return fmt.Errorf("failed to save new product: %w", err)
		}

		slog.InfoContext(txCtx, "Product aggregate saved successfully", "productID", p.ID())
		return nil
	})
	if err != nil {
		slog.ErrorContext(ctx, "Failed to handle CreateProductCommand", "error", err)
		return err
	}

	return nil
}
