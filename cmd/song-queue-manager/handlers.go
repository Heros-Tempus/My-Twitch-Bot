package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/database"
	"github.com/Heros-Tempus/My-Twitch-Bot/internal/models"
	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
)

func (a *App) handlePlayerStatusRequest(msg models.EmptySignal) pubsub.AckType {
	ctx := context.Background()
	log.Println("Received player status request")
	a.mu.Lock()
	defer a.mu.Unlock()
	playback, err := a.service.queries.GetCurrentPlayback(ctx)
	if err != nil {
		a.isIdle = true
		a.SendPlayerStatus("idle", 0)
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
	a.SendPlayerStatus("playback", remaining)
	log.Printf("Sending player status: playback, remaining: %d", remaining)
	return pubsub.AckTypeAck
}

func (a *App) handlePlayerReady(msg models.EmptySignal) pubsub.AckType {
	a.popAndPlayNextTrack(context.Background())
	return pubsub.AckTypeAck
}

func (a *App) handleChatCommand(msg models.Command) pubsub.AckType {
	isMod := strings.Contains(os.Getenv("MODERATORS"), msg.User) || msg.User == os.Getenv("BROADCASTER")
	parsed := ParseCommand(models.Command{User: msg.User, Name: msg.Name, Args: msg.Args})
	ctx := context.Background()

	log.Printf("Parsed command from %s using args %s: %+v", msg.User, msg.Args, parsed)

	switch parsed.Action {
	case ActionHelp:
		return a.handleHelp()
	case ActionSkip:
		return a.handleSkipCmd(isMod)
	case ActionClear:
		return a.handleClearCmd(ctx, isMod)
	case ActionShuffle:
		return a.handleShuffleCmd(ctx)
	case ActionPeek:
		return a.handlePeekCmd(ctx)
	case ActionQueue:
		return a.handleQueueCmd(ctx, parsed.Params)
	}

	return pubsub.AckTypeAck
}

func (a *App) handleHelp() pubsub.AckType {
	a.sendToChat("Usage: !gc <filters> or !gc <command>")
	a.sendToChat("Available filters: --track, --artist, --album, --source_media, --original_composer, --limit")
	a.sendToChat("Available command: --peek")
	a.sendToChat("Mod-only commands: --skip, --shuffle, --clear")
	return pubsub.AckTypeAck
}

func (a *App) handleSkipCmd(isMod bool) pubsub.AckType {
	if !isMod {
		a.sendToChat("Only mods can skip songs!")
		return pubsub.AckTypeAck
	}
	a.Skip()
	return pubsub.AckTypeAck
}

func (a *App) handleClearCmd(ctx context.Context, isMod bool) pubsub.AckType {
	if !isMod {
		a.sendToChat("Only mods can clear the queue!")
		return pubsub.AckTypeAck
	}
	a.service.queries.TruncateQueue(ctx)
	a.Skip()
	return pubsub.AckTypeAck
}

func (a *App) handleShuffleCmd(ctx context.Context) pubsub.AckType {
	a.service.queries.ShuffleQueue(ctx)
	a.sendToChat("Queue has been shuffled!")
	return pubsub.AckTypeAck
}

func (a *App) handlePeekCmd(ctx context.Context) pubsub.AckType {
	peekedTracks, err := a.service.queries.Peek(ctx)
	if err != nil {
		a.sendToChat("Failed to peek at tracks.")
		return pubsub.AckTypeAck
	}

	if len(peekedTracks) == 0 {
		a.sendToChat("No tracks in queue.")
		return pubsub.AckTypeAck
	}

	var peekMessage []string
	for i, track := range peekedTracks {
		peekMessage = append(peekMessage, fmt.Sprintf("%d: %s - %s", i+1, track.Artist, track.Track))
	}
	a.sendToChat(strings.Join(peekMessage, ", "))

	return pubsub.AckTypeAck
}

func (a *App) handleQueueCmd(ctx context.Context, params database.QueueRandomTracksParams) pubsub.AckType {
	queuedTracks, err := a.service.queries.QueueRandomTracks(ctx, params)
	if err != nil {
		a.sendToChat("Failed to queue tracks.")
		return pubsub.AckTypeAck
	}

	a.sendToChat(fmt.Sprintf("Queued %d tracks!", len(queuedTracks)))

	a.mu.Lock()
	idle := a.isIdle
	a.mu.Unlock()

	if idle {
		go a.popAndPlayNextTrack(ctx)
	}
	return pubsub.AckTypeAck
}

func (a *App) handleDisableTrack(msg models.TrackDisablePayload) pubsub.AckType {
	ctx := context.Background()
	log.Printf("Received disable signal from player for track %s", msg.VideoID)
	
	err := a.service.queries.DisableTrack(ctx, msg.VideoID)
	if err != nil {
		log.Printf("Failed to disable track in DB: %v", err)
	} else {
		log.Printf("Successfully disabled track %s in database.", msg.VideoID)
	}
	
	return pubsub.AckTypeAck
}