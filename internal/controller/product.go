package controller

import (
	"context"
	"encoding/json"
	"errors"
	"highload_prog/internal/entity"
	"net/http"
	"strconv"
)

type (
	ProductService interface {
		Get(ctx context.Context, productID int64) (*entity.Product, error)
		Create(ctx context.Context, name string, price int64) (int64, error)
		Delete(ctx context.Context, productID int64) (int64, error)
		Update(ctx context.Context, productID int64, name string, price int64) (int64, error)
	}
	controller struct {
		productService ProductService
	}

	createRequest struct {
		Name  string
		Price int64
	}
	updateRequest struct {
		Name  *string `json:"name"`
		Price *int64  `json:"price"`
	}
)

func New(productService ProductService) *controller {
	return &controller{productService: productService}
}

func (s *controller) Get(w http.ResponseWriter, r *http.Request) {
	ID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	product, err := s.productService.Get(r.Context(), ID)
	if err != nil {
		if errors.Is(err, entity.ErrProductNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	data, err := json.Marshal(product)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *controller) Post(w http.ResponseWriter, r *http.Request) {
	var request = createRequest{}
	if err := json.NewDecoder(r.Body).Decode((&request)); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	name := request.Name
	price := request.Price
	create, err := s.productService.Create(r.Context(), name, price)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data, err := json.Marshal(create)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, err = w.Write(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *controller) Delete(w http.ResponseWriter, r *http.Request) {
	ID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	product, err := s.productService.Delete(r.Context(), ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data, err := json.Marshal(product)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(data)
}

func (s *controller) Update(w http.ResponseWriter, r *http.Request) {
	var request = updateRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if request.Name == nil || request.Price == nil {
		http.Error(w, "name and price are required", http.StatusBadRequest)
		return
	}

	ID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	update, err := s.productService.Update(r.Context(), ID, *request.Name, *request.Price)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data, err := json.Marshal(update)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(data)
}
