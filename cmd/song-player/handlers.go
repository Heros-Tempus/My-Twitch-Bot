package main

import (
	"log"
	"time"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/models"
	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
)

func (a *App) stopTimer() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.timer != nil {
		a.timer.Stop()
		a.timer = nil
	}
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
	log.Printf("Playing track: %s (%ds)", track.AttributionString, track.Duration)
	a.setObsActive(track.Url, track.AttributionString)
	a.scheduleTimer(track.Duration, func() {
		log.Println("Track finished! Sending ready signal...")
		a.sendReadySignal()
	})
	return pubsub.AckTypeAck
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
	a.stopTimer()
	log.Println("Skip signal received!")
	a.sendReadySignal()
	return pubsub.AckTypeAck
}