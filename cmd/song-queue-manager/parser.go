package main

import (
	"database/sql"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/database"
	"github.com/Heros-Tempus/My-Twitch-Bot/internal/models"
)

func ParseCommand(in models.Command) ParsedCommand {
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
