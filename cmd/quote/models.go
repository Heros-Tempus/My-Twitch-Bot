package main

import (
	"sync"
	"time"
)

type ChatMessage struct {
	Message string `json:"message"`
	User    string `json:"user"`
}

type QuoteAction int

const (
	ActionUnknown QuoteAction = iota
	ActionAdd
	ActionGetMostRecent
	ActionGetByID
	ActionGetRandom
	ActionGetByWho
	ActionGetByGame
	ActionGetByDate
	ActionGetByFilters
)

type Command struct {
	User string
	Name string
	Args string
}

type ParsedQuoteCommand struct {
	Action    QuoteAction
	QuoteText string
	Who       string
	Game      string
	QuoteID   int
	Limit     int
	HasDate   bool
	StartDate time.Time
	EndDate   time.Time
}

type Oauth struct {
	BotAccountID string    `json:"BotAccountID"`
	OwnerID      string    `json:"OwnerID"`
	Token        string    `json:"Token"`
	Refresh      string    `json:"Refresh"`
	ClientID     string    `json:"ClientID"`
	ClientSecret string    `json:"ClientSecret"`
	ExpiresAt    time.Time `json:"ExpiresAt"`
}

type TokenCache struct {
	mu    sync.RWMutex
	creds Oauth
}

func (tc *TokenCache) Update(newCreds Oauth) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.creds = newCreds
}

func (tc *TokenCache) Get() (token, clientID string) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.creds.Token, tc.creds.ClientID
}
