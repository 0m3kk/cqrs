package command

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/0m3kk/eventus/cqrs"
	"github.com/0m3kk/eventus/eventsrc"
	"github.com/0m3kk/eventus/sample/domain/aggregate"
	"github.com/0m3kk/eventus/sample/domain/event"
	"github.com/0m3kk/eventus/sample/domain/repository"
)

type UpdateProduct struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Price float64   `json:"price"`
}

type UpdateProductHandler struct {
	repo       *repository.ProductRepository
	transactor cqrs.Transactor
}

func NewUpdateProductHandler(repo *repository.ProductRepository, transactor cqrs.Transactor) UpdateProductHandler {
	return UpdateProductHandler{repo: repo, transactor: transactor}
}

func (h UpdateProductHandler) Handle(ctx context.Context, cmd UpdateProduct) error {
	return h.transactor.WithTransaction(ctx, func(txCtx context.Context) error {
		p, err := h.repo.Load(txCtx, cmd.ID)
		if err != nil {
			return err
		}

		if err := p.TrackChange(txCtx, &event.ProductUpdated{
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
			return fmt.Errorf("track update product failed: %w", err)
		}

		return h.repo.Save(txCtx, p)
	})
}
