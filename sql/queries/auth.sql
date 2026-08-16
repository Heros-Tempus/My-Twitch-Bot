-- name: GetAuth :one
SELECT * FROM auth WHERE twitch_bot_account_id = $1;

-- name: SetAuth :one
INSERT INTO auth (
    twitch_bot_account_id,
    twitch_owner_id,
    twitch_client_id,
    twitch_client_secret,
    oauth_key,
    oauth_refresh_key
    ) 
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateAuth :exec
UPDATE auth 
SET twitch_client_id = $1, twitch_client_secret = $2, oauth_key = $3, oauth_refresh_key = $4, oauth_expires_at = $5
WHERE twitch_bot_account_id = $6;

-- name: RefreshOauth :exec
UPDATE auth 
SET oauth_key = $1, oauth_refresh_key = $2
WHERE twitch_bot_account_id = $3;

-- name: DeleteAuth :exec
DELETE FROM auth WHERE twitch_bot_account_id = $1;