package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Print("no env file found")
	}

	rabbitConString := os.Getenv("RABBIT_CON_STRING")
	log.Printf("RabbitMQ connection string: %s", rabbitConString)
	con, rabbitChan, err := setupRabbitMQ(rabbitConString)
	if err != nil {
		log.Fatal("Error setting up RabbitMQ:", err)
	}
	defer con.Close()
	log.Println("Connected to RabbitMQ")

	dbConString := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable",
		os.Getenv("POSTGRES_USER"),
		os.Getenv("POSTGRES_PASSWORD"),
		os.Getenv("POSTGRES_HOST"),
		os.Getenv("POSTGRES_DB"))
	log.Printf("PostgreSQL connection string: %s", dbConString)
	db, err := sql.Open("postgres", dbConString)
	if err != nil {
		log.Fatal("Error connecting to PostgreSQL:", err)
	}
	defer db.Close()
	log.Println("Connected to PostgreSQL")

	dbQueries := database.New(db)
	botID := os.Getenv("BOT_ID")

	var oauth Oauth
	var useEnvFallback bool

	auth, err := dbQueries.GetAuth(context.Background(), botID)
	if err != nil {
		log.Printf("Auth not found or error reading from DB: %v", err)
		useEnvFallback = true
	} else {
		oauth, err = refreshOath(Oauth{
			BotAccountID: auth.TwitchBotAccountID,
			OwnerID:      auth.TwitchOwnerID,
			Token:        auth.OauthKey,
			Refresh:      auth.OauthRefreshKey,
			ClientID:     auth.TwitchClientID,
			ClientSecret: auth.TwitchClientSecret,
			ExpiresAt:    auth.OauthExpiresAt.Time,
		})
		if err != nil {
			log.Printf("Failed to refresh OAuth token using DB data: %v", err)
			useEnvFallback = true
		}
	}

	if useEnvFallback {
		log.Println("Falling back to .env authentication data...")

		ownerID := os.Getenv("OWNER_ID")
		clientID := os.Getenv("CLIENT_ID")
		clientSecret := os.Getenv("CLIENT_SECRET")
		oauthKey := os.Getenv("OAUTH_KEY")
		oauthRefreshKey := os.Getenv("OAUTH_REFRESH_KEY")

		envOauth := Oauth{
			BotAccountID: botID,
			OwnerID:      ownerID,
			Token:        oauthKey,
			Refresh:      oauthRefreshKey,
			ClientID:     clientID,
			ClientSecret: clientSecret,
		}

		oauth, err = refreshOath(envOauth)
		if err != nil {
			log.Printf("Critical: Failed to refresh OAuth token from .env: %v", err)
			notifyRabbit(rabbitChan, envOauth, err)
			os.Exit(1)
		}

		_, err = dbQueries.SetAuth(context.Background(), database.SetAuthParams{
			TwitchBotAccountID: botID,
			TwitchOwnerID:      ownerID,
			TwitchClientID:     clientID,
			TwitchClientSecret: clientSecret,
			OauthKey:           oauth.Token,
			OauthRefreshKey:    oauth.Refresh,
		})
		if err != nil {
			log.Printf("Warning: Successfully authenticated via .env, but failed to save to DB: %v", err)
		} else {
			log.Println("Successfully saved .env authentication data to DB.")
		}
	}

	notifyRabbit(rabbitChan, oauth, nil)
	log.Println("OAuth service initialized successfully.")

	performRefreshLogic := func(currentOauth Oauth) Oauth {
		newOauth, err := refreshOath(currentOauth)
		if err != nil {
			log.Printf("Error refreshing OAuth token: %v", err)
			notifyRabbit(rabbitChan, currentOauth, err)
			return currentOauth
		}

		err = dbQueries.RefreshOauth(context.Background(), database.RefreshOauthParams{
			OauthKey:           newOauth.Token,
			OauthRefreshKey:    newOauth.Refresh,
			TwitchBotAccountID: newOauth.BotAccountID,
		})
		if err != nil {
			log.Printf("Error updating database with new OAuth token: %v", err)
		}

		notifyRabbit(rabbitChan, newOauth, nil)
		log.Println("Successfully refreshed OAuth token.")
		return newOauth
	}

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	refreshChan, err := setupRefreshListener(con)
	if err != nil {
		log.Printf("Warning: %v", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			remaining := time.Until(oauth.ExpiresAt)
			if oauth.ExpiresAt.Before(time.Now().Add(10 * time.Minute)) {
				log.Printf("Token expiring in %v. Refreshing OAuth token...", remaining.Round(time.Second))
				oauth = performRefreshLogic(oauth)
			} else {
				log.Printf("OAuth service healthy. Token active for another %v", remaining.Round(time.Second))
			}

		case <-refreshChan:
			log.Println("Received immediate refresh request from microservice. Refreshing...")
			oauth = performRefreshLogic(oauth)

		case sig := <-sigChan:
			log.Println("Received interrupt signal. Shutting down...", sig)
			os.Exit(0)
		}
	}
}
