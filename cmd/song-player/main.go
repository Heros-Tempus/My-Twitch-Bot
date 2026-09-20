package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/andreykaipov/goobs"
	"github.com/andreykaipov/goobs/api/requests/sceneitems"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load(".env")

	desktopIP := os.Getenv("DESKTOP_IP")
	obsHost := os.Getenv("OBS_IP")
	obsPort := os.Getenv("OBS_PORT")
	obsPassword := os.Getenv("OBS_PASSWORD")

	obsClient, err := goobs.New(fmt.Sprintf("%s:%s", obsHost, obsPort), goobs.WithPassword(obsPassword))
	if err != nil {
		log.Fatal("Failed to connect to OBS:", err)
	}
	log.Println("Connected to OBS WebSocket")


	app := &App{
		obs:           obsClient,
		sceneName:     os.Getenv("OBS_SCENE_NAME"),
		browserSource: os.Getenv("OBS_BROWSER_SOURCE_NAME"),
		textSource:    os.Getenv("OBS_TEXT_SOURCE_NAME"),
		desktopIP:     desktopIP,
	}
	app.startFileServer()

	idResp, err := app.obs.SceneItems.GetSceneItemId(&sceneitems.GetSceneItemIdParams{
		SceneName:  &app.sceneName,
		SourceName: &app.textSource,
	})
	if err != nil {
		log.Fatalf("Failed to get Scene Item ID for text source: %v", err)
	}
	app.textItemId = idResp.SceneItemId
	log.Printf("Successfully linked to OBS Text Source. Scene Item ID: %v", app.textItemId)

	rabbitConString := os.Getenv("RABBIT_CON_STRING")
	if err := app.setupRabbitMQ(rabbitConString); err != nil {
		log.Fatal("Error setting up RabbitMQ:", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Println("Sending bootup status_request to queue-manager...")
	app.sendStatusRequest()

	log.Println("Player is running. Waiting for instructions...")
	
	<-ctx.Done()
	log.Println("\nReceived shutdown signal. Initiating graceful shutdown...")

	app.stopTimer()
	app.setObsIdle()
	
	if app.rabbitChan != nil {
		log.Println("Closing RabbitMQ channel...")
		app.rabbitChan.Close()
	}
	if app.rabbitConn != nil {
		log.Println("Closing RabbitMQ connection...")
		app.rabbitConn.Close()
	}
	
	log.Println("Disconnecting from OBS...")
	app.obs.Disconnect()

	log.Println("Player shut down gracefully.")
}