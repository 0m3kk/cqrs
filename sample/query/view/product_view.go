package view

import (
	"time"

	"github.com/google/uuid"
)

type ProductView struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Price     float64   `json:"price"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Version   int       `json:"-"`
}
