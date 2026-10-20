package entity

import "errors"

var (
	ErrProductNotFound = errors.New("product not found")
)

type Product struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Price int64  `json:"price"`
}
