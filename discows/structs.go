package discows

import (
	"encoding/json"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type Session struct {
	sync.RWMutex

	wsMutex    sync.Mutex
	wsConn     *websocket.Conn
	gatewayURL string

	messagesChan chan interface{}

	heartbeatChan     chan interface{}
	heartbeatInterval time.Duration
	// lastHeartbeatSent time.Time
	// lastHeartbeatReceived time.Time

	SessionID            string
	AnalyticsToken       string
	lastSequenceReceived atomic.Int64

	Token string

	EventManager EventManager
	Cache        ClientCache
}

type resumePacketData struct {
	Token     string `json:"token"`
	SessionID string `json:"session_id"`
	Seq       int    `json:"seq"`
}

type PresenceActivity struct {
	Name  string `json:"name"`
	Type  int    `json:"type"`
	State string `json:"state"`
	Emoji struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Animated bool   `json:"animated"`
	} `json:"emoji"`
}

type GuildProperties struct {
	Name     string   `json:"name"`
	Features []string `json:"features"`
}

type Guild struct {
	Properties           GuildProperties `json:"properties,omitempty"`
	ID                   string          `json:"id"`
	MemberCount          int             `json:"member_count"`
	Channels             []Channel       `json:"channels"`
	Roles                []interface{}   `json:"roles"`
	Emojis               []interface{}   `json:"emojis"`
	Threads              []interface{}   `json:"threads"`
	Stickers             []interface{}   `json:"stickers"`
	GuildScheduledEvents []interface{}   `json:"guild_scheduled_events"`
	Large                bool            `json:"large,omitempty"`
}

type User struct {
	Username      string `json:"username"`
	PublicFlags   int    `json:"public_flags,omitempty"`
	Flags         int    `json:"flags,omitempty"`
	ID            string `json:"id"`
	GlobalName    string `json:"global_name,omitempty"`
	Discriminator string `json:"discriminator,omitempty"`
	Bot           bool   `json:"bot,omitempty"`
}

func (user *User) String() string {
	if len(user.Discriminator) > 1 {
		return user.Username + "#" + user.Discriminator
	}

	return user.Username
}

type HelloMessage struct {
	HeartbeatInterval time.Duration `json:"heartbeat_interval"`
}

type Ready struct {
	User             User    `json:"user"`
	SessionID        string  `json:"session_id"`
	AnalyticsToken   string  `json:"analytics_token"`
	ResumeGatewayURL string  `json:"resume_gateway_url"`
	Guilds           []Guild `json:"guilds"`
	Sessions         []struct {
		Status     string             `json:"status"`
		SessionID  string             `json:"session_id"`
		Activities []PresenceActivity `json:"activities"`
		Active     bool               `json:"active,omitempty"`
	} `json:"sessions"`
	Experiments       []interface{}   `json:"experiments"`
	UserGuildSettings json.RawMessage `json:"user_guild_settings"`
	Relationships     []interface{}   `json:"relationships"`
	Users             []interface{}   `json:"users"`
}

// PermissionOverwriteType represents the type of resource on which
// a permission overwrite acts.
type PermissionOverwriteType int

// The possible permission overwrite types.
const (
	PermissionOverwriteTypeRole   PermissionOverwriteType = 0
	PermissionOverwriteTypeMember PermissionOverwriteType = 1
)

// A PermissionOverwrite holds permission overwrite data for a Channel
type PermissionOverwrite struct {
	ID    string                  `json:"id"`
	Type  PermissionOverwriteType `json:"type"`
	Deny  int64                   `json:"deny,string"`
	Allow int64                   `json:"allow,string"`
}

// ChannelType is the type of a Channel
type ChannelType int

// Block contains known ChannelType values
const (
	ChannelTypeGuildText          ChannelType = 0
	ChannelTypeDM                 ChannelType = 1
	ChannelTypeGuildVoice         ChannelType = 2
	ChannelTypeGroupDM            ChannelType = 3
	ChannelTypeGuildCategory      ChannelType = 4
	ChannelTypeGuildNews          ChannelType = 5
	ChannelTypeGuildStore         ChannelType = 6
	ChannelTypeGuildNewsThread    ChannelType = 10
	ChannelTypeGuildPublicThread  ChannelType = 11
	ChannelTypeGuildPrivateThread ChannelType = 12
	ChannelTypeGuildStageVoice    ChannelType = 13
	ChannelTypeGuildForum         ChannelType = 15
)

// ChannelFlags represent flags of a channel/thread.
type ChannelFlags int

// Block containing known ChannelFlags values.
const (
	// ChannelFlagPinned indicates whether the thread is pinned in the forum channel.
	// NOTE: forum threads only.
	ChannelFlagPinned ChannelFlags = 1 << 1
	// ChannelFlagRequireTag indicates whether a tag is required to be specified when creating a thread.
	// NOTE: forum channels only.
	ChannelFlagRequireTag ChannelFlags = 1 << 4
)

type Channel struct {
	// The ID of the channel.
	ID string `json:"id"`

	// The ID of the guild to which the channel belongs, if it is in a guild.
	// Else, this ID is empty (e.g. DM channels).
	GuildID string `json:"guild_id"`

	// The name of the channel.
	Name string `json:"name"`

	// The topic of the channel.
	Topic string `json:"topic"`

	// The type of the channel.
	Type ChannelType `json:"type"`

	// The ID of the last message sent in the channel. This is not
	// guaranteed to be an ID of a valid message.
	LastMessageID string `json:"last_message_id"`

	// The timestamp of the last pinned message in the channel.
	// nil if the channel has no pinned messages.
	LastPinTimestamp *time.Time `json:"last_pin_timestamp"`

	// An approximate count of messages in a thread, stops counting at 50
	MessageCount int `json:"message_count"`
	// An approximate count of users in a thread, stops counting at 50
	MemberCount int `json:"member_count"`

	// Whether the channel is marked as NSFW.
	NSFW bool `json:"nsfw"`

	// Icon of the group DM channel.
	Icon string `json:"icon"`

	// The position of the channel, used for sorting in client.
	Position int `json:"position"`

	// The bitrate of the channel, if it is a voice channel.
	Bitrate int `json:"bitrate"`

	// The recipients of the channel. This is only populated in DM channels.
	Recipients []*User `json:"recipients"`

	// The messages in the channel. This is only present in state-cached channels,
	// and State.MaxMessageCount must be non-zero.
	Messages []*Message `json:"-"`

	// A list of permission overwrites present for the channel.
	PermissionOverwrites []*PermissionOverwrite `json:"permission_overwrites"`

	// The user limit of the voice channel.
	UserLimit int `json:"user_limit"`

	// The ID of the parent channel, if the channel is under a category. For threads - id of the channel thread was created in.
	ParentID string `json:"parent_id"`

	// Amount of seconds a user has to wait before sending another message or creating another thread (0-21600)
	// bots, as well as users with the permission manage_messages or manage_channel, are unaffected
	RateLimitPerUser int `json:"rate_limit_per_user"`

	// ID of the creator of the group DM or thread
	OwnerID string `json:"owner_id"`

	// ApplicationID of the DM creator Zeroed if guild channel or not a bot user
	ApplicationID string `json:"application_id"`

	// Channel flags.
	Flags ChannelFlags `json:"flags"`
}

// Emoji struct holds data related to Emoji's
type Emoji struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Roles         []string `json:"roles"`
	User          *User    `json:"user"`
	RequireColons bool     `json:"require_colons"`
	Managed       bool     `json:"managed"`
	Animated      bool     `json:"animated"`
	Available     bool     `json:"available"`
}

// EmojiRegex is the regex used to find and identify emojis in messages
var (
	EmojiRegex = regexp.MustCompile(`<(a|):[A-z0-9_~]+:[0-9]{18,20}>`)
)

// MessageFormat returns a correctly formatted Emoji for use in Message content and embeds
func (e *Emoji) MessageFormat() string {
	if e.ID != "" && e.Name != "" {
		if e.Animated {
			return "<a:" + e.APIName() + ">"
		}

		return "<:" + e.APIName() + ">"
	}

	return e.APIName()
}

// APIName returns an correctly formatted API name for use in the MessageReactions endpoints.
func (e *Emoji) APIName() string {
	if e.ID != "" && e.Name != "" {
		return e.Name + ":" + e.ID
	}
	if e.Name != "" {
		return e.Name
	}
	return e.ID
}

// EmojiParams represents parameters needed to create or update an Emoji.
type EmojiParams struct {
	// Name of the emoji
	Name string `json:"name,omitempty"`
	// A base64 encoded emoji image, has to be smaller than 256KB.
	// NOTE: can be only set on creation.
	Image string `json:"image,omitempty"`
	// Roles for which this emoji will be available.
	Roles []string `json:"roles,omitempty"`
}
