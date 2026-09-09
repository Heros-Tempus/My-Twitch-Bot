package main

import (
	"context"
	"sync"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/models"
	"github.com/coder/websocket"
)

type ChatUser struct {
	ID          string             `json:"id"`
	Login       string             `json:"login"`
	DisplayName string             `json:"display_name"`
	Color       string             `json:"color"`
	Badges      []models.ChatBadge `json:"badges"`
}

type ChatData struct {
	RawText   string                `json:"raw_text"`
	Fragments []models.ChatFragment `json:"fragments"`
}

type Metadata struct {
	MessageType string   `json:"message_type"`
	IsHighlight bool     `json:"is_highlight"`
	IsReply     bool     `json:"is_reply"`
	Effects     []string `json:"effects"`
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
