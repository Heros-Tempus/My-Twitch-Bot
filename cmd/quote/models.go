package main

import (
	"sync"
	"time"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/models"
)

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

type TokenCache struct {
	mu    sync.RWMutex
	creds models.OAuthToken
}

func (tc *TokenCache) Update(newCreds models.OAuthToken) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.creds = newCreds
}

func (tc *TokenCache) Get() (token, clientID string) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.creds.Token, tc.creds.ClientID
}
