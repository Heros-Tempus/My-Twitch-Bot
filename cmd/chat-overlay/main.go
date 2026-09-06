package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/coder/websocket"
	"github.com/joho/godotenv"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
)

type OverlayMessage struct {
	UserID      string         `json:"user_id"`
	DisplayName string         `json:"display_name"`
	Color       string         `json:"color"`
	Badges      []ChatBadge    `json:"badges"`
	RawText     string         `json:"raw_text"`
	Fragments   []ChatFragment `json:"fragments"`
	Effects     []string       `json:"effects,omitempty"`
}

type ChatUser struct {
	ID          string      `json:"id"`
	Login       string      `json:"login"`
	DisplayName string      `json:"display_name"`
	Color       string      `json:"color"`
	Badges      []ChatBadge `json:"badges"`
}

type ChatBadge struct {
	SetID    string `json:"set_id"`
	ID       string `json:"id"`
	ImageURL string `json:"image_url,omitempty"`
}

type ChatFragment struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	EmoteID  string `json:"emote_id,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

type ChatData struct {
	RawText   string         `json:"raw_text"`
	Fragments []ChatFragment `json:"fragments"`
}

type Metadata struct {
	MessageType string   `json:"message_type"`
	IsHighlight bool     `json:"is_highlight"`
	IsReply     bool     `json:"is_reply"`
	Effects     []string `json:"effects"`
}

type Oauth struct {
	BotAccountID string
	OwnerID      string
	Token        string
	Refresh      string
	ClientID     string
	ClientSecret string
	ExpiresAt    time.Time
}

type Emote struct {
	ImageURL string
}

type OverlayCache struct {
	mu     sync.RWMutex
	Emotes map[string]Emote
	Badges map[string]string
}

func NewOverlayCache() *OverlayCache {
	return &OverlayCache{
		Emotes: make(map[string]Emote),
		Badges: make(map[string]string),
	}
}

func (c *OverlayCache) UpdateBadges(newBadges map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Badges = newBadges
}

func (c *OverlayCache) UpdateEmotes(newEmotes map[string]Emote) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, v := range newEmotes {
		c.Emotes[k] = v
	}
}

type App struct {
	Cache          *OverlayCache
	Clients        map[*websocket.Conn]context.CancelFunc
	ClientsMu      sync.Mutex
	EffectTriggers map[string]string

	TokenReady chan struct{}
	TokenOnce  sync.Once
}

func NewApp() *App {
	return &App{
		Cache:      NewOverlayCache(),
		Clients:    make(map[*websocket.Conn]context.CancelFunc),
		TokenReady: make(chan struct{}),
		EffectTriggers: map[string]string{
			"#upsidedown": "effect-upsidedown",
			"#rainbow":    "effect-rainbow",
			"#shake":      "effect-shake",
		},
	}
}

func (a *App) handleAuthUpdate(msg Oauth) pubsub.AckType {
	log.Println("Received OAuth token. Updating Twitch Badges...")
	if err := fetchTwitchBadges(msg.OwnerID, msg.Token, msg.ClientID, a.Cache); err != nil {
		log.Printf("Failed to fetch Twitch badges: %v", err)
	} else {
		log.Println("Successfully loaded Twitch badges.")
	}

	a.TokenOnce.Do(func() {
		close(a.TokenReady)
	})

	return pubsub.AckTypeAck
}
func (a *App) handleIncomingMessage(msg OverlayMessage) pubsub.AckType {
	a.parseEffects(&msg)

	a.Cache.mu.RLock()

	for i, badge := range msg.Badges {
		key := fmt.Sprintf("%s:%s", badge.SetID, badge.ID)
		if imageURL, exists := a.Cache.Badges[key]; exists {
			msg.Badges[i].ImageURL = imageURL // Flattened
		}
	}

	msg.Fragments = enrichFragments(msg.Fragments, a.Cache)

	a.Cache.mu.RUnlock()

	payload, err := json.Marshal(msg)
	if err == nil {
		a.broadcastMessage(payload)
	} else {
		log.Printf("Failed to marshal message: %v", err)
	}

	return pubsub.AckTypeAck
}

func (a *App) parseEffects(msg *OverlayMessage) {
	if msg.Effects == nil {
		msg.Effects = make([]string, 0)
	}
	rawLower := strings.ToLower(msg.RawText)

	activeEffects := make(map[string]bool)
	for trigger, cssClass := range a.EffectTriggers {
		if strings.Contains(rawLower, trigger) {
			msg.Effects = append(msg.Effects, cssClass)
			activeEffects[trigger] = true
		}
	}

	if len(activeEffects) == 0 {
		return
	}

	for i, frag := range msg.Fragments {
		if frag.Type == "text" {
			cleanedText := frag.Text
			for trigger := range activeEffects {
				cleanedText = strings.ReplaceAll(strings.ToLower(cleanedText), trigger, "")
			}
			msg.Fragments[i].Text = strings.TrimSpace(cleanedText) + " "
		}
	}
}

func (a *App) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Printf("Failed to accept websocket: %v", err)
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	defer c.Close(websocket.StatusInternalError, "closing connection")

	log.Println("OBS connected to overlay!")

	a.ClientsMu.Lock()
	a.Clients[c] = cancel
	a.ClientsMu.Unlock()

	<-ctx.Done()

	a.ClientsMu.Lock()
	delete(a.Clients, c)
	a.ClientsMu.Unlock()
	log.Println("OBS disconnected")
}

func (a *App) broadcastMessage(msgBytes []byte) {
	a.ClientsMu.Lock()
	defer a.ClientsMu.Unlock()

	for client, cancel := range a.Clients {
		ctx, writeCancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := client.Write(ctx, websocket.MessageText, msgBytes)
		writeCancel()

		if err != nil {
			log.Printf("Write error to OBS client, removing: %v", err)
			cancel()
		}
	}
}

func main() {
	_ = godotenv.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Println("Booting chat-overlay service...")
	app := NewApp()

	ownerID := os.Getenv("OWNER_ID")
	rabbitConstring := os.Getenv("RABBIT_CON_STRING")
	fmt.Printf("RabbitMQ URI: %s\n", rabbitConstring)
	log.Println("Fetching 3rd Party Emotes...")
	if err := fetch7TVEmotes(ownerID, app.Cache); err != nil {
		log.Printf("Failed to fetch 7TV emotes: %v", err)
	}
	if err := fetchBTTVEmotes(ownerID, app.Cache); err != nil {
		log.Printf("Failed to fetch BTTV emotes: %v", err)
	}
	if err := fetchFFZEmotes(ownerID, app.Cache); err != nil {
		log.Printf("Failed to fetch FFZ emotes: %v", err)
	}

	conn, err := pubsub.ConnectWithBackoff(rabbitConstring, 5)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open RabbitMQ channel: %v", err)
	}
	defer ch.Close()

	authExchange := "twitch"
	authQueue := "auth.refreshed.overlay"
	authKey := "auth.refreshed.overlay"

	chatExchange := "twitch"
	chatQueue := "twitch.chat.overlay.send"
	chatKey := "twitch.chat.overlay.send"

	if err = pubsub.DeclareExchange(ch, authExchange, "topic"); err != nil {
		log.Fatalf("Failed to declare auth exchange: %v", err)
	}
	if err = pubsub.DeclareAndBindQueue(ch, authExchange, authQueue, authKey); err != nil {
		log.Fatalf("Failed to declare auth queue: %v", err)
	}

	if err = pubsub.DeclareAndBindQueue(ch, "twitch", chatQueue, chatKey); err != nil {
		log.Fatalf("Failed to declare chat queue: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", app.handleWebSocket)
	server := &http.Server{Addr: ":8080", Handler: mux}

	go func() {
		log.Println("Overlay WebSocket server running on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	err = pubsub.SubscribeJSON(conn, authExchange, authQueue, authKey, pubsub.SimpleQueueTypeDurable, app.handleAuthUpdate)
	if err != nil {
		log.Fatalf("Failed to subscribe to auth: %v", err)
	}

	log.Println("Waiting for initial OAuth token from auth service...")

	select {
	case <-app.TokenReady:
		log.Println("Token received. Ready to process chat.")
	case <-ctx.Done():
		log.Println("Shutdown signal received during boot. Exiting.")
		server.Shutdown(context.Background())
		return
	}

	err = pubsub.SubscribeJSON(conn, chatExchange, chatQueue, chatKey, pubsub.SimpleQueueTypeDurable, app.handleIncomingMessage)
	if err != nil {
		log.Fatalf("Failed to subscribe to chat: %v", err)
	}

	<-ctx.Done()
	log.Println("\nShutdown signal received. Shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	log.Println("Graceful shutdown complete.")
}

func enrichFragments(original []ChatFragment, c *OverlayCache) []ChatFragment {
	var enriched []ChatFragment
	for _, frag := range original {
		if frag.Type != "text" {
			enriched = append(enriched, frag)
			continue
		}
		words := strings.Split(frag.Text, " ")
		var textBuffer strings.Builder
		for i, word := range words {
			if cachedEmote, isEmote := c.Emotes[word]; isEmote {
				if textBuffer.Len() > 0 {
					enriched = append(enriched, ChatFragment{Type: "text", Text: textBuffer.String()})
					textBuffer.Reset()
				}
				enriched = append(enriched, ChatFragment{Type: "emote", Text: word, ImageURL: cachedEmote.ImageURL})
				if i < len(words)-1 {
					textBuffer.WriteString(" ")
				}
			} else {
				textBuffer.WriteString(word)
				if i < len(words)-1 {
					textBuffer.WriteString(" ")
				}
			}
		}
		if textBuffer.Len() > 0 {
			enriched = append(enriched, ChatFragment{Type: "text", Text: textBuffer.String()})
		}
	}
	return enriched
}

func fetchTwitchBadges(ownerID, token, clientID string, c *OverlayCache) error {
	if ownerID == "" || token == "" || clientID == "" {
		return fmt.Errorf("missing twitch credentials")
	}

	newBadges := make(map[string]string)
	fetch := func(url string) error {
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Client-Id", clientID)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		var res struct {
			Data []struct {
				SetID    string `json:"set_id"`
				Versions []struct {
					ID         string `json:"id"`
					ImageURL4x string `json:"image_url_4x"`
				} `json:"versions"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			return err
		}
		for _, set := range res.Data {
			for _, version := range set.Versions {
				newBadges[fmt.Sprintf("%s:%s", set.SetID, version.ID)] = version.ImageURL4x
			}
		}
		return nil
	}

	if err := fetch("https://api.twitch.tv/helix/chat/badges/global"); err != nil {
		return err
	}
	if err := fetch("https://api.twitch.tv/helix/chat/badges?broadcaster_id=" + ownerID); err != nil {
		return err
	}

	c.UpdateBadges(newBadges)
	return nil
}

func fetch7TVEmotes(ownerID string, c *OverlayCache) error {
	client := &http.Client{Timeout: 10 * time.Second}
	newEmotes := make(map[string]Emote)

	respGlobal, err := client.Get("https://7tv.io/v3/emote-sets/global")
	if err == nil && respGlobal.StatusCode == http.StatusOK {
		var res struct {
			Emotes []struct {
				Name string `json:"name"`
				Data struct {
					Host struct {
						URL string `json:"url"`
					} `json:"host"`
				} `json:"data"`
			} `json:"emotes"`
		}
		if json.NewDecoder(respGlobal.Body).Decode(&res) == nil {
			for _, e := range res.Emotes {
				newEmotes[e.Name] = Emote{ImageURL: "https:" + e.Data.Host.URL + "/4x.webp"}
			}
		}
		respGlobal.Body.Close()
	}

	if ownerID != "" {
		respUser, err := client.Get(fmt.Sprintf("https://7tv.io/v3/users/twitch/%s", ownerID))
		if err == nil && respUser.StatusCode == http.StatusOK {
			var res struct {
				EmoteSet struct {
					Emotes []struct {
						Name string `json:"name"`
						Data struct {
							Host struct {
								URL string `json:"url"`
							} `json:"host"`
						} `json:"data"`
					} `json:"emotes"`
				} `json:"emote_set"`
			}
			if json.NewDecoder(respUser.Body).Decode(&res) == nil {
				for _, e := range res.EmoteSet.Emotes {
					newEmotes[e.Name] = Emote{ImageURL: "https:" + e.Data.Host.URL + "/4x.webp"}
				}
			}
			respUser.Body.Close()
		}
	}
	c.UpdateEmotes(newEmotes)
	return nil
}

func fetchBTTVEmotes(ownerID string, c *OverlayCache) error {
	client := &http.Client{Timeout: 10 * time.Second}
	newEmotes := make(map[string]Emote)
	add := func(emotes []struct {
		ID   string `json:"id"`
		Code string `json:"code"`
	}) {
		for _, e := range emotes {
			newEmotes[e.Code] = Emote{ImageURL: fmt.Sprintf("https://cdn.betterttv.net/emote/%s/3x", e.ID)}
		}
	}

	respGlobal, err := client.Get("https://api.betterttv.net/3/cached/emotes/global")
	if err == nil && respGlobal.StatusCode == http.StatusOK {
		var res []struct {
			ID   string `json:"id"`
			Code string `json:"code"`
		}
		if json.NewDecoder(respGlobal.Body).Decode(&res) == nil {
			add(res)
		}
		respGlobal.Body.Close()
	}

	if ownerID != "" {
		respUser, err := client.Get(fmt.Sprintf("https://api.betterttv.net/3/cached/users/twitch/%s", ownerID))
		if err == nil && respUser.StatusCode == http.StatusOK {
			var res struct {
				ChannelEmotes []struct {
					ID   string `json:"id"`
					Code string `json:"code"`
				} `json:"channelEmotes"`
				SharedEmotes []struct {
					ID   string `json:"id"`
					Code string `json:"code"`
				} `json:"sharedEmotes"`
			}
			if json.NewDecoder(respUser.Body).Decode(&res) == nil {
				add(res.ChannelEmotes)
				add(res.SharedEmotes)
			}
			respUser.Body.Close()
		}
	}
	c.UpdateEmotes(newEmotes)
	return nil
}

func fetchFFZEmotes(ownerID string, c *OverlayCache) error {
	client := &http.Client{Timeout: 10 * time.Second}
	newEmotes := make(map[string]Emote)
	process := func(res struct {
		Sets map[string]struct {
			Emoticons []struct {
				Name string            `json:"name"`
				URLs map[string]string `json:"urls"`
			} `json:"emoticons"`
		} `json:"sets"`
	}) {
		for _, set := range res.Sets {
			for _, e := range set.Emoticons {
				var imgURL string
				if url, ok := e.URLs["4"]; ok {
					imgURL = url
				} else if url, ok := e.URLs["2"]; ok {
					imgURL = url
				} else if url, ok := e.URLs["1"]; ok {
					imgURL = url
				}
				if imgURL != "" {
					newEmotes[e.Name] = Emote{ImageURL: "https:" + imgURL}
				}
			}
		}
	}

	respGlobal, err := client.Get("https://api.frankerfacez.com/v1/set/global")
	if err == nil && respGlobal.StatusCode == http.StatusOK {
		var res struct {
			Sets map[string]struct {
				Emoticons []struct {
					Name string            `json:"name"`
					URLs map[string]string `json:"urls"`
				} `json:"emoticons"`
			} `json:"sets"`
		}
		if json.NewDecoder(respGlobal.Body).Decode(&res) == nil {
			process(res)
		}
		respGlobal.Body.Close()
	}

	if ownerID != "" {
		respUser, err := client.Get(fmt.Sprintf("https://api.frankerfacez.com/v1/room/id/%s", ownerID))
		if err == nil && respUser.StatusCode == http.StatusOK {
			var res struct {
				Sets map[string]struct {
					Emoticons []struct {
						Name string            `json:"name"`
						URLs map[string]string `json:"urls"`
					} `json:"emoticons"`
				} `json:"sets"`
			}
			if json.NewDecoder(respUser.Body).Decode(&res) == nil {
				process(res)
			}
			respUser.Body.Close()
		}
	}
	c.UpdateEmotes(newEmotes)
	return nil
}
