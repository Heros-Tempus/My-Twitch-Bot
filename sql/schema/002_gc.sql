-- +goose Up
CREATE TABLE tracks (
    video_id VARCHAR(255) PRIMARY KEY,
    url TEXT NOT NULL,
    track TEXT NOT NULL,
    artist TEXT NOT NULL,
    album TEXT,
    source_media TEXT,
    original_composer TEXT,
    duration_seconds INTEGER,
    content_id_risk BOOLEAN,
    enabled BOOLEAN
);

-- +goose Down
DROP TABLE IF EXISTS tracks;