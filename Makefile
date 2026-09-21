-include .env

.PHONY: track_audit
track_audit:
	@docker compose exec -T postgres psql "$(GOOSE_DBSTRING)" --csv -c "SELECT * FROM tracks WHERE enabled = false;" > disabled_tracks.csv