package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/database"
)

func (a *App) runManualAuthFlow(botID, clientID, clientSecret string) error {
	redirectURI := "http://localhost:8081/callback"
	scopes := "user:bot user:read:chat user:write:chat"

	// scopes to be added for microservices that have yet to be made
	// moderator:manage:announcements
	// moderator:manage:shoutouts

	authEndpoint, _ := url.Parse("https://id.twitch.tv/oauth2/authorize")
	q := authEndpoint.Query()
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", scopes)
	authEndpoint.RawQuery = q.Encode()

	authURL := authEndpoint.String()

	log.Println("\n==================================================================")
	log.Println("ACTION REQUIRED: OAUTH TOKEN INVALID OR MISSING")
	log.Println("Please click the link below to authorize the bot:")
	log.Printf("\n%s\n\n", authURL)
	log.Println("Waiting 3 minutes for authorization...")
	log.Println("==================================================================")
	a.publishOAuth(fmt.Errorf("manual auth required"))

	codeChan := make(chan string)
	errChan := make(chan error)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errStr := r.URL.Query().Get("error_description")
			fmt.Fprintf(w, "Authorization failed: %s", errStr)
			errChan <- fmt.Errorf("user denied authorization: %s", errStr)
			return
		}
		fmt.Fprintf(w, "Authorization successful! You can close this window and return to your terminal.")
		codeChan <- code
	})

	srv := &http.Server{Addr: ":8081", Handler: mux}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("OAuth callback server error: %v", err)
		}
	}()

	var finalErr error

	select {
	case code := <-codeChan:
		log.Println("Authorization code received. Exchanging for token...")
		oauth, err := exchangeAuthCode(code, clientID, clientSecret, redirectURI)
		if err != nil {
			finalErr = fmt.Errorf("failed to exchange code: %w", err)
			break
		}

		oauth.BotAccountID = botID
		oauth.OwnerID = os.Getenv("OWNER_ID")
		oauth.ClientID = clientID
		oauth.ClientSecret = clientSecret

		a.oauth = oauth
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := a.db.RefreshOauth(ctx, database.RefreshOauthParams{
			OauthKey:           a.oauth.Token,
			OauthRefreshKey:    a.oauth.Refresh,
			TwitchBotAccountID: a.oauth.BotAccountID,
		}); err != nil {
			log.Printf("Error updating database with new OAuth token: %v", err)
		} else {
			log.Println("Successfully persisted refreshed OAuth token to PostgreSQL.")
		}
		log.Println("Successfully acquired new OAuth tokens via Authorization Code Grant.")

	case err := <-errChan:
		finalErr = err
	case <-time.After(3 * time.Minute):
		finalErr = fmt.Errorf("timed out waiting for user authorization")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)

	return finalErr
}
