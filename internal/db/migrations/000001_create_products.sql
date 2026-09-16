-- Migration 000001: creates the `product` table.
-- A migration is a versioned SQL file describing schema changes.
-- It is the "git" of the database: every structural change becomes a new file.
--
-- How to apply it here (no migration tool):
--   docker compose exec go_db psql -U root -d postgres -f /dev/stdin < internal/db/migrations/000001_create_products.sql
-- or, with psql installed on your machine:
--   psql -U root -h localhost -d postgres -f internal/db/migrations/000001_create_products.sql
--
-- Naming pattern: NNNNNN_description.sql (order = apply order).

CREATE TABLE IF NOT EXISTS product (
    -- SERIAL = auto-increment (like MySQL's AUTO_INCREMENT).
    -- PRIMARY KEY guarantees a unique, NOT NULL value.
    id           SERIAL PRIMARY KEY,
    -- TEXT with NOT NULL: the product name is required at the database level.
    product_name TEXT           NOT NULL,
    -- NUMERIC(10,2) = decimal with 2 places (avoid float for money in the DB).
    -- CHECK is a validation at the database level: price can never be negative.
    price        NUMERIC(10, 2) NOT NULL CHECK (price >= 0)
);