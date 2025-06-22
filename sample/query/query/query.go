package query

import "github.com/google/uuid"

type GetProductByID struct {
	ID uuid.UUID `json:"id"`
}
