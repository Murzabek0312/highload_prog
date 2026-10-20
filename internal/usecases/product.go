package usecases

import (
	"context"
	"highload_prog/internal/entity"
)

type (
	ProductRepository interface {
		Get(ctx context.Context, productID int64) (*entity.Product, error)
		Create(ctx context.Context, product *entity.Product) (int64, error)
		Delete(ctx context.Context, productID int64) (int64, error)
		Update(ctx context.Context, productID int64, name string, price int64) (int64, error)
	}
)

type service struct {
	products ProductRepository
}

func New(products ProductRepository) *service {
	return &service{products: products}
}

func (s *service) Get(ctx context.Context, productID int64) (*entity.Product, error) {
	return s.products.Get(ctx, productID)
}

func (s *service) Create(ctx context.Context, name string, price int64) (int64, error) {
	product := &entity.Product{
		Name:  name,
		Price: price,
	}
	return s.products.Create(ctx, product)
}

func (s *service) Delete(ctx context.Context, id int64) (int64, error) {
	return s.products.Delete(ctx, id)
}

func (s *service) Update(ctx context.Context, id int64, name string, price int64) (int64, error) {
	return s.products.Update(ctx, id, name, price)
}
