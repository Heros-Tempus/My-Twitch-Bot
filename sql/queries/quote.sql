-- name: GetQuote :one
SELECT * FROM quotes WHERE id = $1;

-- name: GetMostRecentQuote :one
SELECT * FROM quotes ORDER BY quote_date DESC LIMIT 1;

-- name: AddQuote :exec
INSERT INTO quotes (quote, who, game) VALUES ($1, $2, $3);

-- name: RandomQuote :one
SELECT * FROM quotes ORDER BY RANDOM() LIMIT 1;

-- name: DeleteQuote :exec
DELETE FROM quotes WHERE id = $1;

-- name: UpdateQuote :exec
UPDATE quotes SET quote = $1, who = $2, game = $3 WHERE id = $4;

-- name: GetRandomQuoteByGame :one
SELECT * FROM quotes WHERE game = $1 ORDER BY RANDOM() LIMIT 1;

-- name: GetRandomQuoteByWho :one
SELECT * FROM quotes WHERE who = $1 ORDER BY RANDOM() LIMIT 1;

-- name: GetRandomQuoteByDate :one
SELECT * FROM quotes WHERE quote_date = $1 ORDER BY RANDOM() LIMIT 1;

-- name: GetRandomQuotesByFilters :many
WITH selected_quotes AS (
    SELECT *
    FROM quotes
    WHERE (sqlc.narg('quote')::text IS NULL
            OR quote ILIKE '%' || REPLACE(REPLACE(sqlc.narg('quote')::text, '%', '\%'), '_', '\_') || '%' ESCAPE '\')
        AND (sqlc.narg('who')::text IS NULL
            OR who ILIKE '%' || REPLACE(REPLACE(sqlc.narg('who')::text, '%', '\%'), '_', '\_') || '%' ESCAPE '\')
        AND (sqlc.narg('game')::text IS NULL
            OR game ILIKE '%' || REPLACE(REPLACE(sqlc.narg('game')::text, '%', '\%'), '_', '\_') || '%' ESCAPE '\')
        AND (
            (sqlc.narg('start_date')::date IS NULL AND sqlc.narg('end_date')::date IS NULL)
            OR quote_date BETWEEN COALESCE(sqlc.narg('start_date')::date, sqlc.narg('end_date')::date)
                            AND COALESCE(sqlc.narg('end_date')::date, sqlc.narg('start_date')::date)
            )
    ORDER BY RANDOM()
    LIMIT GREATEST(1, COALESCE(sqlc.narg('limit_count')::integer, 1))
)
SELECT *
FROM selected_quotes;