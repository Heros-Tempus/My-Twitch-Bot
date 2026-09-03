-- +goose Up
CREATE TABLE queue (
    id SERIAL PRIMARY KEY,
    video_id VARCHAR(255) REFERENCES tracks(video_id),
    position SERIAL
);

-- +goose Down
DROP TABLE IF EXISTS queue;