package main


import (
	"context"
	"log"
	"time"
)

func (a *App) runEventLoop(ctx context.Context) {
	var timer *time.Timer
	var timerCh <-chan time.Time

	for {
		select {
		case status := <-a.statusChan:
			if status.Status == "playback" {
				log.Printf("Resuming playback, %d seconds remaining", status.TimeRemaining)

				if timer != nil {
					timer.Stop()
				}
				timer = time.NewTimer(time.Duration(status.TimeRemaining) * time.Second)
				timerCh = timer.C
			} else {
				log.Println("Queue is idle.")
				a.setObsIdle()
			}

		case track := <-a.trackChan:
			log.Printf("Playing track: %s (%ds)", track.AttributionString, track.Duration)

			a.setObsActive(track.Url, track.AttributionString)

			if timer != nil {
				timer.Stop()
			}
			timer = time.NewTimer(time.Duration(track.Duration) * time.Second)
			timerCh = timer.C

		case <-a.skipChan:
			log.Println("Skip signal received!")
			if timer != nil {
				timer.Stop()
				timerCh = nil
			}
			a.sendReadySignal()

		case <-timerCh:
			log.Println("Track finished! Sending ready signal...")
			timerCh = nil
			a.sendReadySignal()

		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			a.setObsIdle()
			return
		}
	}
}
