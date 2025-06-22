package command

import "github.com/google/uuid"

// CreateProductCommand is the command for creating a new product.
type CreateProductCommand struct {
	ID    uuid.UUID
	Name  string
	Price float64
}
