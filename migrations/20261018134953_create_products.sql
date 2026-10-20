-- +goose Up
create table if not exists products
(
    id    bigint generated always as identity primary key,
    name  text   not null,
    price bigint not null
);