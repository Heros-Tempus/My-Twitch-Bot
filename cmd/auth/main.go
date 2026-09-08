package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	_ = godotenv.Load(".env")

	rabbitConString := os.Getenv("RABBIT_CON_STRING")
	log.Printf("RabbitMQ connection string: %s", rabbitConString)
	con, rabbitChan, err := setupRabbitMQ(rabbitConString)
	if err != nil {
		log.Fatal("Error setting up RabbitMQ:", err)
	}
	defer con.Close()
	log.Println("Connected to RabbitMQ")

	dbConString := os.Getenv("POSTGRES_CON_STRING")
	log.Printf("PostgreSQL connection string: %s", dbConString)
	db, err := sql.Open("postgres", dbConString)
	if err != nil {
		log.Fatal("Error connecting to PostgreSQL:", err)
	}
	defer db.Close()
	log.Println("Connected to PostgreSQL")

	dbQueries := database.New(db)
	botID := os.Getenv("BOT_ID")
	refreshChan, err := setupRefreshListener(con)
	if err != nil {
		log.Printf("Warning: %v", err)
	}

	app := newApp(dbQueries, con, rabbitChan, refreshChan)
	app.oauth = loadOAuth(app.db, botID)
	if app.oauth.Token == "" {
		app.oauth = app.loadEnvOAuth(botID)
		if app.oauth.Token == "" {
			log.Fatal("Critical: OAuth initialization failed")
		}
	}
	app.publishOAuth(nil)
	log.Println("OAuth service initialized successfully.")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	app.run(ctx)
}
