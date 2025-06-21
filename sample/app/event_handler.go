package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/0m3kk/cqrs/event"
	"github.com/0m3kk/cqrs/sample/domain"
)

// ProductViewProjection is a subscriber that creates a denormalized view.
// In a real app, this would write to a different database (e.g., Elasticsearch, Redis).
type ProductViewProjection struct {
	// In a real app, this would be a repository to a read model database.
}

func NewProductViewProjection() *ProductViewProjection {
	return &ProductViewProjection{}
}

// HandleProductCreated processes the ProductCreated event.
func (p *ProductViewProjection) HandleProductCreated(ctx context.Context, evt event.OutboxEvent) error {
	var productCreatedEvt domain.ProductCreated
	if err := json.Unmarshal(evt.Payload, &productCreatedEvt); err != nil {
		return fmt.Errorf("failed to unmarshal ProductCreated event: %w", err)
	}

	slog.InfoContext(ctx, "Projecting ProductView",
		"productID", productCreatedEvt.AggregateID(),
		"name", productCreatedEvt.Name,
		"price", productCreatedEvt.Price)

	// Here you would save the denormalized view to your read database.
	// For example:
	// view := ProductView{ID: evt.AggregateID, Name: evt.Name, Price: evt.Price}
	// viewRepository.Save(ctx, view)

	return nil
}
