package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/models"
	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
)

func (a *App) stopTimer() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.timer != nil {
		a.timer.Stop()
		a.timer = nil
		return true
	}
	return false
}

func (a *App) scheduleTimer(durationSeconds int32, onComplete func()) {
	a.mu.Lock()
	if a.timer != nil {
		a.timer.Stop()
	}
	timer := time.NewTimer(time.Duration(durationSeconds) * time.Second)
	a.timer = timer
	a.mu.Unlock()

	go func() {
		<-timer.C
		a.mu.Lock()
		if a.timer == timer {
			a.timer = nil
		}
		a.mu.Unlock()
		onComplete()
	}()
}

func (a *App) handleTrack(track models.PlayerTrackPayload) pubsub.AckType {
	log.Printf("Loading track into browser: %s (%ds)", track.AttributionString, track.Duration)
	
	a.mu.Lock()
	a.currentAttribution = track.AttributionString
	a.mu.Unlock()

	a.scheduleTimer(track.Duration, func() {
		log.Println("Track finished! Sending ready signal...")
		a.sendReadySignal()
	})

	a.loadObsVideo(track.Url)
	
	return pubsub.AckTypeAck
}

func (a *App) handleTrackPlaying(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	a.mu.Lock()
	attribution := a.currentAttribution
	a.mu.Unlock()

	if attribution != "" {
		a.showObsAttribution(attribution)
	}
	w.WriteHeader(http.StatusOK)
}

func (a *App) handleStatus(status models.PlayerStatusResponse) pubsub.AckType {
	if status.Status == "playback" {
		log.Printf("Resuming playback, %d seconds remaining", status.TimeRemaining)
		a.scheduleTimer(status.TimeRemaining, func() {
			log.Println("Track finished! Sending ready signal...")
			a.sendReadySignal()
		})
		return pubsub.AckTypeAck
	}

	log.Println("Queue is idle.")
	a.setObsIdle()
	return pubsub.AckTypeAck
}

func (a *App) handleSkip(msg models.EmptySignal) pubsub.AckType {
	log.Println("Skip signal received!")
	if a.stopTimer() {
		a.sendReadySignal()
	}
	return pubsub.AckTypeAck
}

func (a *App) handleDisableTrack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		VideoID string `json:"video_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	log.Printf("Frontend reported track %s as unplayable. Aborting timer...", payload.VideoID)

	if err := pubsub.PublishJSON(a.rabbitChan, pubsub.ExchangeBot, pubsub.KeySongDisable, models.TrackDisablePayload{VideoID: payload.VideoID}); err != nil {
		log.Printf("Failed to publish disable signal: %v", err)
	}

	if a.stopTimer() {
		a.sendReadySignal()
	}

	w.WriteHeader(http.StatusOK)
}
