package main

import (
	"log"
	"os"
	"time"

	"github.com/andreykaipov/goobs/api/requests/filters"
	"github.com/andreykaipov/goobs/api/requests/inputs"
	"github.com/andreykaipov/goobs/api/requests/sceneitems"
)

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

	a.triggerAudioDuck("Duck In", "Duck Out")
}

func (a *App) setObsActive(url, attribution string) {
	url = buildBrowserSourceURL(os.Getenv("DESKTOP_IP"), url)
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

	a.triggerAudioDuck("Duck Out", "Duck In")

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

func (a *App) triggerAudioDuck(enableFilter, disableFilter string) {
	source := "Game Audio"
	enabled := true
	disabled := false

	_, err := a.obs.Filters.SetSourceFilterEnabled(&filters.SetSourceFilterEnabledParams{
		SourceName:    &source,
		FilterName:    &enableFilter,
		FilterEnabled: &enabled,
	})
	if err != nil {
		log.Printf("Failed to enable %s filter: %v", enableFilter, err)
	}

	_, err = a.obs.Filters.SetSourceFilterEnabled(&filters.SetSourceFilterEnabledParams{
		SourceName:    &source,
		FilterName:    &disableFilter,
		FilterEnabled: &disabled,
	})
	if err != nil {
		log.Printf("Failed to disable %s filter: %v", disableFilter, err)
	}
}
