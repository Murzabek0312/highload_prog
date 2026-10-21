package repository

import (
	"context"
	"errors"
	"highload_prog/internal/entity"
	"highload_prog/internal/repository/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type repository struct {
	queries *db.Queries
}

func New(pool *pgxpool.Pool) *repository {
	return &repository{
		queries: db.New(pool),
	}
}

func (r *repository) Get(ctx context.Context, productID int64) (*entity.Product, error) {
	found, err := r.queries.GetProduct(ctx, productID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, entity.ErrProductNotFound
		}
		return nil, err
	}

	return &entity.Product{
		ID:    found.ID,
		Name:  found.Name,
		Price: found.Price,
	}, err
}

func (r *repository) Create(ctx context.Context, product *entity.Product) (int64, error) {
	p, err := r.queries.CreateProduct(ctx, db.CreateProductParams{
		Name:  product.Name,
		Price: product.Price,
	})
	if err != nil {
		return -1, err
	}
	return p.ID, err
}

func (r *repository) Delete(ctx context.Context, productID int64) (int64, error) {
	return r.queries.DeleteProduct(ctx, productID)
}
func (r *repository) Update(ctx context.Context, productID int64, name string, price int64) (int64, error) {
	return r.queries.UpdateProduct(ctx, db.UpdateProductParams{
		ID:    productID,
		Name:  name,
		Price: price,
	})

}
