package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/database"
	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	_ = godotenv.Load(".env")

	dbConString := os.Getenv("POSTGRES_CON_STRING")
	log.Printf("PostgreSQL connection string: %s", dbConString)
	dbConn, err := sql.Open("postgres", dbConString)
	if err != nil {
		log.Fatal("Error connecting to PostgreSQL:", err)
	}
	defer func() {
		log.Println("Closing PostgreSQL connection...")
		dbConn.Close()
	}()
	log.Println("Connected to PostgreSQL")

	service := NewService(dbConn)

	rabbitConString := os.Getenv("RABBIT_CON_STRING")
	log.Printf("RabbitMQ connection string: %s", rabbitConString)

	con, err := pubsub.ConnectWithBackoff(rabbitConString, 5)
	if err != nil {
		log.Fatal("Error connecting to RabbitMQ:", err)
	}
	defer func() {
		log.Println("Closing RabbitMQ connection...")
		con.Close()
	}()

	ch, err := con.Channel()
	if err != nil {
		log.Fatal("Error opening RabbitMQ channel:", err)
	}
	defer func() {
		log.Println("Closing RabbitMQ channel...")
		ch.Close()
	}()

	rabbitClient := &RabbitClient{ch: ch}
	app := &App{
		rabbit:  rabbitClient,
		service: service,
		isIdle:  true,
	}

	err = setupRabbitMQSubscriptions(con, ch, app)
	if err != nil {
		log.Fatal("Error setting up RabbitMQ subscriptions:", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Println("Bot is running. Waiting for shutdown signal...")

	<-ctx.Done()

	log.Println("Shutdown signal received. Initiating graceful shutdown...")

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	log.Println("Executing StopAndWipe...")
	if err := app.service.StopAndWipe(shutdownCtx); err != nil {
		log.Printf("Failed to complete StopAndWipe during shutdown: %v", err)
	} else {
		log.Println("StopAndWipe completed successfully.")
	}
}

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

func (a *App) popAndPlayNextTrack(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()

	trackData, err := a.service.queries.PopNextTrack(ctx)
	if err != nil {
		a.isIdle = true
		_ = a.service.queries.ClearCurrentPlayback(ctx)
		a.rabbit.SendPlayerStatus("idle", 0)
		return
	}

	a.isIdle = false

	_ = a.service.queries.TrackCurrentPlayback(ctx, database.TrackCurrentPlaybackParams{
		VideoID:         sql.NullString{String: trackData.Url, Valid: true},
		DurationSeconds: trackData.DurationSeconds,
	})

	t := Track{
		Artist: trackData.Artist,
		Track:  trackData.Track,
	}
	if trackData.SourceMedia.Valid {
		t.SourceMedia = trackData.SourceMedia.String
	}
	if trackData.Album.Valid {
		t.Album = trackData.Album.String
	}
	if trackData.OriginalComposer.Valid {
		t.OriginalComposer = trackData.OriginalComposer.String
	}

	payload := PlayerTrackPayload{
		Url:               trackData.ID,
		Duration:          trackData.DurationSeconds.Int32,
		AttributionString: buildAttribution(t),
	}
	a.rabbit.SendNextTrack(payload)
	a.rabbit.sendToChat("Now playing: " + payload.AttributionString)
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

func ParseCommand(in Request) ParsedCommand {
	parts := strings.Split(in.Args, "--")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		key := strings.ToLower(strings.SplitN(part, " ", 2)[0])

		switch key {
		case "help":
			return ParsedCommand{Action: ActionHelp}
		case "skip":
			return ParsedCommand{Action: ActionSkip}
		case "clear":
			return ParsedCommand{Action: ActionClear}
		case "shuffle":
			return ParsedCommand{Action: ActionShuffle}
		case "peek":
			return ParsedCommand{Action: ActionPeek}
		}
	}

	params := database.QueueRandomTracksParams{}
	var limit int32 = 1

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		keyVal := strings.SplitN(part, " ", 2)
		key := strings.ToLower(keyVal[0])
		var val string
		if len(keyVal) == 2 {
			val = strings.TrimSpace(keyVal[1])
		}

		switch key {
		case "track":
			params.Track = sql.NullString{String: val, Valid: true}
		case "artist":
			params.Artist = sql.NullString{String: val, Valid: true}
		case "album":
			params.Album = sql.NullString{String: val, Valid: true}
		case "source_media":
			params.SourceMedia = sql.NullString{String: val, Valid: true}
		case "original_composer":
			params.OriginalComposer = sql.NullString{String: val, Valid: true}
		case "limit":
			if parsed, err := strconv.ParseInt(val, 10, 32); err == nil {
				limit = int32(parsed)
			}
		}
	}
	log.Printf("User: %s", in.User)
	log.Printf("Broadcaster: %s", os.Getenv("BROADCASTER"))
	log.Printf("User is broadcaster: %t", in.User == os.Getenv("BROADCASTER"))
	if limit > 10 && in.User != os.Getenv("BROADCASTER") {
		limit = 10
	}
	params.LimitCount = sql.NullInt32{Int32: limit, Valid: true}

	return ParsedCommand{Action: ActionQueue, Params: params}
}

func (s *Service) StopAndWipe(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	qtx := s.queries.WithTx(tx)

	if err := qtx.TruncateQueue(ctx); err != nil {
		return err
	}
	if err := qtx.ClearCurrentPlayback(ctx); err != nil {
		return err
	}

	return tx.Commit()
}

func buildAttribution(t Track) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("%s - %s", t.Artist, t.Track))

	if t.SourceMedia != "" {
		parts = append(parts, fmt.Sprintf("Source media: (%s)", t.SourceMedia))
	}
	if t.Album != "" {
		parts = append(parts, fmt.Sprintf("Album: [%s]", t.Album))
	}
	if t.OriginalComposer != "" {
		parts = append(parts, fmt.Sprintf("Composed by: %s", t.OriginalComposer))
	}

	return strings.Join(parts, "\n")
}
