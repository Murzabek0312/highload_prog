-- name: CreateProduct :one
insert into products (name, price)
values ($1, $2)
    returning id, name, price;

-- name: GetProduct :one
select * from products where id = $1;

-- name: DeleteProduct :execrows
delete from products where id = $1;

-- name: UpdateProduct :execrows
update products
set name = $1, price = $2
where id = $3;
