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
		oauth, refreshErr := refreshOath(models.OAuthToken{
			BotAccountID: auth.TwitchBotAccountID,
			OwnerID:      auth.TwitchOwnerID,
			Token:        auth.OauthKey,
			Refresh:      auth.OauthRefreshKey,
			ClientID:     auth.TwitchClientID,
			ClientSecret: auth.TwitchClientSecret,
			ExpiresAt:    auth.OauthExpiresAt.Time,
		})
		
		if refreshErr == nil {
			a.oauth = oauth
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
	envOauth := models.OAuthToken{
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
		return
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
	a.oauth = oauth
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
