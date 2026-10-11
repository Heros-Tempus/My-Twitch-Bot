-include .env

.PHONY: track_audit track_dump quote_dump track_seed quote_seed enable_video

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

enable_video:
	@if [ -z "$(ID)" ]; then \
		echo "Error: Please specify video ID(s) using ID=\"id1 id2\""; \
		exit 1; \
	fi
	@docker compose exec -T postgres psql "$(GOOSE_DBSTRING)" -c "UPDATE tracks SET enabled = true WHERE video_id = ANY(string_to_array('$(ID)', ' '));"