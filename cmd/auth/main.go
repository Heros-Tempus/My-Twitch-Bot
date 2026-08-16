package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"syscall"
	"time"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/database"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	amqp "github.com/rabbitmq/amqp091-go"
)

func connectWithBackoff(uri string, maxAttempts int) (*amqp.Connection, error) {
	backoff := time.Second

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		con, err := amqp.Dial(uri)
		if err == nil {
			return con, nil
		}

		var amqpErr *amqp.Error
		if errors.As(err, &amqpErr) {
			switch amqpErr.Code {
			case 403, 530:
				return nil, fmt.Errorf("non-retryable AMQP error: %w", err)
			}
		}

		var netErr net.Error
		if errors.As(err, &netErr) || isConnRefused(err) {
			fmt.Printf("connect attempt %d failed (%v), retrying in %s\n", attempt, err, backoff)
			time.Sleep(backoff)
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			continue
		}

		return nil, fmt.Errorf("unrecoverable error connecting to RabbitMQ: %w", err)
	}

	return nil, fmt.Errorf("failed to connect after %d attempts", maxAttempts)
}

func isConnRefused(err error) bool {
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		var sysErr *os.SyscallError
		if errors.As(opErr.Err, &sysErr) {
			return errors.Is(sysErr.Err, syscall.ECONNREFUSED)
		}
	}
	return false
}

func main() {
	_ = godotenv.Load(".env")
	rabbitConString := os.Getenv("RABBIT_CON_STRING")
	con, err := connectWithBackoff(rabbitConString, 5)
	if err != nil {
		log.Fatal("Error connecting to RabbitMQ:", err)
	}
	defer con.Close()
	log.Println("Connected to RabbitMQ")

	dbConString := fmt.Sprintf("postgres://%s:%s@%s/%s",
		os.Getenv("POSTGRES_USER"),
		os.Getenv("POSTGRES_PASSWORD"),
		os.Getenv("POSTGRES_HOST"),
		os.Getenv("POSTGRES_DB"))
	db, err := sql.Open("postgres", dbConString)
	if err != nil {
		log.Fatal("Error connecting to PostgreSQL:", err)
	}
	defer db.Close()
	log.Println("Connected to PostgreSQL")

	dbQueries := database.New(db)

	auth, err := dbQueries.GetAuth(context.Background(), os.Getenv("BOT_ID"))
	if err == sql.ErrNoRows {
		botID := os.Getenv("BOT_ID")
		ownerID := os.Getenv("OWNER_ID")
		clientID := os.Getenv("CLIENT_ID")
		clientSecret := os.Getenv("CLIENT_SECRET")
		oauthKey := os.Getenv("OAUTH_KEY")
		oauthRefreshKey := os.Getenv("OAUTH_REFRESH_KEY")

		err = dbQueries.SetAuth(context.Background(), database.SetAuthParams{
			TwitchBotAccountID: botID,
			TwitchOwnerID:      ownerID,
			TwitchClientID:     clientID,
			TwitchClientSecret: clientSecret,
			OauthKey:           oauthKey,
			OauthRefreshKey:    oauthRefreshKey,
		})
		if err != nil {
			log.Fatal("Error setting auth data:", err)
		}
	}
	if err != nil {
		log.Fatal("Error fetching auth data:", err)
	}
	log.Printf("Fetched auth data for bot ID %s: %+v", auth.TwitchBotAccountID, auth)
}
