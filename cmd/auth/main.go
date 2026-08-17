package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/database"
	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Oauth struct {
	BotAccountID string
	Token        string
	Refresh      string
	ClientID     string
	ClientSecret string
	ExpiresAt    time.Time
}

type twitchTokenResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresIn    int      `json:"expires_in"`
	Scope        []string `json:"scope"`
	TokenType    string   `json:"token_type"`
}

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

func refreshOath(token Oauth) (Oauth, error) {
	endpoint := "https://id.twitch.tv/oauth2/token"

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", token.Refresh)
	data.Set("client_id", token.ClientID)
	data.Set("client_secret", token.ClientSecret)

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return token, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return token, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return token, fmt.Errorf("twitch API returned unexpected status code: %d", resp.StatusCode)
	}

	var tokenResp twitchTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return token, fmt.Errorf("failed to decode response: %w", err)
	}

	token.Token = tokenResp.AccessToken
	token.Refresh = tokenResp.RefreshToken
	token.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	return token, nil
}

func notifyRabbit(chann *amqp.Channel, token Oauth, err error) {
	if err != nil {
		if err := pubsub.PublishJSON(chann, "twitch.topic", "auth.failed", token); err != nil {
			log.Fatal("Error publishing auth failed message to RabbitMQ:", err)
			return
		}
	}

	if err := pubsub.PublishJSON(chann, "twitch.auth", "auth.refreshed.#", token); err != nil {
		log.Fatal("Error publishing OAuth token to RabbitMQ:", err)
	}
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
	rabbitChan, err := con.Channel()
	if err != nil {
		fmt.Println("Error creating channel:", err)
		return
	}
	if err := pubsub.DeclareExchange(rabbitChan, "twitch.auth", "fanout"); err != nil {
		log.Fatal("Error declaring RabbitMQ fanout exchange:", err)
	}
	log.Println("Declared RabbitMQ fanout exchange")
	if err := pubsub.DeclareExchange(rabbitChan, "twitch.topic", "topic"); err != nil {
		log.Fatal("Error declaring RabbitMQ topic exchange:", err)
	}
	log.Println("Declared RabbitMQ topic exchange")

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
	}
	if err != nil {
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

	for {
		if oauth.ExpiresAt.Before(time.Now().Add(5 * time.Minute)) {
			oauth, err = refreshOath(oauth)
			if err != nil {
				log.Fatal("Error refreshing OAuth token:", err)
				notifyRabbit(rabbitChan, oauth, err)
			}
			dbQueries.RefreshOauth(context.Background(), database.RefreshOauthParams{
				OauthKey: oauth.Token,
				OauthRefreshKey: oauth.Refresh,
				TwitchBotAccountID: oauth.BotAccountID,
			})
			notifyRabbit(rabbitChan, oauth, nil)
		}
	}
}
