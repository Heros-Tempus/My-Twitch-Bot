package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
	"github.com/andreykaipov/goobs"
	"github.com/andreykaipov/goobs/api/requests/sceneitems"
	"github.com/joho/godotenv"
)

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

	startFileServer()

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

//go:embed yt.html
var ytHTML []byte

func startFileServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/yt.html", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write(ytHTML)
	})
	go func() {
		log.Println("serving yt.html on :8000")
		log.Fatal(http.ListenAndServe("0.0.0.0:8000", mux))
	}()
}

func buildBrowserSourceURL(host, videoID string) string {
	u := url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(host, "8000"),
		Path:   "/yt.html",
	}
	q := u.Query()
	q.Set("v", videoID)
	u.RawQuery = q.Encode()
	return u.String()
}
