package main

import "time"

type Oauth struct {
	BotAccountID string
	Token        string
	Refresh      string
	ClientID     string
	ClientSecret string
	ExpiresAt    time.Time
}

type TwitchTokenResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresIn    int      `json:"expires_in"`
	Scope        []string `json:"scope"`
	TokenType    string   `json:"token_type"`
}
