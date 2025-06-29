package query

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/0m3kk/eventus/sample/query/repository"
	"github.com/0m3kk/eventus/sample/query/view"
)

type GetProductByID struct {
	ID uuid.UUID `json:"id"`
}

// GetProductByIDHandler is a handler for retrieving a product view by its ID.
type GetProductByIDHandler struct {
	repository repository.ProductViewRepository
}

func NewGetProductByIDHandler(repository repository.ProductViewRepository) *GetProductByIDHandler {
	return &GetProductByIDHandler{repository: repository}
}

// Query retrieves a product view by its ID and returns it as a view.ProductView.
func (g GetProductByIDHandler) Query(ctx context.Context, query GetProductByID) (view.ProductView, error) {
	productView, err := g.repository.GetProductViewByID(ctx, query.ID)
	if err != nil {
		return view.ProductView{}, fmt.Errorf("get product view by id = %s failed. %w", query.ID, err)
	}
	if productView == nil {
		return view.ProductView{}, fmt.Errorf("product with id = %s not found. %w", query.ID, ErrorProductNotFound)
	}
	return *productView, nil
}
