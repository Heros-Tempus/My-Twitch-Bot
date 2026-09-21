-include .env

.PHONY: track_audit track_dump quote_dump

track_audit:
	@docker compose exec -T postgres psql "$(GOOSE_DBSTRING)" --csv -c "SELECT * FROM tracks WHERE enabled = false;" > disabled_tracks.csv

track_dump:
	@docker compose exec -T postgres pg_dump -O "$(GOOSE_DBSTRING)" -t tracks > tracks_dump.sql

quote_dump:
	@docker compose exec -T postgres pg_dump -O "$(GOOSE_DBSTRING)" -t quotes > quotes_dump.sql