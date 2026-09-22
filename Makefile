-include .env

.PHONY: track_audit track_dump quote_dump track_seed quote_seed

track_audit:
	@docker compose exec -T postgres psql "$(GOOSE_DBSTRING)" --csv -c "SELECT * FROM tracks WHERE enabled = false;" > disabled_tracks.csv

track_dump:
	@docker compose exec -T postgres pg_dump -O "$(GOOSE_DBSTRING)" -t tracks > tracks_dump.sql

quote_dump:
	@docker compose exec -T postgres pg_dump -O "$(GOOSE_DBSTRING)" -t quotes > quotes_dump.sql

track_seed:
	@docker compose exec -T postgres psql "$(GOOSE_DBSTRING)" < tracks_dump.sql

quote_seed:
	@docker compose exec -T postgres psql "$(GOOSE_DBSTRING)" < quotes_dump.sql