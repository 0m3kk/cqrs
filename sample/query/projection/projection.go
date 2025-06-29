package projection

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/0m3kk/eventus/eventsrc"
	"github.com/0m3kk/eventus/sample/domain/event"
	"github.com/0m3kk/eventus/sample/query/repository"
	"github.com/0m3kk/eventus/sample/query/view"
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
		return p.handleProductCreated(ctx, evt)
	case event.ProductUpdatedEventType:
		return p.handleProductUpdated(ctx, evt)
	}
	return nil
}

// handleProductCreated processes the ProductCreated event.
func (p *ProductProjectionHandler) handleProductCreated(ctx context.Context, evt eventsrc.OutboxEvent) error {
	var productCreatedEvt event.ProductCreated
	if err := json.Unmarshal(evt.Payload, &productCreatedEvt); err != nil {
		return fmt.Errorf("failed to unmarshal ProductCreated event: %w", err)
	}

	slog.InfoContext(ctx, "Projecting ProductView",
		"productID", productCreatedEvt.AggregateID(),
		"name", productCreatedEvt.Name,
		"price", productCreatedEvt.Price)

	// The actual business logic is to save the view.
	if err := p.repo.SaveProductView(ctx, view.ProductView{
		ID:        productCreatedEvt.AggID,
		Name:      productCreatedEvt.Name,
		Price:     productCreatedEvt.Price,
		CreatedAt: evt.Ts,
		UpdatedAt: evt.Ts,
		Version:   evt.Version,
	}); err != nil {
		slog.ErrorContext(ctx, "save product view failed", "error", err)
	}
	return nil
}

func (p *ProductProjectionHandler) handleProductUpdated(ctx context.Context, evt eventsrc.OutboxEvent) error {
	var productUpdatedEvt event.ProductUpdated
	if err := json.Unmarshal(evt.Payload, &productUpdatedEvt); err != nil {
		return fmt.Errorf("failed to unmarshal ProductUpdated event: %w", err)
	}

	slog.InfoContext(ctx, "Projecting ProductView",
		"productID", productUpdatedEvt.AggregateID(),
		"name", productUpdatedEvt.Name,
		"price", productUpdatedEvt.Price)

	// The actual business logic is to save the view.
	if err := p.repo.UpdateProductView(ctx, view.ProductView{
		ID:        productUpdatedEvt.AggID,
		Name:      productUpdatedEvt.Name,
		Price:     productUpdatedEvt.Price,
		UpdatedAt: evt.Ts,
		Version:   evt.Version,
	}); err != nil {
		slog.ErrorContext(ctx, "update product view failed", "error", err)
	}
	return nil
}
