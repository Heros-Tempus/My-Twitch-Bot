package main

import (
	"log"
	"time"

	"github.com/andreykaipov/goobs/api/requests/inputs"
	"github.com/andreykaipov/goobs/api/requests/sceneitems"
)

func (a *App) executeObsSpin(url string) {
	_, err := a.obs.Inputs.SetInputSettings(&inputs.SetInputSettingsParams{
		InputName:     &a.browserSource,
		InputSettings: map[string]interface{}{"url": url},
	})
	if err != nil {
		log.Printf("Failed to set OBS browser URL: %v", err)
	}

	visible := true
	_, err = a.obs.SceneItems.SetSceneItemEnabled(&sceneitems.SetSceneItemEnabledParams{
		SceneName:        &a.sceneName,
		SceneItemId:      &a.spinnerItemId,
		SceneItemEnabled: &visible,
	})
	if err != nil {
		log.Printf("Failed to make spinner source visible: %v", err)
	}
}

func (a *App) hideObsSpinner() {
	hidden := false
	_, err := a.obs.SceneItems.SetSceneItemEnabled(&sceneitems.SetSceneItemEnabledParams{
		SceneName:        &a.sceneName,
		SceneItemId:      &a.spinnerItemId,
		SceneItemEnabled: &hidden,
	})
	if err != nil {
		log.Printf("Failed to hide spinner source: %v", err)
	}
	go func() {
		time.Sleep(300 * time.Millisecond)
		_, _ = a.obs.Inputs.SetInputSettings(&inputs.SetInputSettingsParams{
			InputName:     &a.browserSource,
			InputSettings: map[string]interface{}{"url": "about:blank"},
		})
		a.mu.Lock()
		a.isSpinning = false
		a.mu.Unlock()
	}()
}
