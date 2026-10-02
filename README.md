# Twitch Bot Microservices

A distributed, locally-hosted Twitch bot built in Go. This bot is composed of several specialized microservices that communicate via RabbitMQ. State is managed via a PostgreSQL database, and the bot interfaces directly with OBS Studio over WebSockets to control media playback and render chat overlays. 

It is designed to run via Docker Compose on a desktop machine, communicating with an OBS instance on a separate laptop over a peer-to-peer Tailscale connection. Though it could in principle run on the same machine as the OBS instance, or they could run on seperate machines without a peer-to-peer conection if done so on a local network.

---

## Architecture

The application is orchestrated via Docker Compose and uses multi-stage Docker builds to compile the Go binaries into lightweight Alpine Linux containers. Internally, services share common data structures (such as `OAuthToken`, `ChatMessage`, and `OverlayMessage`) via a centralized models package.

### Infrastructure
* **RabbitMQ**: The central message broker handling pub/sub communication across all services using the `rabbitmq:3-management-alpine` image. It routes traffic through a primary `bot_topic_exchange` and utilizes a `twitch.dlx` dead-letter exchange for unprocessable messages. The pub/sub wrapper includes an exponential backoff strategy for resilient broker connections.
* **PostgreSQL**: The persistent data store running the postgres:15-alpine image. It manages five primary tables: `auth`, `tracks`, `queue`, `current_playback`, and `quotes`.

### Microservices
* **Auth (`auth`)**: Acts as the local OAuth client, managing Twitch tokens. If no token exists, it spins up a web server on port `8081` to handle the authorization redirect. It proactively refreshes the token when it is within 10 minutes of expiring and broadcasts the update to all services.
* **Chat Listener (`chat-listener`)**: Connects to Twitch's EventSub WebSocket API to monitor chat. It acts as the central event router, parsing commands to dedicated queues and mapping Twitch chat metadata (colors, badges, emote fragments) into an internal `OverlayMessage` struct. It transparently handles Twitch edge server reconnections and triggers auth refreshes if its subscription is revoked.
* **Chat Writer (`chat-writer`)**: The sole egress point for standard text responses, posting outbound messages via the Twitch Helix HTTP API. If it encounters an expired token (HTTP 401), it safely requeues the outbound message and pauses execution until a new token is broadcast.
* **Quote (`quote`)**: Manages the saving and retrieving of stream quotes. When a user adds a quote without specifying the game, the service queries the Twitch Helix API to automatically attribute the quote to the broadcaster's current active category.
* **Song Queue Manager (`song-queue-manager`)**: Manages the internal state of the music queue, including peeking, shuffling, and clearing tracks. It safeguards the queue by capping multi-track additions to 10 songs for non-broadcasters, automatically disables tracks in the database if the player reports them as unplayable, and performs a graceful `StopAndWipe` transaction to clear the queue table upon shutdown.
* **Song Player (`song-player`)**: Uses the goobs library to directly control OBS WebSockets and embeds a local web server (port `8000`) serving a YouTube IFrame API frontend (`yt.html`). It dynamically toggles OBS audio ducking filters during playback, displays on-screen attribution text that automatically hides after 15 seconds, and skips dead tracks if the YouTube iframe fails to load.
* **Chat Overlay (`chat-overlay`)**: Embeds a local web server (port `8080`) serving the `chat.html` frontend. On boot, it caches 3rd-party emotes from 7TV, BTTV, and FFZ. It enriches incoming messages with official Twitch badges, replaces emote text with image URLs, and acts as a failsafe by broadcasting high-priority, on-screen alerts to OBS if the OAuth token fails.
* **Spinner (`spinner`)**: An out-of-band service serving an HTML5 canvas wheel on port `8082`. Triggered via an authenticated HTTP POST request to `/api/spin`, it reads wheel options from a local text file, toggles the OBS browser source to display the animation, announces the winner to chat, and optionally auto-deletes the winning entry from the source file.

---

## Setup & Installation

### 1. Prerequisites
* **Twitch Client & OAuth Tokens**: Required for the bot to sign in to Twitch. Client and Secret must be acquired by registering the bot in the Twitch Dev Console.
* **Docker & Docker Compose**: Required to build and run the microservices.
* **Goose**: Required to run the SQL migrations against the PostgreSQL database.
* **OBS Studio**: Must have obs-websocket enabled and configured.
* **Tailscale (Optional)**: If running the bot and OBS on separate networks.

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

# Twitch Authentication
BOT_ID="Your_Bot_Account's_ID"
OWNER_ID="Your_Account's_ID"
CLIENT_ID="Your_Client_ID"
CLIENT_SECRET="Your_Client_Secret"
OAUTH_KEY="Your_OAUTH_Key"
OAUTH_REFRESH_KEY="Your_OAUTH_Refresh_Key"
BROADCASTER="Your_Username"

# Goose Migrations
export GOOSE_DBSTRING="Your_Postgres_Connection_String"
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

# Spinner Configuration
SPINNER_HOTKEY="Your AHK Hotkey choice"
HOTKEY_API_SECRET='"A static key to help secure the hotkey API. Note the single quotes around the double quotes."'
OBS_SPINNER_SOURCE_NAME="The browser source name for the spinner"
WHEEL_FOLDER_HOST_PATH=/mnt/c/Users/Your username/path/to/your/items folder
WHEEL_ITEMS_CONTAINER_PATH=/app/data/items.txt
DELETE_WINNER="true_or_false"
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

## Tools & Utilities

* **Overlay Configurator (`chat_coonfig.py`)**: A Tkinter GUI tool that generates custom URL query parameters for the OBS chat browser source. It allows dynamic configuration of text color, font size, gradients, alpha fades, and chat flow direction (e.g., bottom-up or top-down) without altering the CSS.
* **Spinner Hotkey (`spinner-hotkey.ahk`)**: An AutoHotkey script that automatically parses the `.env` file for your `SPINNER_HOTKEY` and `HOTKEY_API_SECRET`, allowing you to natively trigger the spinner's HTTP API over the local network using a keyboard shortcut.
* **Makefile commands (`Makefile`)**: A collection of commands useful for managing database backups and seeding. `make track_dump` and `make quote_dump` create backups of the `track` and `quote` tables, `make track_seed` and `make quote_seed` load the backups into the `track` and `quote` tables, and `make track_audit` finds all the tracks which have been disabled in the `tracks` table and saves them to a csv.

---

## Usage & Commands

### Chat Overlay Effects
The Chat Overlay service supports dynamic text effects triggered by specific hashtags.
* `#upsidedown`: Flips the message text upside down.
* `#rainbow`: Applies a rainbow gradient to the text.
* `#shake`: Applies a shaking animation to the text.

### Song Player Commands
The Song Player service directs a browser source to open to a video url in the tracks table, and displays an attribution string from the associated data.
* `!gc` or `!gamechops`: Base command to queue music.
* Filters: `--track`, `--artist`, `--album`, `--source_media`, `--original_composer`, `--limit`
* `!gc --peek`: Shows the upcoming tracks in the queue.

**Moderator Only:**
* `!gc --skip`: Skips the currently playing track.
* `!gc --shuffle`: Shuffles the current queue.
* `!gc --clear`: Empties the entire queue.

* **Shorthand Flags**: You can use `-p` (peek), `-s` (skip), `-c` (clear), and `-sh` (shuffle) as alternatives to the full `--` commands.

### Quote Commands
* `!quote --add "Quote text here" - Who, Game`: Adds a new quote to the database. If the game is omitted, the bot will auto-fetch the active Twitch category.
* `!quote`: Retrieves a random quote.
* `!quote [ID]`: Retrieves a specific quote by its numerical ID.
* `!quote 0`: Retrieves the most recently added quote.
* **Filters**: `--user` (or `--who`), `--game`, `--limit`, `--date`. The `--date` filter accepts robust natural language formats (e.g., `2006`, `Jan-06`, or ranges like `2020` to `2023`). Use `!quote -h` for in-chat syntax help.
