package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func ParseQuoteCommand(args string) (ParsedQuoteCommand, error) {
	args = strings.TrimSpace(args)

	if strings.HasPrefix(args, "--add") {
		addStr := strings.TrimSpace(strings.TrimPrefix(args, "--add"))
		return parseAddCommand(addStr)
	}

	if id, err := strconv.Atoi(args); err == nil {
		if id == 0 {
			return ParsedQuoteCommand{Action: ActionGetMostRecent}, nil
		}
		return ParsedQuoteCommand{Action: ActionGetByID, QuoteID: id}, nil
	}

	return parseFetchFilters(args)
}

func parseAddCommand(addStr string) (ParsedQuoteCommand, error) {
	cmd := ParsedQuoteCommand{Action: ActionAdd}
	re := regexp.MustCompile(`^"([^"]+)"(?:\s*-\s*([^,]+))?(?:,\s*([^,]+))?(?:,\s*(.+))?$`)

	matches := re.FindStringSubmatch(addStr)
	if len(matches) == 0 {
		return cmd, fmt.Errorf(`invalid add format. Expected: "quote" - user, game, year`)
	}

	cmd.QuoteText = strings.TrimSpace(matches[1])
	cmd.Who = strings.TrimSpace(matches[2])
	cmd.Game = strings.TrimSpace(matches[3])

	return cmd, nil
}

func parseFetchFilters(args string) (ParsedQuoteCommand, error) {
	cmd := ParsedQuoteCommand{Limit: 1}

	if args == "" {
		cmd.Action = ActionGetRandom
		return cmd, nil
	}

	re := regexp.MustCompile(`(?i)--(user|who|game|date|limit)\s+(?:"([^"]+)"|([^\s]+))`)
	matches := re.FindAllStringSubmatch(args, -1)

	var activeFilters int

	for _, match := range matches {
		key := strings.ToLower(match[1])
		value := match[2]
		if value == "" {
			value = match[3]
		}

		switch key {
		case "who", "user":
			cmd.Who = strings.TrimPrefix(value, "@")
			activeFilters++
		case "game":
			cmd.Game = value
			activeFilters++
		case "limit":
			if l, err := strconv.Atoi(value); err == nil && l > 0 {
				cmd.Limit = l
			}
		case "date":
			start, end, err := parseDateRange(value)
			if err != nil {
				return cmd, fmt.Errorf("invalid date format: %w", err)
			}
			cmd.HasDate = true
			cmd.StartDate = start
			cmd.EndDate = end
			activeFilters++
		}
	}

	cmd.QuoteText = strings.TrimSpace(re.ReplaceAllString(args, ""))
	if cmd.QuoteText != "" {
		activeFilters++
	}

	switch activeFilters {
	case 0:
		cmd.Action = ActionGetRandom
	case 1:
		switch {
		case cmd.Who != "":
			cmd.Action = ActionGetByWho
		case cmd.Game != "":
			cmd.Action = ActionGetByGame
		case cmd.HasDate:
			cmd.Action = ActionGetByDate
		default:
			cmd.Action = ActionGetByFilters
		}
	default:
		cmd.Action = ActionGetByFilters
	}

	return cmd, nil
}

func parseDateRange(input string) (time.Time, time.Time, error) {
	input = strings.TrimSpace(input)
	var left, right string

	lowerInput := strings.ToLower(input)
	if strings.Contains(input, " - ") {
		parts := strings.SplitN(input, " - ", 2)
		left, right = strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	} else if strings.Contains(lowerInput, " to ") {
		parts := strings.SplitN(lowerInput, " to ", 2)
		left, right = strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	} else {
		return parseSingleBounds(input)
	}

	startBounds, _, err := parseSingleBounds(left)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	_, endBounds, err := parseSingleBounds(right)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	return startBounds, endBounds, nil
}

func parseSingleBounds(input string) (time.Time, time.Time, error) {
	clean := strings.ReplaceAll(input, "/", "-")
	clean = strings.ReplaceAll(clean, ".", "-")
	clean = strings.ReplaceAll(clean, " ", "-")

	yearLayouts := []string{"2006"}
	monthLayouts := []string{"2006-01", "01-2006", "Jan-2006", "January-2006", "01-06", "Jan-06"}
	dayLayouts := []string{"2006-01-02", "01-02-2006", "02-01-2006", "Jan-02-2006", "02-Jan-2006", "01-02-06", "2006-Jan-02"}

	for _, layout := range yearLayouts {
		if t, err := time.Parse(layout, clean); err == nil {
			start := time.Date(t.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
			return start, start.AddDate(1, 0, -1), nil
		}
	}
	for _, layout := range monthLayouts {
		if t, err := time.Parse(layout, clean); err == nil {
			start := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
			return start, start.AddDate(0, 1, -1), nil
		}
	}
	for _, layout := range dayLayouts {
		if t, err := time.Parse(layout, clean); err == nil {
			return t, t, nil
		}
	}

	return time.Time{}, time.Time{}, fmt.Errorf("unrecognized date format: %s", input)
}
