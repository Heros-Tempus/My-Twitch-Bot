package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
	"github.com/joho/godotenv"
	amqp "github.com/rabbitmq/amqp091-go"
)

type App struct {
	rabbitConn *amqp.Connection
	rabbitChan *amqp.Channel
}

func main() {
	_ = godotenv.Load(".env")

	app := &App{}
	rabbitConString := os.Getenv("RABBIT_CON_STRING")
	log.Printf("Con string: %s: ", rabbitConString)
	if err := app.setupRabbitMQ(rabbitConString); err != nil {
		log.Fatalf("Failed to setup RabbitMQ: %v", err)
	}

	config, err := loadConfig("hotkeys.json")
	if err != nil {
		log.Fatalf("Failed to load hotkeys config: %v", err)
	}
	log.Printf("Loaded hotkeys config: %v", config)

	mux := http.NewServeMux()
	secretKey := os.Getenv("HOTKEY_API_SECRET")
	authMiddleware := RequireAuth(secretKey)
	for _, targetEndpoint := range config {
		route := "/" + targetEndpoint
		routingKey := pubsub.RoutingKey(targetEndpoint)

		mux.Handle("POST "+route, authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("Received POST request for route: %s", route)
			err := pubsub.PublishJSON(app.rabbitChan, pubsub.ExchangeBot, routingKey, struct{}{})
			if err != nil {
				log.Printf("Failed to publish event for route %s: %v", route, err)
				http.Error(w, "Failed to dispatch event", http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte(`{"status":"dispatched"}`))
		})))

		log.Printf("Registered POST %s -> RabbitMQ Routing Key: %s", route, routingKey)
	}
	server := &http.Server{
		Addr:    ":8083",
		Handler: mux,
	}
	go func() {
		log.Println("Hotkey listener microservice running on :8083")
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	sig := <-stop
	log.Printf("Received signal %v. Initiating graceful shutdown...", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("HTTP server forced shutdown: %v", err)
	} else {
		log.Println("HTTP server stopped cleanly")
	}

	app.cleanupRabbitMQ()

	log.Println("Shutdown complete")
}
