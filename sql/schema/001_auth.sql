-- +goose Up
CREATE TABLE auth (
    twitch_bot_account_id TEXT PRIMARY KEY,
    twitch_owner_id TEXT NOT NULL,
    twitch_client_id TEXT NOT NULL,
    twitch_client_secret TEXT NOT NULL,
    oauth_key TEXT NOT NULL,
    oauth_refresh_key TEXT NOT NULL,
    oauth_expires_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- +goose Down
DROP TABLE auth;