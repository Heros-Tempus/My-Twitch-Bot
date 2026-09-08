package main

import (
	"context"
	"database/sql"
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

func loadOAuth(dbQueries *database.Queries, botID string) Oauth {
	auth, err := dbQueries.GetAuth(context.Background(), botID)
	if err != nil {
		log.Printf("Auth not found or error reading from DB: %v", err)
		return Oauth{}
	}
	oauth, err := refreshOath(Oauth{
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
		return Oauth{}
	}
	return oauth
}

func (a *App) loadEnvOAuth(botID string) Oauth {
	log.Println("Falling back to .env authentication data...")
	envOauth := Oauth{
		BotAccountID: botID,
		OwnerID:      os.Getenv("OWNER_ID"),
		Token:        os.Getenv("OAUTH_KEY"),
		Refresh:      os.Getenv("OAUTH_REFRESH_KEY"),
		ClientID:     os.Getenv("CLIENT_ID"),
		ClientSecret: os.Getenv("CLIENT_SECRET"),
	}

	oauth, err := refreshOath(envOauth)
	if err != nil {
		log.Printf("Critical: Failed to refresh OAuth token from .env: %v", err)
		a.publishOAuth(err)
		return Oauth{}
	}

	_, err = a.db.SetAuth(context.Background(), database.SetAuthParams{
		TwitchBotAccountID: botID,
		TwitchOwnerID:      envOauth.OwnerID,
		TwitchClientID:     envOauth.ClientID,
		TwitchClientSecret: envOauth.ClientSecret,
		OauthKey:           oauth.Token,
		OauthRefreshKey:    oauth.Refresh,
	})
	if err != nil {
		log.Printf("Warning: Successfully authenticated via .env, but failed to save to DB: %v", err)
	} else {
		log.Println("Successfully saved .env authentication data to DB.")
	}
	return oauth
}

func (a *App) publishOAuth(err error) {
	notifyRabbit(a.rabbitChan, a.oauth, err)
}

func (a *App) refreshOAuth() {
	newOauth, err := refreshOath(a.oauth)
	if err != nil {
		log.Printf("Error refreshing OAuth token: %v", err)
		a.publishOAuth(err)
		return
	}

	if err := a.db.RefreshOauth(context.Background(), database.RefreshOauthParams{
		OauthKey:           newOauth.Token,
		OauthRefreshKey:    newOauth.Refresh,
		TwitchBotAccountID: newOauth.BotAccountID,
	}); err != nil {
		log.Printf("Error updating database with new OAuth token: %v", err)
	}

	a.oauth = newOauth
	a.publishOAuth(nil)
	log.Println("Successfully refreshed OAuth token.")
}

func (a *App) run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			remaining := time.Until(a.oauth.ExpiresAt)
			if a.oauth.ExpiresAt.Before(time.Now().Add(10 * time.Minute)) {
				log.Printf("Token expiring in %v. Refreshing OAuth token...", remaining.Round(time.Second))
				a.refreshOAuth()
			} else {
				log.Printf("OAuth service healthy. Token active for another %v", remaining.Round(time.Second))
			}

		case <-a.refreshChan:
			log.Println("Received immediate refresh request from microservice. Refreshing...")
			a.refreshOAuth()

		case <-ctx.Done():
			log.Println("Received shutdown signal. Shutting down...")
			return
		}
	}
}
