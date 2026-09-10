# Twitch Bot Microservices

A distributed, locally-hosted Twitch bot built in Go. This bot is composed of several specialized microservices that communicate via RabbitMQ. State is managed via a PostgreSQL database, and the bot interfaces directly with OBS Studio over WebSockets to control media playback and render chat overlays. 

It is designed to run via Docker Compose on a desktop machine, communicating with an OBS instance on a separate laptop over a peer-to-peer Tailscale connection.

---

## Architecture

The application is orchestrated via Docker Compose and uses multi-stage Docker builds to compile the Go binaries into lightweight Alpine Linux containers. 

### Infrastructure
* **RabbitMQ**: The central message broker handling pub/sub communication across all services using the `rabbitmq:3-management-alpine` image.
* **PostgreSQL**: The persistent data store for authentication tokens, quotes, and the song queue using the `postgres:15-alpine` image.

### Microservices
* **Auth (`auth`)**: Manages Twitch OAuth tokens, proactively refreshes them, broadcasts them to other services via RabbitMQ, and persists them to the database.
* **Chat Listener (`chat-listener`)**: Subscribes to Twitch's EventSub WebSocket to monitor incoming chat messages and route commands.
* **Chat Writer (`chat-writer`)**: Handles outgoing HTTP requests to the Twitch API to post messages to the chat.
* **Quote (`quote`)**: Manages the saving and retrieving of stream quotes from the PostgreSQL database.
* **Song Queue Manager (`song-queue-manager`)**: Manages the internal state of the music queue, including peeking, shuffling, and clearing tracks.
* **Song Player (`song-player`)**: Embeds a local web server (exposed on port `8000`) to play YouTube videos and communicates with OBS to toggle source visibility and audio filters.
* **Chat Overlay (`chat-overlay`)**: Embeds a local web server (exposed on port `8080`) serving an HTML overlay, and pipes chat messages and 3rd-party emotes to OBS via WebSockets.

---

## Setup & Installation

### 1. Prerequisites
* **Twitch Client & OAuth Tokens**: Required for the bot to sign in to Twitch. Client and Secret must be acquired by registering the bot in the Twitch Dev Console. OAuth Token and Refresh Token can be acquired through Twitch CLI.
* **Docker & Docker Compose**: Required to build and run the microservices.
* **Goose**: Required to run the SQL migrations against the PostgreSQL database.
* **OBS Studio**: Must have obs-websocket enabled and configured.
* **Tailscale (Optional)**: If running the bot and OBS on separate machines.

### 2. Environment Variables
Create a `.env` file in the root directory. You can use the following template:

```env
# RabbitMQ Configuration
RABBITMQ_DEFAULT_USER="Your_Default_Rabbit_User"
RABBITMQ_DEFAULT_PASS="Your_Default_Rabbit_Pass"
RABBITMQ_USER="Your_Rabbit_User"
RABBITMQ_PASS="Your_Rabbit_Pass"

# PostgreSQL Configuration
POSTGRES_USER="Your_Postgres_User"
POSTGRES_PASSWORD="Your_Postgres_Pass"
POSTGRES_DB="Your_Postgres_DB"
POSTGRES_HOST="Your_Postgres_Host"

# Twitch Authentication
BOT_ID="Your_Bot_Account's_ID"
OWNER_ID="Your_Account's_ID"
CLIENT_ID="Your_Client_ID"
CLIENT_SECRET="Your_Client_Secret"
OAUTH_KEY="Your_OAUTH_Key"
OAUTH_REFRESH_KEY="Your_OAUTH_Refresh_Key"
BROADCASTER="Your_Username"

# Goose Migrations
POSTGRES_CON_STRING=postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}/${POSTGRES_DB}?sslmode=disable
export GOOSE_DBSTRING=${POSTGRES_CON_STRING}
export GOOSE_MIGRATION_DIR=./sql/schema
export GOOSE_DRIVER=postgres

# OBS Configuration
OBS_IP="Your_OBS_IP"
OBS_PORT="Your_OBS_Port (it usually defaults to 4455)"
OBS_PASSWORD="Your_OBS_Password"
OBS_BROWSER_SOURCE_NAME="OBS_Browser_Source_Name_For_The_Song_Player"
OBS_TEXT_SOURCE_NAME="OBS_Text_Source_Name_For_The_Song_Player's_Attribution_String"
OBS_SCENE_NAME="Your_OBS_Scene_Name"

# Network Configuration
DESKTOP_IP="IP_Address_for_the_machine_running_the_bot"
TZ="Your_Timezone"
```

### 3. Database Initialization
Before starting the Go services, you must start the database and run the schema migrations.

1. Start the infrastructure:
   `bash
   docker compose up -d postgres rabbitmq
   `
2. Run the Goose migrations to build the tables (`auth`, `tracks`, `queue`, `current_playback`, `quotes`):
   `bash
   source .env
   goose up
   `
3. (Optional) Seed the `tracks` table: If you'd like to quickly test the music player without manually adding songs, I've created a [database dump gist](https://gist.github.com/Heros-Tempus/0bf3e961f3ef2075bbd24c1b56edd78f) pre-populated with links and metadata scraped from the GameChops YouTube channel. You can execute this SQL dump against your newly migrated database to fill the `tracks` table.

### 4. Running the Bot
Once the database is primed, start the remaining microservices:
`bash
docker compose up -d
`

---

## Usage & Commands

### Chat Overlay Effects
The Chat Overlay service supports dynamic text effects triggered by specific hashtags.
* `#upsidedown`: Flips the message text upside down.
* `#rainbow`: Applies a rainbow gradient to the text.
* `#shake`: Applies a shaking animation to the text.

### Song Player Commands
The Song Player service directs a browser source to open to a video url in the tracks table, and displays an attribution string based associated data.
* `!gc` or `!gamechops`: Base command to queue music.
* Filters: `--track`, `--artist`, `--album`, `--source_media`, `--original_composer`, `--limit`
* `!gc --peek`: Shows the upcoming tracks in the queue.

**Moderator Only:**
* `!gc --skip`: Skips the currently playing track.
* `!gc --shuffle`: Shuffles the current queue.
* `!gc --clear`: Empties the entire queue.

### Quote Commands
* `!quote --add "Quote text here" - Who, Game`: Adds a new quote to the database.
* `!quote`: Retrieves a random quote.
* `!quote [ID]`: Retrieves a specific quote by its numerical ID.
* `!quote 0`: Retrieves the most recently added quote.
* Filters: `--user` (or `--who`), `--game`, `--date`.
