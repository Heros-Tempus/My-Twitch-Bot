-- +goose Up
CREATE TABLE current_playback (
    id INTEGER PRIMARY KEY,
    video_id VARCHAR(255),
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    duration_seconds INTEGER
);

-- +goose Down
DROP TABLE IF EXISTS current_playback;