package main

import (
	"context"
	"log"
	"time"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
	"github.com/andreykaipov/goobs/api/requests/inputs"
	"github.com/andreykaipov/goobs/api/requests/sceneitems"
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

func (a *App) setObsIdle() {
	hidden := false
	_, _ = a.obs.SceneItems.SetSceneItemEnabled(&sceneitems.SetSceneItemEnabledParams{
		SceneName:        &a.sceneName,
		SceneItemId:      &a.textItemId,
		SceneItemEnabled: &hidden,
	})
	_, err := a.obs.Inputs.SetInputSettings(&inputs.SetInputSettingsParams{
		InputName:     &a.browserSource,
		InputSettings: map[string]interface{}{"url": "about:blank"},
	})
	if err != nil {
		log.Printf("Failed to set OBS browser idle: %v", err)
	}

	_, err = a.obs.Inputs.SetInputSettings(&inputs.SetInputSettingsParams{
		InputName:     &a.textSource,
		InputSettings: map[string]interface{}{"text": ""},
	})
	if err != nil {
		log.Printf("Failed to clear OBS text: %v", err)
	}
}

func (a *App) setObsActive(url, attribution string) {
	url = "http://localhost:8000/yt.html?v=" + url
	_, err := a.obs.Inputs.SetInputSettings(&inputs.SetInputSettingsParams{
		InputName:     &a.browserSource,
		InputSettings: map[string]interface{}{"url": url},
	})
	if err != nil {
		log.Printf("Failed to set OBS browser URL: %v", err)
	}

	_, err = a.obs.Inputs.SetInputSettings(&inputs.SetInputSettingsParams{
		InputName:     &a.textSource,
		InputSettings: map[string]interface{}{"text": attribution},
	})
	if err != nil {
		log.Printf("Failed to set OBS attribution text: %v", err)
	}
	visible := true
	_, err = a.obs.SceneItems.SetSceneItemEnabled(&sceneitems.SetSceneItemEnabledParams{
		SceneName:        &a.sceneName,
		SceneItemId:      &a.textItemId,
		SceneItemEnabled: &visible,
	})
	if err != nil {
		log.Printf("Failed to make text source visible: %v", err)
	}

	go func() {
		time.Sleep(15 * time.Second)
		hidden := false
		_, err := a.obs.SceneItems.SetSceneItemEnabled(&sceneitems.SetSceneItemEnabledParams{
			SceneName:        &a.sceneName,
			SceneItemId:      &a.textItemId,
			SceneItemEnabled: &hidden,
		})
		if err != nil {
			log.Printf("Failed to hide text source: %v", err)
		}
	}()
}

func (a *App) sendStatusRequest() {
	const exchange = "player"
	const key = "player.signals.status_request"
	const targetQueue = "player.requests.status"

	err := pubsub.DeclareAndBindQueue(a.rabbit, exchange, targetQueue, key)
	if err != nil {
		log.Printf("Warning: Failed to pre-declare queue-manager status queue: %v", err)
	}
	if err := pubsub.PublishJSON(a.rabbit, exchange, key, EmptySignal{}); err != nil {
		log.Printf("Failed to send status_request: %v", err)
	}
}

func (a *App) sendReadySignal() {
	const exchange = "player"
	const key = "player.signals.ready"

	if err := pubsub.PublishJSON(a.rabbit, exchange, key, EmptySignal{}); err != nil {
		log.Printf("Failed to send player_ready: %v", err)
	}
}
