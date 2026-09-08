-- +goose Up
CREATE TABLE quotes (
    id SERIAL PRIMARY KEY,
    quote TEXT NOT NULL,
    who TEXT,
    game TEXT,
    quote_date DATE DEFAULT CURRENT_DATE
);
-- +goose Down
DROP TABLE IF EXISTS quotes;