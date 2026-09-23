package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/database"
	"github.com/Heros-Tempus/My-Twitch-Bot/internal/models"
	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
)

func (app *App) SendToChat(text string) {
	msg := models.ChatMessage{
		Message: text,
		User:    "Bot",
	}
	pubsub.PublishJSON(app.RabbitChann, pubsub.ExchangeBot, pubsub.KeyChatMessage, msg)
	log.Printf("PUBLISHED TO CHAT: %s", text)
}

func (app *App) sendFormattedQuote(id int32, text string, who, game sql.NullString, date sql.NullTime) {
	app.SendToChat(fmt.Sprintf(`Quote %d: "%s" - %s [%s, %s]`, id, text, who.String, game.String, date.Time.Format("2006-01-02")))
}

func (app *App) handleFetchError(err error, contextMsg string) pubsub.AckType {
	log.Printf("ERROR: Failed to fetch %s: %v", contextMsg, err)
	app.SendToChat(fmt.Sprintf("Failed to fetch %s.", contextMsg))
	return pubsub.AckTypeNackDiscard
}

func (app *App) HandleAuthMessage(msg models.OAuthToken) pubsub.AckType {
	app.TokenCache.Update(msg)
	log.Println("INFO: Successfully updated Twitch API token cache.")
	return pubsub.AckTypeAck
}

func (app *App) HandleCommandMessage(cmd models.Command) pubsub.AckType {
	ctx := context.Background()

	parsedCmd, err := ParseQuoteCommand(cmd.Args)
	if err != nil {
		log.Printf("WARN: Parse error: %v", err)
		app.SendToChat(fmt.Sprintf("@%s, %v", cmd.User, err))
		return pubsub.AckTypeNackDiscard
	}

	if parsedCmd.Action == ActionHelp {
		app.SendToChat("Usage: !quote <quote_id> or !quote --add <text> or !quote [search text]")
		app.SendToChat("Read: '!quote' (random), '!quote 0' (most recent), or '!quote <id>' (specific).")
		app.SendToChat("Add: '!quote --add \"Text\" --who @user --game name' (Game is auto-fetched if omitted).")
		app.SendToChat("Search: '!quote [search text] [--who @user] [--game name] [--date YYYY or YYYY to YYYY] [--limit X]'.")
		return pubsub.AckTypeAck
	}

	if parsedCmd.Action == ActionAdd {
		return app.handleAddQuote(ctx, cmd, parsedCmd)
	}

	return app.handleFetchQuotes(ctx, parsedCmd)
}

func (app *App) handleAddQuote(ctx context.Context, cmd models.Command, parsedCmd ParsedQuoteCommand) pubsub.AckType {
	if parsedCmd.Game == "" {
		token, clientID := app.TokenCache.Get()
		if token == "" || clientID == "" {
			app.SendToChat(fmt.Sprintf("@%s, I can't add that right now. Missing Twitch API token.", cmd.User))
			return pubsub.AckTypeNackDiscard
		}

		fetchedGame, err := GetCurrentGame(app.OwnerID, clientID, token)
		if err != nil {
			log.Printf("ERROR: Failed to fetch game from Twitch: %v", err)
			parsedCmd.Game = "Unknown"
		} else {
			parsedCmd.Game = fetchedGame
		}
	}

	err := app.DB.AddQuote(ctx, database.AddQuoteParams{
		Quote: parsedCmd.QuoteText,
		Who:   nullString(parsedCmd.Who),
		Game:  nullString(parsedCmd.Game),
	})
	if err != nil {
		log.Printf("ERROR: Failed to add quote to database: %v", err)
		app.SendToChat("Failed to save the quote to the database.")
		return pubsub.AckTypeNackDiscard
	}

	successMsg := fmt.Sprintf(`Successfully added quote: "%s" - %s [%s, %s]`, parsedCmd.QuoteText, parsedCmd.Who, parsedCmd.Game, time.Now().Format("2006-01-02"))
	app.SendToChat(successMsg)
	return pubsub.AckTypeAck
}

func (app *App) handleFetchQuotes(ctx context.Context, parsedCmd ParsedQuoteCommand) pubsub.AckType {
	switch parsedCmd.Action {
	case ActionGetMostRecent:
		quote, err := app.DB.GetMostRecentQuote(ctx)
		if err != nil {
			return app.handleFetchError(err, "most recent quote")
		}
		app.sendFormattedQuote(quote.ID, quote.Quote, quote.Who, quote.Game, quote.QuoteDate)

	case ActionGetByID:
		quote, err := app.DB.GetQuote(ctx, int32(parsedCmd.QuoteID))
		if err != nil {
			return app.handleFetchError(err, "quote by ID")
		}
		app.sendFormattedQuote(quote.ID, quote.Quote, quote.Who, quote.Game, quote.QuoteDate)

	case ActionGetRandom:
		quote, err := app.DB.RandomQuote(ctx)
		if err != nil {
			return app.handleFetchError(err, "random quote")
		}
		app.sendFormattedQuote(quote.ID, quote.Quote, quote.Who, quote.Game, quote.QuoteDate)

	case ActionGetByGame:
		log.Printf("INFO: Routing to ActionGetByGame for %s", parsedCmd.Game)
		quote, err := app.DB.GetRandomQuoteByGame(ctx, nullString(parsedCmd.Game))
		if err != nil {
			return app.handleFetchError(err, "quote by game")
		}
		app.sendFormattedQuote(quote.ID, quote.Quote, quote.Who, quote.Game, quote.QuoteDate)

	case ActionGetByWho:
		log.Printf("INFO: Routing to ActionGetByWho for %s", parsedCmd.Who)
		quote, err := app.DB.GetRandomQuoteByWho(ctx, nullString(parsedCmd.Who))
		if err != nil {
			return app.handleFetchError(err, "quote by who")
		}
		app.sendFormattedQuote(quote.ID, quote.Quote, quote.Who, quote.Game, quote.QuoteDate)

	case ActionGetByDate:
		log.Printf("INFO: Routing to ActionGetByDate for %s to %s", parsedCmd.StartDate, parsedCmd.EndDate)
		quote, err := app.DB.GetRandomQuoteByDate(ctx, nullTime(parsedCmd.StartDate))
		if err != nil {
			return app.handleFetchError(err, "quote by date")
		}
		app.sendFormattedQuote(quote.ID, quote.Quote, quote.Who, quote.Game, quote.QuoteDate)

	case ActionGetByFilters:
		quotes, err := app.DB.GetRandomQuotesByFilters(ctx, database.GetRandomQuotesByFiltersParams{
			Quote:      nullString(parsedCmd.QuoteText),
			Who:        nullString(parsedCmd.Who),
			Game:       nullString(parsedCmd.Game),
			StartDate:  nullTime(parsedCmd.StartDate),
			EndDate:    nullTime(parsedCmd.EndDate),
			LimitCount: nullInt32(parsedCmd.Limit),
		})
		if err != nil {
			return app.handleFetchError(err, "quotes by filters")
		}
		for _, quote := range quotes {
			app.sendFormattedQuote(quote.ID, quote.Quote, quote.Who, quote.Game, quote.QuoteDate)
		}
	}
	return pubsub.AckTypeAck
}
