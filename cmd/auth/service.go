package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/database"
	"github.com/Heros-Tempus/My-Twitch-Bot/internal/models"
)

func (a *App) loadOAuth(botID string) {
	clientID := os.Getenv("CLIENT_ID")
	clientSecret := os.Getenv("CLIENT_SECRET")

	auth, err := a.db.GetAuth(context.Background(), botID)
	if err == nil {
		a.oauth = models.OAuthToken{
			BotAccountID: auth.TwitchBotAccountID,
			OwnerID:      auth.TwitchOwnerID,
			Token:        auth.OauthKey,
			Refresh:      auth.OauthRefreshKey,
			ClientID:     auth.TwitchClientID,
			ClientSecret: auth.TwitchClientSecret,
			ExpiresAt:    auth.OauthExpiresAt.Time,
		}
		refreshErr := a.refreshOAuth()
		if refreshErr == nil {
			return
		}
		log.Printf("Failed to refresh DB token: %v", refreshErr)
	} else {
		log.Printf("No valid auth found in DB for bot %s.", botID)
	}

	err = a.runManualAuthFlow(botID, clientID, clientSecret)
	if err == nil {
		_, dbErr := a.db.SetAuth(context.Background(), database.SetAuthParams{
			TwitchBotAccountID: botID,
			TwitchOwnerID:      a.oauth.OwnerID,
			TwitchClientID:     clientID,
			TwitchClientSecret: clientSecret,
			OauthKey:           a.oauth.Token,
			OauthRefreshKey:    a.oauth.Refresh,
		})
		if dbErr != nil {
			log.Printf("Warning: Failed to save newly granted token to DB: %v", dbErr)
		}
		return
	}

	log.Printf("Manual authorization failed or timed out: %v", err)

	a.loadEnvOAuth(botID)
}

func (a *App) loadEnvOAuth(botID string) {
	log.Println("Falling back to .env authentication data...")
	a.oauth = models.OAuthToken{
		BotAccountID: botID,
		OwnerID:      os.Getenv("OWNER_ID"),
		Token:        os.Getenv("OAUTH_KEY"),
		Refresh:      os.Getenv("OAUTH_REFRESH_KEY"),
		ClientID:     os.Getenv("CLIENT_ID"),
		ClientSecret: os.Getenv("CLIENT_SECRET"),
	}
	err := a.refreshOAuth()
	if err != nil {
		log.Printf("Critical: Failed to refresh OAuth token from .env: %v", err)
		a.publishOAuth(err)
		return
	}
}

func (a *App) refreshOAuth() error {
	newOauth, err := refreshOath(a.oauth)
	if err != nil {
		log.Printf("Error refreshing OAuth token: %v", err)
		a.publishOAuth(err)
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Println("Persisting refreshed OAuth token to PostgreSQL...")
	if err := a.db.RefreshOauth(ctx, database.RefreshOauthParams{
		OauthKey:           newOauth.Token,
		OauthRefreshKey:    newOauth.Refresh,
		TwitchBotAccountID: newOauth.BotAccountID,
	}); err != nil {
		log.Printf("Error updating database with new OAuth token: %v", err)
	} else {
		log.Println("Successfully persisted refreshed OAuth token to PostgreSQL.")
	}

	a.oauth = newOauth
	log.Println("Successfully refreshed OAuth token. Publishing update...")
	a.publishOAuth(nil)
	log.Println("Successfully published refreshed OAuth token.")
	return nil
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
