-- name: QueueRandomTracks :many
WITH selected_tracks AS (
    SELECT video_id
    FROM tracks
    WHERE (sqlc.narg('track')::text IS NULL
           OR track ILIKE '%' || REPLACE(REPLACE(sqlc.narg('track')::text, '%', '\%'), '_', '\_') || '%' ESCAPE '\')
      AND (sqlc.narg('artist')::text IS NULL
           OR artist ILIKE '%' || REPLACE(REPLACE(sqlc.narg('artist')::text, '%', '\%'), '_', '\_') || '%' ESCAPE '\')
      AND (sqlc.narg('album')::text IS NULL
           OR album ILIKE '%' || REPLACE(REPLACE(sqlc.narg('album')::text, '%', '\%'), '_', '\_') || '%' ESCAPE '\')
      AND (sqlc.narg('source_media')::text IS NULL
           OR source_media ILIKE '%' || REPLACE(REPLACE(sqlc.narg('source_media')::text, '%', '\%'), '_', '\_') || '%' ESCAPE '\')
      AND (sqlc.narg('original_composer')::text IS NULL
           OR original_composer ILIKE '%' || REPLACE(REPLACE(sqlc.narg('original_composer')::text, '%', '\%'), '_', '\_') || '%' ESCAPE '\')
    ORDER BY RANDOM()
    LIMIT GREATEST(1, COALESCE(sqlc.narg('limit_count')::integer, 1))
)
INSERT INTO queue (video_id)
SELECT video_id
FROM selected_tracks
RETURNING video_id;

-- name: PopNextTrack :one
WITH next_in_queue AS (
    SELECT id, video_id 
    FROM queue 
    ORDER BY position ASC 
    LIMIT 1 
    FOR UPDATE SKIP LOCKED
),
popped_queue AS (
    DELETE FROM queue 
    WHERE id = (SELECT id FROM next_in_queue)
    RETURNING video_id
)
SELECT 
    t.url, 
    t.duration_seconds, 
    t.track, 
    t.artist, 
    t.album, 
    t.source_media, 
    t.original_composer
FROM popped_queue q
JOIN tracks t ON q.video_id = t.video_id;

-- name: ShuffleQueue :exec
UPDATE queue SET position = random_order.new_pos
FROM (SELECT id, ROW_NUMBER() OVER (ORDER BY RANDOM()) as new_pos FROM queue) random_order
WHERE queue.id = random_order.id;

-- name: TrackCurrentPlayback :exec
INSERT INTO current_playback (id, video_id, started_at, duration_seconds)
VALUES (1, $1, NOW(), $2)
ON CONFLICT (id) DO UPDATE 
SET video_id = EXCLUDED.video_id, started_at = EXCLUDED.started_at, duration_seconds = EXCLUDED.duration_seconds;

-- name: GetCurrentPlayback :one
SELECT * FROM current_playback WHERE id = 1;

-- name: TruncateQueue :exec
TRUNCATE TABLE queue;

-- name: ClearCurrentPlayback :exec
DELETE FROM current_playback WHERE id = 1;