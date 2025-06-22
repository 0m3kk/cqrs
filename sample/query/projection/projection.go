package projection

import (
	"context"

	"github.com/0m3kk/eventus/eventsrc"
	"github.com/0m3kk/eventus/sample/domain/event"
	"github.com/0m3kk/eventus/sample/query/repository"
)

// ProductProjectionHandler is a subscriber that creates a denormalized view.
// In a real app, this would write to a different database (e.g., Elasticsearch, Redis).
type ProductProjectionHandler struct {
	repo *repository.ProductViewRepository
}

func NewProductProjectionHandler(repo *repository.ProductViewRepository) *ProductProjectionHandler {
	return &ProductProjectionHandler{repo: repo}
}

func (p *ProductProjectionHandler) Handle(ctx context.Context, evt eventsrc.OutboxEvent) error {
	switch evt.EventType {
	case event.ProductCreatedEventType:
		return p.HandleProductCreated(ctx, evt)
	}
	return nil
}
