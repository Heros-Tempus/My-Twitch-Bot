package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Heros-Tempus/My-Twitch-Bot/internal/database"
	"github.com/Heros-Tempus/My-Twitch-Bot/internal/pubsub"
)

func (app *App) SendToChat(text string) {
	msg := ChatMessage{
		Message: text,
		User:    "Bot",
	}

	pubsub.PublishJSON(app.Rabbit, "twitch", "twitch.chat.send", msg)
	log.Printf("PUBLISHED TO CHAT: %s", text)
}

func (app *App) HandleAuthMessage(msg Oauth) pubsub.AckType {
	app.TokenCache.Update(msg)
	log.Println("INFO: Successfully updated Twitch API token cache.")
	return pubsub.AckTypeAck
}

func (app *App) HandleCommandMessage(cmd Command) pubsub.AckType {
	ctx := context.Background()

	parsedCmd, err := ParseQuoteCommand(cmd.Args)
	if err != nil {
		log.Printf("WARN: Parse error: %v", err)
		app.SendToChat(fmt.Sprintf("@%s, %v", cmd.User, err))
		return pubsub.AckTypeNackDiscard
	}

	switch parsedCmd.Action {
	case ActionAdd:
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

		err = app.DB.AddQuote(ctx, database.AddQuoteParams{
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

	case ActionGetMostRecent:
		quote, err := app.DB.GetMostRecentQuote(ctx)
		if err != nil {
			log.Printf("ERROR: Failed to fetch most recent quote: %v", err)
			app.SendToChat("Failed to fetch most recent quote.")
			return pubsub.AckTypeNackDiscard
		}
		app.SendToChat(fmt.Sprintf(`Quote %d: "%s" - %s [%s, %s]`, quote.ID, quote.Quote, quote.Who.String, quote.Game.String, quote.QuoteDate.Time.Format("2006-01-02")))
	case ActionGetByID:
		quote, err := app.DB.GetQuote(ctx, int32(parsedCmd.QuoteID))
		if err != nil {
			log.Printf("ERROR: Failed to fetch quote by ID: %v", err)
			app.SendToChat("Failed to fetch quote by ID.")
			return pubsub.AckTypeNackDiscard
		}
		app.SendToChat(fmt.Sprintf(`Quote %d: "%s" - %s [%s, %s]`, quote.ID, quote.Quote, quote.Who.String, quote.Game.String, quote.QuoteDate.Time.Format("2006-01-02")))
	case ActionGetRandom:
		quote, err := app.DB.RandomQuote(ctx)
		if err != nil {
			log.Printf("ERROR: Failed to fetch random quote: %v", err)
			app.SendToChat("Failed to fetch random quote.")
			return pubsub.AckTypeNackDiscard
		}
		app.SendToChat(fmt.Sprintf(`Quote %d: "%s" - %s [%s, %s]`, quote.ID, quote.Quote, quote.Who.String, quote.Game.String, quote.QuoteDate.Time.Format("2006-01-02")))
	case ActionGetByGame:
		quote, err := app.DB.GetRandomQuoteByGame(ctx, nullString(parsedCmd.Game))
		if err != nil {
			log.Printf("ERROR: Failed to fetch quote by game: %v", err)
			app.SendToChat("Failed to fetch quote by game.")
			return pubsub.AckTypeNackDiscard
		}
		app.SendToChat(fmt.Sprintf(`Quote %d: "%s" - %s [%s, %s]`, quote.ID, quote.Quote, quote.Who.String, quote.Game.String, quote.QuoteDate.Time.Format("2006-01-02")))
		log.Printf("INFO: Routing to ActionGetByGame for %s", parsedCmd.Game)

	case ActionGetByWho:
		quote, err := app.DB.GetRandomQuoteByWho(ctx, nullString(parsedCmd.Who))
		if err != nil {
			log.Printf("ERROR: Failed to fetch quote by who: %v", err)
			app.SendToChat("Failed to fetch quote by who.")
			return pubsub.AckTypeNackDiscard
		}
		app.SendToChat(fmt.Sprintf(`Quote %d: "%s" - %s [%s, %s]`, quote.ID, quote.Quote, quote.Who.String, quote.Game.String, quote.QuoteDate.Time.Format("2006-01-02")))
		log.Printf("INFO: Routing to ActionGetByWho for %s", parsedCmd.Who)
	case ActionGetByDate:
		quote, err := app.DB.GetRandomQuoteByDate(ctx, nullTime(parsedCmd.StartDate))
		if err != nil {
			log.Printf("ERROR: Failed to fetch quote by date: %v", err)
			app.SendToChat("Failed to fetch quote by date.")
			return pubsub.AckTypeNackDiscard
		}
		app.SendToChat(fmt.Sprintf(`Quote %d: "%s" - %s [%s, %s]`, quote.ID, quote.Quote, quote.Who.String, quote.Game.String, quote.QuoteDate.Time.Format("2006-01-02")))
		log.Printf("INFO: Routing to ActionGetByDate for %s to %s", parsedCmd.StartDate, parsedCmd.EndDate)
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
			log.Printf("ERROR: Failed to fetch quotes by filters: %v", err)
			app.SendToChat("Failed to fetch quotes by filters.")
			return pubsub.AckTypeNackDiscard
		}
		for _, quote := range quotes {
			app.SendToChat(fmt.Sprintf(`Quote %d: "%s" - %s [%s, %s]`, quote.ID, quote.Quote, quote.Who.String, quote.Game.String, quote.QuoteDate.Time.Format("2006-01-02")))
		}
	}
	return pubsub.AckTypeAck
}
