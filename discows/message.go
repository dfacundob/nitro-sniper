package discows

import (
	"encoding/json"
	"time"
)

// The MessageFlags of a Message
type MessageFlags int

// Constants for MessageFlags
const (
	MessageFlagCrossposted MessageFlags = 1 << iota
	MessageFlagIsCrosspost
	MessageFlagSuppressEmbeds
	MessageFlagSourceMessageDeleted
	MessageFlagUrgent
	MessageFlagHasThread
	MessageFlagEphemeral
	MessageFlagLoading // Message is an interaction of type 5, awaiting further response
	MessageFlagFailedToMentionSomeRolesInThread
	_
	_
	_
	MessageFlagSuppressNotifications
	MessageFlagIsVoiceMessage
	MessageFlagsNone MessageFlags = 0
)

type Message struct {
	ApplicationID string             `json:"application_id,omitempty"`
	Type          int                `json:"type"`
	ID            string             `json:"id"`
	ChannelID     string             `json:"channel_id"`
	GuildID       string             `json:"guild_id,omitempty"`
	Timestamp     time.Time          `json:"timestamp"`
	Flags         MessageFlags       `json:"flags"`
	Embeds        []MessageEmbed     `json:"embeds,omitempty"`
	Content       string             `json:"content"`
	Components    []MessageComponent `json:"components,omitempty"`
	Author        User               `json:"author"`
	Attachments   []any              `json:"attachments,omitempty"`
	Reactions     []MessageReactions `json:"reactions,omitempty"`
}

func (m *Message) UnmarshalJSON(data []byte) error {
	type message Message
	var v struct {
		message
		RawComponents []unmarshalableMessageComponent `json:"components"`
	}
	err := json.Unmarshal(data, &v)
	if err != nil {
		return err
	}
	*m = Message(v.message)
	m.Components = make([]MessageComponent, len(v.RawComponents))
	for i, v := range v.RawComponents {
		m.Components[i] = v.MessageComponent
	}
	return err
}

type MessageAttachment struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	ProxyURL    string `json:"proxy_url"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Size        int    `json:"size"`
	Ephemeral   bool   `json:"ephemeral"`
}

type MessageEmbedFooter struct {
	Text         string `json:"text,omitempty"`
	IconURL      string `json:"icon_url,omitempty"`
	ProxyIconURL string `json:"proxy_icon_url,omitempty"`
}

type MessageEmbedImage struct {
	URL      string `json:"url"`
	ProxyURL string `json:"proxy_url,omitempty"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
}

type MessageEmbedThumbnail struct {
	URL      string `json:"url"`
	ProxyURL string `json:"proxy_url,omitempty"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
}

type MessageEmbedVideo struct {
	URL    string `json:"url,omitempty"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

type MessageEmbedProvider struct {
	URL  string `json:"url,omitempty"`
	Name string `json:"name,omitempty"`
}

type MessageEmbedAuthor struct {
	URL          string `json:"url,omitempty"`
	Name         string `json:"name"`
	IconURL      string `json:"icon_url,omitempty"`
	ProxyIconURL string `json:"proxy_icon_url,omitempty"`
}

type MessageEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

type MessageEmbed struct {
	URL         string                 `json:"url,omitempty"`
	Type        EmbedType              `json:"type,omitempty"`
	Title       string                 `json:"title,omitempty"`
	Description string                 `json:"description,omitempty"`
	Timestamp   string                 `json:"timestamp,omitempty"`
	Color       int                    `json:"color,omitempty"`
	Footer      *MessageEmbedFooter    `json:"footer,omitempty"`
	Image       *MessageEmbedImage     `json:"image,omitempty"`
	Thumbnail   *MessageEmbedThumbnail `json:"thumbnail,omitempty"`
	Video       *MessageEmbedVideo     `json:"video,omitempty"`
	Provider    *MessageEmbedProvider  `json:"provider,omitempty"`
	Author      *MessageEmbedAuthor    `json:"author,omitempty"`
	Fields      []*MessageEmbedField   `json:"fields,omitempty"`
}

// https://discord.com/developers/docs/resources/channel#embed-object-embed-types
type EmbedType string

const (
	EmbedTypeRich    EmbedType = "rich"
	EmbedTypeImage   EmbedType = "image"
	EmbedTypeVideo   EmbedType = "video"
	EmbedTypeGifv    EmbedType = "gifv"
	EmbedTypeArticle EmbedType = "article"
	EmbedTypeLink    EmbedType = "link"
)

// MessageReactions holds a reactions object for a message.
type MessageReactions struct {
	Count int    `json:"count"`
	Me    bool   `json:"me"`
	Emoji *Emoji `json:"emoji"`
}
