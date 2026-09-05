package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
	"github.com/andreykaipov/goobs"
	"github.com/andreykaipov/goobs/api/requests/inputs"
	"github.com/andreykaipov/goobs/api/requests/sceneitems"
	"github.com/joho/godotenv"
	amqp "github.com/rabbitmq/amqp091-go"
)

type PlayerStatusResponse struct {
	Status        string `json:"status"`
	TimeRemaining int32  `json:"time_remaining,omitempty"`
}

type PlayerTrackPayload struct {
	Url               string `json:"url"`
	Duration          int32  `json:"duration"`
	AttributionString string `json:"attribution_string"`
}

type EmptySignal struct{}

type App struct {
	rabbit        *amqp.Channel
	obs           *goobs.Client
	browserSource string
	textSource    string
	sceneName     string
	textItemId    int

	trackChan  chan PlayerTrackPayload
	statusChan chan PlayerStatusResponse
	skipChan   chan struct{}
}

func main() {
	_ = godotenv.Load(".env")

	obsHost := os.Getenv("OBS_IP")
	obsPort := os.Getenv("OBS_PORT")
	obsPassword := os.Getenv("OBS_PASSWORD")
	
	obsClient, err := goobs.New(fmt.Sprintf("%s:%s", obsHost, obsPort), goobs.WithPassword(obsPassword))
	if err != nil {
		log.Fatal("Failed to connect to OBS:", err)
	}
	defer obsClient.Disconnect()
	log.Println("Connected to OBS WebSocket")

	rabbitConString := os.Getenv("RABBIT_CON_STRING")
	con, err := pubsub.ConnectWithBackoff(rabbitConString, 5)
	if err != nil {
		log.Fatal("Error connecting to RabbitMQ:", err)
	}
	defer con.Close()

	ch, err := con.Channel()
	if err != nil {
		log.Fatal("Error opening RabbitMQ channel:", err)
	}
	defer ch.Close()
	log.Println("Connected to RabbitMQ")

	app := &App{
		rabbit:        ch,
		obs:           obsClient,
		sceneName:     os.Getenv("OBS_SCENE_NAME"),
		browserSource: os.Getenv("OBS_BROWSER_SOURCE_NAME"),
		textSource:    os.Getenv("OBS_TEXT_SOURCE_NAME"),
		trackChan:     make(chan PlayerTrackPayload, 5),
		statusChan:    make(chan PlayerStatusResponse, 1),
		skipChan:      make(chan struct{}, 5),
	}
	idResp, err := app.obs.SceneItems.GetSceneItemId(&sceneitems.GetSceneItemIdParams{
		SceneName:  &app.sceneName,
		SourceName: &app.textSource,
	})
	if err != nil {
		log.Fatalf("Failed to get Scene Item ID for text source: %v", err)
	}
	app.textItemId = idResp.SceneItemId
	log.Printf("Successfully linked to OBS Text Source. Scene Item ID: %v", app.textItemId)

	setupSubscriptions(con, ch, app)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Println("Sending bootup status_request to queue-manager...")
	app.sendStatusRequest()

	log.Println("Player is running. Waiting for instructions...")
	app.runEventLoop(ctx)

	log.Println("Player shut down gracefully.")
}

func setupSubscriptions(con *amqp.Connection, ch *amqp.Channel, app *App) {
	_ = pubsub.SubscribeJSON(con, "player", "player.responses.track", "player.track.next", pubsub.SimpleQueueTypeDurable, func(payload PlayerTrackPayload) pubsub.AckType {
		app.trackChan <- payload
		return pubsub.AckTypeAck
	})

	_ = pubsub.SubscribeJSON(con, "player", "player.responses.status", "player.status.response", pubsub.SimpleQueueTypeDurable, func(payload PlayerStatusResponse) pubsub.AckType {
		app.statusChan <- payload
		return pubsub.AckTypeAck
	})

	_ = pubsub.SubscribeJSON(con, "player", "player.responses.skip", "player.action.skip", pubsub.SimpleQueueTypeDurable, func(msg EmptySignal) pubsub.AckType {
		app.skipChan <- struct{}{}
		return pubsub.AckTypeAck
	})
}

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