package models

import "time"

type OAuthToken struct {
	BotAccountID string    `json:"BotAccountID"`
	OwnerID      string    `json:"OwnerID"`
	Token        string    `json:"Token"`
	Refresh      string    `json:"Refresh"`
	ClientID     string    `json:"ClientID"`
	ClientSecret string    `json:"ClientSecret"`
	ExpiresAt    time.Time `json:"ExpiresAt"`
}

type ChatMessage struct {
	User    string `json:"user"`
	Message string `json:"message"`
}

type Command struct {
	User string `json:"user"`
	Name string `json:"name"`
	Args string `json:"args"`
}

type PlayerStatusResponse struct {
	Status        string `json:"status"`
	TimeRemaining int32  `json:"time_remaining,omitempty"`
}

type PlayerTrackPayload struct {
	Url               string `json:"url"`
	Duration          int32  `json:"duration"`
	AttributionString string `json:"attribution_string"`
}

type EmptySignal struct{}

type OverlayMessage struct {
	UserID      string         `json:"user_id"`
	DisplayName string         `json:"display_name"`
	Color       string         `json:"color"`
	Badges      []ChatBadge    `json:"badges"`
	RawText     string         `json:"raw_text"`
	Fragments   []ChatFragment `json:"fragments"`
	Effects     []string       `json:"effects,omitempty"`
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