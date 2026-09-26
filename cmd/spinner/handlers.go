package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/models"
	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
)

func (a *App) handleSpinTrigger(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	a.mu.Lock()
	if a.isSpinning {
		a.mu.Unlock()
		log.Println("Spin in progress. Ignoring duplicate hotkey.")
		w.WriteHeader(http.StatusOK)
		return
	}
	a.isSpinning = true
	a.mu.Unlock()

	file, err := os.Open(a.dataPath)
	if err != nil {
		http.Error(w, "Failed to read wheel items text file", http.StatusInternalServerError)
		a.mu.Lock()
		a.isSpinning = false
		a.mu.Unlock()
		return
	}
	defer file.Close()

	var items []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			items = append(items, line)
		}
	}

	if err := scanner.Err(); err != nil || len(items) == 0 {
		http.Error(w, "Failed to parse items or file is empty", http.StatusInternalServerError)
		a.mu.Lock()
		a.isSpinning = false
		a.mu.Unlock()
		return
	}

	winnerIndex := rand.Intn(len(items))
	spinnerUrl := buildSpinnerURL(a.hostIP, items, winnerIndex)

	if os.Getenv("DELETE_WINNER") == "true" {
		filePath := os.Getenv("WHEEL_ITEMS_CONTAINER_PATH")

		var remainingItems []string
		for i, item := range items {
			if i != winnerIndex {
				remainingItems = append(remainingItems, item)
			}
		}

		err := os.WriteFile(filePath, []byte(strings.Join(remainingItems, "\n")), 0644)
		if err != nil {
			log.Printf("Failed to delete winning item from file: %v", err)
		} else {
			log.Printf("Winner deleted from file successfully.")
		}
	}

	a.executeObsSpin(spinnerUrl)
	w.WriteHeader(http.StatusOK)
}

func (a *App) handleSpinnerComplete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		Winner string `json:"winner"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	a.sendToChat(fmt.Sprintf("The wheel has spoken! Winner: %s", payload.Winner))

	go func() {
		time.Sleep(2 * time.Second)
		a.hideObsSpinner()
	}()

	w.WriteHeader(http.StatusOK)
}

func (a *App) sendToChat(message string) {
	err := pubsub.PublishJSON(a.rabbitChan, pubsub.ExchangeBot, pubsub.KeyChatMessage, models.ChatMessage{Message: message, User: "bot"})
	if err != nil {
		log.Printf("Failed to send message to chat: %v", err)
	}
}
