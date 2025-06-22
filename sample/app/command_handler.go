package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/0m3kk/eventus/cqrs"
	"github.com/0m3kk/eventus/eventsrc"
	"github.com/0m3kk/eventus/sample/domain"
)

// OutboxWriter defines an interface for saving events to the outbox.
type OutboxWriter interface {
	SaveEvents(ctx context.Context, events []eventsrc.Event) error
}

// CreateProductCommand is the command for creating a new product.
type CreateProductCommand struct {
	Name  string
	Price float64
}

// CreateProductHandler handles the CreateProductCommand.
type CreateProductHandler struct {
	outboxWriter OutboxWriter
	transactor   cqrs.Transactor
}

func NewCreateProductHandler(outboxWriter OutboxWriter, transactor cqrs.Transactor) *CreateProductHandler {
	return &CreateProductHandler{
		outboxWriter: outboxWriter,
		transactor:   transactor,
	}
}

func (h *CreateProductHandler) Handle(ctx context.Context, cmd CreateProductCommand) error {
	slog.InfoContext(ctx, "Handling CreateProductCommand", "name", cmd.Name)

	// Create the aggregate. This also creates the initial event.
	product := domain.NewProduct(cmd.Name, cmd.Price)

	// Use the transactor to ensure saving to outbox is atomic.
	err := h.transactor.WithTransaction(ctx, func(txCtx context.Context) error {
		// In a real application, you would also save the aggregate's state here.
		// For this example, we only focus on the outbox part.
		// e.g., productRepo.Save(txCtx, product)

		eventsToCommit := product.GetUncommittedEvents()
		if err := h.outboxWriter.SaveEvents(txCtx, eventsToCommit); err != nil {
			return fmt.Errorf("failed to save events to outbox: %w", err)
		}
		return nil
	})
	if err != nil {
		slog.ErrorContext(ctx, "Failed to handle CreateProductCommand", "error", err)
		return err
	}

	slog.InfoContext(ctx, "Product creation events saved to outbox", "productID", product.ID)
	return nil
}
