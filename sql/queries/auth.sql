-- name: GetAuth :one
SELECT * FROM auth WHERE twitch_bot_account_id = $1;

-- name: SetAuth :exec
INSERT INTO auth (
    twitch_bot_account_id,
    twitch_client_id,
    twitch_client_secret,
    oauth_key,
    oath_refresh_key
    ) 
VALUES ($1, $2, $3, $4, $5);

-- name: UpdateAuth :exec
UPDATE auth 
SET twitch_client_id = $1, twitch_client_secret = $2, oauth_key = $3, oath_refresh_key = $4
WHERE twitch_bot_account_id = $5;

-- name: RefreshOath :exec
UPDATE auth 
SET oauth_key = $1, oath_refresh_key = $2
WHERE twitch_bot_account_id = $3;

-- name: DeleteAuth :exec
DELETE FROM auth WHERE twitch_bot_account_id = $1;