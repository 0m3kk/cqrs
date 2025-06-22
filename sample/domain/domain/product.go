package domain

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// Product is our aggregate root.
type Product struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Price float64   `json:"price"`
}

func (p Product) Validate() error {
	if p.Name == "" {
		return errors.New("product name cannot be empty")
	}
	if p.Price <= 0 {
		return fmt.Errorf("product price must be positive, but got %f", p.Price)
	}
	return nil
}
