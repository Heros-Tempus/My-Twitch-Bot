package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
)

func (a *App) handlePlayerStatusRequest(msg EmptySignal) pubsub.AckType {
	ctx := context.Background()
	log.Println("Received player status request")
	a.mu.Lock()
	defer a.mu.Unlock()

	playback, err := a.service.queries.GetCurrentPlayback(ctx)
	if err != nil {
		a.isIdle = true
		a.rabbit.SendPlayerStatus("idle", 0)
		log.Println("Sending player status: idle")
		return pubsub.AckTypeAck
	}

	elapsedSeconds := int32(time.Since(playback.StartedAt.Time).Seconds())
	remaining := playback.DurationSeconds.Int32 - elapsedSeconds

	if remaining <= 0 {
		a.isIdle = true
		go a.popAndPlayNextTrack(ctx)
		log.Println("Sending player status: idle")
		return pubsub.AckTypeAck
	}

	a.isIdle = false
	a.rabbit.SendPlayerStatus("playback", remaining)
	log.Printf("Sending player status: playback, remaining: %d", remaining)
	return pubsub.AckTypeAck
}

func (a *App) handlePlayerReady(msg EmptySignal) pubsub.AckType {
	a.popAndPlayNextTrack(context.Background())
	return pubsub.AckTypeAck
}

func (a *App) handleChatCommand(msg Request) pubsub.AckType {
	input := Request{User: msg.User, Name: msg.Name, Args: msg.Args}
	isMod := strings.Contains(os.Getenv("MODERATORS"), msg.User) || msg.User == os.Getenv("BROADCASTER")
	parsed := ParseCommand(input)
	ctx := context.Background()
	log.Printf("Parsed command from %s using args %s: %+v", msg.User, msg.Args, parsed)
	switch parsed.Action {
	case ActionHelp:
		a.rabbit.sendToChat("Usage: !gc <filters> or !gc <command>")
		a.rabbit.sendToChat("Available filters: --track, --artist, --album, --source_media, --original_composer, --limit")
		a.rabbit.sendToChat("Available command: --peek")
		a.rabbit.sendToChat("Mod-only commands: --skip, --shuffle, --clear")

	case ActionSkip:
		if !isMod {
			a.rabbit.sendToChat("Only mods can skip songs!")
			return pubsub.AckTypeAck
		}
		a.rabbit.Skip()

	case ActionClear:
		if !isMod {
			a.rabbit.sendToChat("Only mods can clear the queue!")
			return pubsub.AckTypeAck
		}
		a.service.queries.TruncateQueue(ctx)
		a.rabbit.Skip()

	case ActionShuffle:
		a.service.queries.ShuffleQueue(ctx)
		a.rabbit.sendToChat("Queue has been shuffled!")

	case ActionPeek:
		peekedTracks, err := a.service.queries.Peek(ctx)
		if err != nil {
			a.rabbit.sendToChat("Failed to peek at tracks.")
			return pubsub.AckTypeAck
		}
		var peekMessage []string
		for i, track := range peekedTracks {
			peekMessage = append(peekMessage, fmt.Sprintf("%d: %s - %s", i+1, track.Artist, track.Track))
		}
		if len(peekMessage) > 0 {
			a.rabbit.sendToChat(strings.Join(peekMessage, ", "))
		} else {
			a.rabbit.sendToChat("No tracks in queue.")
		}

	case ActionQueue:
		queuedTracks, err := a.service.queries.QueueRandomTracks(ctx, parsed.Params)
		if err != nil {
			a.rabbit.sendToChat("Failed to queue tracks.")
			return pubsub.AckTypeAck
		}

		a.rabbit.sendToChat(fmt.Sprintf("Queued %d tracks!", len(queuedTracks)))

		a.mu.Lock()
		idle := a.isIdle
		a.mu.Unlock()

		if idle {
			go a.popAndPlayNextTrack(ctx)
		}
	}
	return pubsub.AckTypeAck
}