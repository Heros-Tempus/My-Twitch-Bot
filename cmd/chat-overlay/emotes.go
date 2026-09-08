package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

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
