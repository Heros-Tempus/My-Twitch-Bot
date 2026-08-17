package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	_ = godotenv.Load(".env")

	rabbitConString := os.Getenv("RABBIT_CON_STRING")
	con, rabbitChan, err := setupRabbitMQ(rabbitConString)
	if err != nil {
		log.Fatal(err)
	}
	defer con.Close()

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

		auth, err = dbQueries.SetAuth(context.Background(), database.SetAuthParams{
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
	} else if err != nil {
		log.Fatal("Error fetching auth data:", err)
	}

	oauth, err := refreshOath(Oauth{
		BotAccountID: auth.TwitchBotAccountID,
		Token:        auth.OauthKey,
		Refresh:      auth.OauthRefreshKey,
		ClientID:     auth.TwitchClientID,
		ClientSecret: auth.TwitchClientSecret,
		ExpiresAt:    auth.OauthExpiresAt.Time,
	})
	if err != nil {
		log.Fatal("Error refreshing OAuth token:", err)
		notifyRabbit(rabbitChan, oauth, err)
	}
	notifyRabbit(rabbitChan, oauth, nil)

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		<-ticker.C

		remaining := time.Until(oauth.ExpiresAt)

		if oauth.ExpiresAt.Before(time.Now().Add(10 * time.Minute)) {
			log.Printf("Token expiring in %v. Refreshing OAuth token...", remaining.Round(time.Second))

			newOauth, err := refreshOath(oauth)
			if err != nil {
				log.Printf("Error refreshing OAuth token: %v", err)
				notifyRabbit(rabbitChan, oauth, err)
				continue
			}

			oauth = newOauth

			err = dbQueries.RefreshOauth(context.Background(), database.RefreshOauthParams{
				OauthKey:           oauth.Token,
				OauthRefreshKey:    oauth.Refresh,
				TwitchBotAccountID: oauth.BotAccountID,
			})
			if err != nil {
				log.Printf("Error updating database with new OAuth token: %v", err)
			}

			notifyRabbit(rabbitChan, oauth, nil)
			log.Println("Successfully refreshed OAuth token and updated database.")

		} else {
			log.Printf("OAuth service healthy. Token active for another %v", remaining.Round(time.Second))
		}
	}
}
