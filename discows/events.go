package discows

import (
	"encoding/json"
)

// EventType wraps all EventType types
type EventType string

// Constants for the gateway events
const (
	// EventTypeRaw is not a real event type, but is used to pass raw payloads to the bot.EventManager
	EventTypeRaw                                 EventType = "__RAW__"
	EventTypeSessionClose                        EventType = "__SESSION_CLOSE__"
	EventTypeHeartbeatAck                        EventType = "__HEARTBEAT_ACK__"
	EventTypeReady                               EventType = "READY"
	EventTypeResumed                             EventType = "RESUMED"
	EventTypeApplicationCommandPermissionsUpdate EventType = "APPLICATION_COMMAND_PERMISSIONS_UPDATE"
	EventTypeAutoModerationRuleCreate            EventType = "AUTO_MODERATION_RULE_CREATE"
	EventTypeAutoModerationRuleUpdate            EventType = "AUTO_MODERATION_RULE_UPDATE"
	EventTypeAutoModerationRuleDelete            EventType = "AUTO_MODERATION_RULE_DELETE"
	EventTypeAutoModerationActionExecution       EventType = "AUTO_MODERATION_ACTION_EXECUTION"
	EventTypeChannelCreate                       EventType = "CHANNEL_CREATE"
	EventTypeChannelUpdate                       EventType = "CHANNEL_UPDATE"
	EventTypeChannelDelete                       EventType = "CHANNEL_DELETE"
	EventTypeChannelPinsUpdate                   EventType = "CHANNEL_PINS_UPDATE"
	EventTypeThreadCreate                        EventType = "THREAD_CREATE"
	EventTypeThreadUpdate                        EventType = "THREAD_UPDATE"
	EventTypeThreadDelete                        EventType = "THREAD_DELETE"
	EventTypeThreadListSync                      EventType = "THREAD_LIST_SYNC"
	EventTypeThreadMemberUpdate                  EventType = "THREAD_MEMBER_UPDATE"
	EventTypeThreadMembersUpdate                 EventType = "THREAD_MEMBERS_UPDATE"
	EventTypeGuildCreate                         EventType = "GUILD_CREATE"
	EventTypeGuildUpdate                         EventType = "GUILD_UPDATE"
	EventTypeGuildDelete                         EventType = "GUILD_DELETE"
	EventTypeGuildAuditLogEntryCreate            EventType = "GUILD_AUDIT_LOG_ENTRY_CREATE"
	EventTypeGuildBanAdd                         EventType = "GUILD_BAN_ADD"
	EventTypeGuildBanRemove                      EventType = "GUILD_BAN_REMOVE"
	EventTypeGuildEmojisUpdate                   EventType = "GUILD_EMOJIS_UPDATE"
	EventTypeGuildStickersUpdate                 EventType = "GUILD_STICKERS_UPDATE"
	EventTypeGuildIntegrationsUpdate             EventType = "GUILD_INTEGRATIONS_UPDATE"
	EventTypeGuildMemberAdd                      EventType = "GUILD_MEMBER_ADD"
	EventTypeGuildMemberRemove                   EventType = "GUILD_MEMBER_REMOVE"
	EventTypeGuildMemberUpdate                   EventType = "GUILD_MEMBER_UPDATE"
	EventTypeGuildMembersChunk                   EventType = "GUILD_MEMBERS_CHUNK"
	EventTypeGuildRoleCreate                     EventType = "GUILD_ROLE_CREATE"
	EventTypeGuildRoleUpdate                     EventType = "GUILD_ROLE_UPDATE"
	EventTypeGuildRoleDelete                     EventType = "GUILD_ROLE_DELETE"
	EventTypeGuildScheduledEventCreate           EventType = "GUILD_SCHEDULED_EVENT_CREATE"
	EventTypeGuildScheduledEventUpdate           EventType = "GUILD_SCHEDULED_EVENT_UPDATE"
	EventTypeGuildScheduledEventDelete           EventType = "GUILD_SCHEDULED_EVENT_DELETE"
	EventTypeGuildScheduledEventUserAdd          EventType = "GUILD_SCHEDULED_EVENT_USER_ADD"
	EventTypeGuildScheduledEventUserRemove       EventType = "GUILD_SCHEDULED_EVENT_USER_REMOVE"
	EventTypeIntegrationCreate                   EventType = "INTEGRATION_CREATE"
	EventTypeIntegrationUpdate                   EventType = "INTEGRATION_UPDATE"
	EventTypeIntegrationDelete                   EventType = "INTEGRATION_DELETE"
	EventTypeInteractionCreate                   EventType = "INTERACTION_CREATE"
	EventTypeInviteCreate                        EventType = "INVITE_CREATE"
	EventTypeInviteDelete                        EventType = "INVITE_DELETE"
	EventTypeMessageCreate                       EventType = "MESSAGE_CREATE"
	EventTypeMessageUpdate                       EventType = "MESSAGE_UPDATE"
	EventTypeMessageDelete                       EventType = "MESSAGE_DELETE"
	EventTypeMessageDeleteBulk                   EventType = "MESSAGE_DELETE_BULK"
	EventTypeMessageReactionAdd                  EventType = "MESSAGE_REACTION_ADD"
	EventTypeMessageReactionRemove               EventType = "MESSAGE_REACTION_REMOVE"
	EventTypeMessageReactionRemoveAll            EventType = "MESSAGE_REACTION_REMOVE_ALL"
	EventTypeMessageReactionRemoveEmoji          EventType = "MESSAGE_REACTION_REMOVE_EMOJI"
	EventTypePresenceUpdate                      EventType = "PRESENCE_UPDATE"
	EventTypeStageInstanceCreate                 EventType = "STAGE_INSTANCE_CREATE"
	EventTypeStageInstanceDelete                 EventType = "STAGE_INSTANCE_DELETE"
	EventTypeStageInstanceUpdate                 EventType = "STAGE_INSTANCE_UPDATE"
	EventTypeTypingStart                         EventType = "TYPING_START"
	EventTypeUserUpdate                          EventType = "USER_UPDATE"
	EventTypeVoiceStateUpdate                    EventType = "VOICE_STATE_UPDATE"
	EventTypeVoiceServerUpdate                   EventType = "VOICE_SERVER_UPDATE"
	EventTypeWebhooksUpdate                      EventType = "WEBHOOKS_UPDATE"
)

// Opcode are opcodes used by discord
type Opcode int

// https://discord.com/developers/docs/topics/opcodes-and-status-codes#gateway-gateway-opcodes
const (
	OpcodeDispatch            Opcode = iota // Receive
	OpcodeHeartbeat                         // Send/Receive
	OpcodeIdentify                          // Send
	OpcodePresenceUpdate                    // Send
	OpcodeVoiceStateUpdate                  // Send
	_                                       //
	OpcodeResume                            // Send
	OpcodeReconnect                         // Receive
	OpcodeRequestGuildMembers               // Send
	OpcodeInvalidSession                    // Receive
	OpcodeHello                             // Receive
	OpcodeHeartbeatACK                      // Receive
)

type WSMessage struct {
	Op         Opcode          `json:"op,omitempty"`
	D          json.RawMessage `json:"d,omitempty"`
	S          int             `json:"s,omitempty"`
	T          EventType       `json:"t,omitempty"`
	DataParsed interface{}     `json:"-"`
}

func (e *WSMessage) UnmarshalJSON(data []byte) error {
	var v struct {
		Op Opcode          `json:"op"`
		D  json.RawMessage `json:"d,omitempty"`
		S  int             `json:"s,omitempty"`
		T  EventType       `json:"t,omitempty"`
	}

	defer func() {
		e.Op = v.Op
		e.D = v.D
		e.S = v.S
		e.T = v.T
	}()

	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}

	var err error = nil

	switch v.Op {
	case OpcodeDispatch:
		e.DataParsed, err = UnmarshalEventData(v.D, v.T)
	case OpcodeHello:
		var d HelloMessage
		err = json.Unmarshal(v.D, &d)
		e.DataParsed = d
	case OpcodeInvalidSession:
		var d bool
		err = json.Unmarshal(v.D, &d)
		e.DataParsed = d
	}

	return err
}

func UnmarshalEventData(data []byte, eventType EventType) (interface{}, error) {
	var (
		eventData interface{}
		err       error
	)
	switch eventType {
	case EventTypeReady:
		var d EventReady
		err = json.Unmarshal(data, &d)
		eventData = d

	case EventTypeResumed:
		// no data
		eventData = EventResumed{}

	case EventTypeGuildCreate:
		var d EventGuildCreate
		err = json.Unmarshal(data, &d)
		eventData = d

	case EventTypeGuildUpdate:
		var d EventGuildUpdate
		err = json.Unmarshal(data, &d)
		eventData = d

	case EventTypeGuildDelete:
		var d EventGuildDelete
		err = json.Unmarshal(data, &d)
		eventData = d

	case EventTypeMessageCreate:
		var d EventMessageCreate
		err = json.Unmarshal(data, &d)
		eventData = d

	case EventTypeMessageUpdate:
		var d EventMessageUpdate
		err = json.Unmarshal(data, &d)
		eventData = d

	case EventTypePresenceUpdate:
		var d EventPresenceUpdate
		err = json.Unmarshal(data, &d)
		eventData = d

	case EventTypeUserUpdate:
		var d EventUserUpdate
		err = json.Unmarshal(data, &d)
		eventData = d

	default:
		eventData = nil
		// 	var d EventUnknown
		// 	err = json.Unmarshal(data, &d)
		// 	eventData = d
	}

	return eventData, err
}

type EventHandler interface {
	Type() EventType
	Handle(*Session, interface{})
}

type EventRaw struct {
	WSMessage
}

type eventRawHandler func(*Session, EventRaw)

func (eh eventRawHandler) Type() EventType {
	return EventTypeRaw
}

func (eh eventRawHandler) Handle(s *Session, i interface{}) {
	if t, ok := i.(EventRaw); ok {
		eh(s, t)
	}
}

type EventSessionClose struct {
	Error     error
	Reconnect bool
}

type eventSessionCloseHandler func(*Session, EventSessionClose)

func (eh eventSessionCloseHandler) Type() EventType {
	return EventTypeSessionClose
}

func (eh eventSessionCloseHandler) Handle(s *Session, i interface{}) {
	if t, ok := i.(EventSessionClose); ok {
		eh(s, t)
	}
}

type EventReady struct {
	Ready
}

type eventReadyHandler func(*Session, EventReady)

func (eh eventReadyHandler) Type() EventType {
	return EventTypeReady
}

func (eh eventReadyHandler) Handle(s *Session, i interface{}) {
	if t, ok := i.(EventReady); ok {
		eh(s, t)
	}
}

type EventResumed struct {
	// no data
}

type eventResumedHandler func(*Session, EventResumed)

func (eh eventResumedHandler) Type() EventType {
	return EventTypeResumed
}

func (eh eventResumedHandler) Handle(s *Session, i interface{}) {
	if t, ok := i.(EventResumed); ok {
		eh(s, t)
	}
}

type EventGuildCreate struct {
	Guild
}

type eventGuildCreateHandler func(*Session, EventGuildCreate)

func (eh eventGuildCreateHandler) Type() EventType {
	return EventTypeGuildCreate
}

func (eh eventGuildCreateHandler) Handle(s *Session, i interface{}) {
	if t, ok := i.(EventGuildCreate); ok {
		eh(s, t)
	}
}

type EventGuildUpdate struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type eventGuildUpdateHandler func(*Session, EventGuildUpdate)

func (eh eventGuildUpdateHandler) Type() EventType {
	return EventTypeGuildUpdate
}

func (eh eventGuildUpdateHandler) Handle(s *Session, i interface{}) {
	if t, ok := i.(EventGuildUpdate); ok {
		eh(s, t)
	}
}

type EventGuildDelete struct {
	ID string `json:"id"`
}

type eventGuildDeleteHandler func(*Session, EventGuildDelete)

func (eh eventGuildDeleteHandler) Type() EventType {
	return EventTypeGuildDelete
}

func (eh eventGuildDeleteHandler) Handle(s *Session, i interface{}) {
	if t, ok := i.(EventGuildDelete); ok {
		eh(s, t)
	}
}

type EventMessageCreate struct {
	Message
}

type eventMessageCreateHandler func(*Session, EventMessageCreate)

func (eh eventMessageCreateHandler) Type() EventType {
	return EventTypeMessageCreate
}

func (eh eventMessageCreateHandler) Handle(s *Session, i interface{}) {
	if t, ok := i.(EventMessageCreate); ok {
		eh(s, t)
	}
}

type EventMessageUpdate struct {
	Message
}

type eventMessageUpdateHandler func(*Session, EventMessageUpdate)

func (eh eventMessageUpdateHandler) Type() EventType {
	return EventTypeMessageUpdate
}

func (eh eventMessageUpdateHandler) Handle(s *Session, i interface{}) {
	if t, ok := i.(EventMessageUpdate); ok {
		eh(s, t)
	}
}

type EventPresenceUpdate struct {
	PresenceActivity
}

type eventPresenceUpdateHandler func(*Session, EventPresenceUpdate)

func (eh eventPresenceUpdateHandler) Type() EventType {
	return EventTypePresenceUpdate
}

func (eh eventPresenceUpdateHandler) Handle(s *Session, i interface{}) {
	if t, ok := i.(EventPresenceUpdate); ok {
		eh(s, t)
	}
}

type EventUserUpdate struct {
	User
}

type eventUserUpdateHandler func(*Session, EventUserUpdate)

func (eh eventUserUpdateHandler) Type() EventType {
	return EventTypeUserUpdate
}

func (eh eventUserUpdateHandler) Handle(s *Session, i interface{}) {
	if t, ok := i.(EventUserUpdate); ok {
		eh(s, t)
	}
}
