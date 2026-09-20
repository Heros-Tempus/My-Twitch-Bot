package pubsub

type ExchangeName = string
type QueueName = string
type RoutingKey = string

const ExchangeBot ExchangeName = "bot_topic_exchange"

const (
	// Auth Service
	QueueAuthRequests QueueName = "q_auth_requests"

	// Chat Listener
	QueueListenerOAuth QueueName = "q_chat_listener_oauth"

	// Chat Overlay
	QueueOverlayOAuth    QueueName = "q_chat_overlay_oauth"
	QueueOverlayMessages QueueName = "q_chat_overlay_messages"
	QueueOverlayAlerts   QueueName = "q_chat_overlay_alerts"

	// Chat Writer
	QueueWriterOAuth    QueueName = "q_chat_writer_oauth"
	QueueWriterOutbound QueueName = "q_chat_writer_outbound"

	// Quote Service
	QueueQuoteOAuth    QueueName = "q_quote_oauth"
	QueueQuoteCommands QueueName = "q_quote_commands"

	// Song Manager
	QueueSongManagerCommands QueueName = "q_song_manager_commands"
	QueueSongManagerSkip     QueueName = "q_song_manager_skip"
	QueueSongManagerStatus   QueueName = "q_song_manager_status"
	QueueSongReady           QueueName = "q_song_ready"

	// Song Player
	QueueSongPlayerSong QueueName = "q_song_player_song"
	QueueSongStatusResp QueueName = "q_song_status_response"
	QueueSongDisable    QueueName = "q_song_disable"
)

// --- Routing Keys ---
const (
	// Auth Flow
	KeyTokenRefreshed  RoutingKey = "auth.token.refreshed"
	KeyTokenRefreshReq RoutingKey = "auth.token.request_refresh"
	KeyTokenFailed     RoutingKey = "auth.token.failed"

	// Chat Flow
	KeyChatMessage    RoutingKey = "chat.message.plain"
	KeyChatOverlay    RoutingKey = "chat.message.overlay"
	KeyChatWriteAttr  RoutingKey = "chat.write.attribution"
	KeyChatWriteQuote RoutingKey = "chat.write.quote"

	// Commands
	KeyCmdSong  RoutingKey = "chat.command.song"
	KeyCmdQuote RoutingKey = "chat.command.quote"

	// Song Player Controls
	KeySongPlay        RoutingKey = "song.player.play"
	KeySongSkip        RoutingKey = "song.player.skip"
	KeySongStatusReply RoutingKey = "song.player.status_reply"

	// Song Manager Events
	KeySongReady     RoutingKey = "song.manager.ready"
	KeySongStatusReq RoutingKey = "song.manager.status_req"
	KeySongDisable   RoutingKey = "song.manager.disable"
)

type SimpleQueueType string

const (
	SimpleQueueTypeDurable   SimpleQueueType = "durable"
	SimpleQueueTypeTransient SimpleQueueType = "transient"
)

type Queue struct {
	Type SimpleQueueType
	Name string
}

type AckType string

const (
	AckTypeAck         AckType = "ack"
	AckTypeNackRequeue AckType = "nack_requeue"
	AckTypeNackDiscard AckType = "nack_discard"
)

type Ack struct {
	Type AckType
	Name string
}
