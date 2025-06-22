package projection

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/0m3kk/eventus/eventsrc"
	"github.com/0m3kk/eventus/sample/domain/event"
)

// HandleProductCreated processes the ProductCreated event.
func (p *ProductProjectionHandler) HandleProductCreated(ctx context.Context, evt eventsrc.OutboxEvent) error {
	var productCreatedEvt event.ProductCreated
	if err := json.Unmarshal(evt.Payload, &productCreatedEvt); err != nil {
		return fmt.Errorf("failed to unmarshal ProductCreated event: %w", err)
	}

	slog.InfoContext(ctx, "Projecting ProductView",
		"productID", productCreatedEvt.AggregateID(),
		"name", productCreatedEvt.Name,
		"price", productCreatedEvt.Price)

	// The actual business logic is to save the view.
	return p.repo.SaveProductView(ctx, productCreatedEvt)
}
