package main

import (
	"context"
	"database/sql"
	"log"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/database"
	"github.com/Heros-Tempus/My-Twitch-Bot/internal/models"
)

func (a *App) popAndPlayNextTrack(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for {
		trackData, err := a.service.queries.PopNextTrack(ctx)
		if err != nil {
			a.isIdle = true
			_ = a.service.queries.ClearCurrentPlayback(ctx)
			a.SendPlayerStatus("idle", 0)
			return
		}

		if isYouTubeVideoAvailable(trackData.Url) {
			a.isIdle = false

			_ = a.service.queries.TrackCurrentPlayback(ctx, database.TrackCurrentPlaybackParams{
				VideoID:         sql.NullString{String: trackData.Url, Valid: true},
				DurationSeconds: trackData.DurationSeconds,
			})

			t := Track{
				Artist: trackData.Artist,
				Track:  trackData.Track,
			}
			if trackData.SourceMedia.Valid {
				t.SourceMedia = trackData.SourceMedia.String
			}
			if trackData.Album.Valid {
				t.Album = trackData.Album.String
			}
			if trackData.OriginalComposer.Valid {
				t.OriginalComposer = trackData.OriginalComposer.String
			}

			payload := models.PlayerTrackPayload{
				Url:               trackData.ID,
				Duration:          trackData.DurationSeconds.Int32,
				AttributionString: buildAttribution(t),
			}

			a.SendNextTrack(payload)
			a.sendToChat("Now playing: " + payload.AttributionString)
			return
		}

		log.Printf("Track %s (%s) is unavailable. Disabling in database...", trackData.ID, trackData.Track)

		// TODO: Add disable query here
		// _ = a.service.queries.DisableTrack(ctx, trackData.ID)

	}
}
func (s *Service) StopAndWipe(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	qtx := s.queries.WithTx(tx)

	if err := qtx.TruncateQueue(ctx); err != nil {
		return err
	}
	if err := qtx.ClearCurrentPlayback(ctx); err != nil {
		return err
	}

	return tx.Commit()
}
