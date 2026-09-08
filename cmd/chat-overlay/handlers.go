package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
)

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

func (a *App) handleAuthFailure(msg Oauth) pubsub.AckType {
	log.Println("Received OAuth failure signal.")
	a.broadcastMessage([]byte("Received OAuth failure signal."))
	return pubsub.AckTypeNackDiscard
}

func (a *App) handleIncomingMessage(msg OverlayMessage) pubsub.AckType {
	a.Cache.mu.RLock()

	for i, badge := range msg.Badges {
		key := fmt.Sprintf("%s:%s", badge.SetID, badge.ID)
		if imageURL, exists := a.Cache.Badges[key]; exists {
			msg.Badges[i].ImageURL = imageURL
		}
	}

	msg.Fragments = enrichFragments(msg.Fragments, a.Cache)

	a.Cache.mu.RUnlock()

	a.parseEffects(&msg)

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
			cleanedText := stripTriggersCaseInsensitive(frag.Text, activeEffects)
			if cleanedText != "" {
				cleanedText += " "
			}
			msg.Fragments[i].Text = cleanedText
		}
	}
}

func stripTriggersCaseInsensitive(text string, activeEffects map[string]bool) string {
	result := text
	for trigger := range activeEffects {
		if trigger == "" {
			continue
		}
		for {
			idx := strings.Index(strings.ToLower(result), trigger)
			if idx == -1 {
				break
			}
			result = result[:idx] + result[idx+len(trigger):]
		}
	}
	return strings.TrimSpace(result)
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
