package discows

import (
	"bytes"
	"compress/zlib"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sniper/global"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

// Discord WebSocket
// Can be used for self-bot
// Optimized for sniper use

var (
	//wssGatewayURL = "wss://gateway.discord.gg/?encoding=json&v=9&compress=zlib-stream"
	// wssGatewayURL = "wss://gateway.discord.gg/?encoding=json&v=9"
	wssGatewayURL = "wss://gateway.discord.gg"
)

var ErrWSAlreadyOpen = errors.New("web socket already opened")
var ErrWSNotFound = errors.New("no websocket connection exists")
var ErrWSTimeout = errors.New("web socket send operation timed out")

type HeartBeatMessage int

type identifyPacketData struct {
	Token        string `json:"token"`
	Capabilities int    `json:"capabilities"`
	Properties   struct {
		Os                     string      `json:"os"`
		Browser                string      `json:"browser"`
		Device                 string      `json:"device"`
		SystemLocale           string      `json:"system_locale"`
		BrowserUserAgent       string      `json:"browser_user_agent"`
		BrowserVersion         string      `json:"browser_version"`
		OsVersion              string      `json:"os_version"`
		Referrer               string      `json:"referrer"`
		ReferringDomain        string      `json:"referring_domain"`
		ReferrerCurrent        string      `json:"referrer_current"`
		ReferringDomainCurrent string      `json:"referring_domain_current"`
		ReleaseChannel         string      `json:"release_channel"`
		ClientBuildNumber      int         `json:"client_build_number"`
		ClientEventSource      interface{} `json:"client_event_source"`
		DesignId               int         `json:"design_id"`
	} `json:"properties"`
	Presence struct {
		Status     string        `json:"status"`
		Since      int           `json:"since"`
		Activities []interface{} `json:"activities"`
		Afk        bool          `json:"afk"`
	} `json:"presence"`
	Compress    bool `json:"compress"`
	ClientState struct {
		GuildVersions map[string]string `json:"guild_versions"`
	} `json:"client_state"`
}

func NewSession(Token string) *Session {
	return &Session{
		Token: Token,
	}
}

var dialerUsed = &websocket.Dialer{
	Proxy:            http.ProxyFromEnvironment,
	HandshakeTimeout: 45 * time.Second,
	// EnableCompression: true,
	TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
}

func (s *Session) Open() error {
	s.Lock()
	defer s.Unlock()

	if s.wsConn != nil {
		return ErrWSAlreadyOpen
	}

	if s.gatewayURL == "" {
		s.gatewayURL = wssGatewayURL
	}

	var host = "gateway.discord.gg"
	if parsedURL, err := url.Parse(s.gatewayURL); err == nil {
		if !strings.HasSuffix(parsedURL.Path, "/") {
			parsedURL.Path = parsedURL.Path + "/"
		}

		query := parsedURL.Query()
		query.Set("encoding", "json")
		query.Set("v", "9")
		// query.Set("compress", "zlib-stream")
		parsedURL.RawQuery = query.Encode()

		s.gatewayURL = parsedURL.String()

		host = parsedURL.Hostname()
	}

	// client.lastHeartbeatSent = time.Now().UTC()
	var err error = nil
	s.wsMutex.Lock()
	s.wsConn, _, err = dialerUsed.Dial(s.gatewayURL, http.Header{
		"Host": {host},
		// "Connection":    {"Upgrade"},
		"Pragma":        {"no-cache"},
		"Cache-Control": {"no-cache"},
		"User-Agent":    {"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"},
		// "Upgrade":       {"websocket"},
		"Origin":          {"https://discord.com"},
		"Accept-Encoding": {"gzip, deflate, br"},
		"Accept-Language": {"en-US,en;q=0.9"},
	})
	if s.wsConn != nil {
		s.wsConn.SetReadLimit(-1)
	}
	s.wsMutex.Unlock()

	if err != nil {
		s.gatewayURL = ""
		err = fmt.Errorf("couldn't dial discord: %w", err)
		return err
	}

	defer func() {
		if err != nil {
			s.wsMutex.Lock()
			if s.wsConn != nil {
				s.wsConn.Close()
				s.wsConn = nil
			}
			s.wsMutex.Unlock()
		}
	}()

	if s.messagesChan == nil {
		s.messagesChan = make(chan interface{})
	}

	if s.heartbeatChan == nil {
		s.heartbeatChan = make(chan interface{})
	}

	// first message should be an OpcodeHello packet
	mt, m, err := s.wsConn.ReadMessage()
	if err != nil {
		// err = fmt.Errorf("couldn't read 1st message: %w", err)
		return err
	}

	// here we handle the message and if it's an OpcodeHello, it'll send the "Identify" packet
	e, err := s.handleMessage(mt, m)
	if err != nil {
		err = fmt.Errorf("couldn't handle 1st message: %w", err)
		return err
	}

	if e.Op != OpcodeHello {
		err = fmt.Errorf("expecting Op 10 (OpcodeHello), got Op %d instead", e.Op)
		return err
	}

	if _, ok := e.DataParsed.(HelloMessage); !ok {
		err = fmt.Errorf("expected a Hello message, unknown received")
		return err
	}

	// it must've sent the "Identify" packet, so we should get either "READY" or "RESUMED" now
	// not gonna check if we received those two, but keeping this ReadMessage because
	// if it's an invalid token, it'll return an error NOW
	mt, m, err = s.wsConn.ReadMessage()
	if err != nil {
		// err = fmt.Errorf("couldn't read 2nd message: %w", err)
		return err
	}
	_, err = s.handleMessage(mt, m)
	if err != nil {
		err = fmt.Errorf("couldn't handle 2nd message: %w", err)
		return err
	}

	s.Cache.Init()
	go s.listenForMessages(s.wsConn)

	return nil
}

func (s *Session) Close() {
	s.CloseWithCode(websocket.CloseNormalClosure)
}

func (s *Session) CloseWithCode(closeCode int) {
	s.Lock()
	defer s.Unlock()

	if closeCode != websocket.CloseServiceRestart {
		s.Cache.Reset()
	}

	if s.messagesChan != nil {
		close(s.messagesChan)
		s.messagesChan = nil
	}

	if s.heartbeatChan != nil {
		close(s.heartbeatChan)
		s.heartbeatChan = nil
	}

	if s.wsConn != nil {
		s.wsMutex.Lock()
		_ = s.wsConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(closeCode, ""))
		_ = s.wsConn.Close()
		s.wsConn = nil
		s.wsMutex.Unlock()

		if closeCode == websocket.CloseNormalClosure || closeCode == websocket.CloseGoingAway {
			s.SessionID = ""
			s.gatewayURL = ""
			s.lastSequenceReceived.Store(0)
		}
	}
}

func (s *Session) reconnect() {
	var err error

	var wait time.Duration = time.Duration(1)
	time.Sleep(wait * time.Second)

	for {
		err = s.Open()
		if err == nil {
			return
		}

		// Certain race conditions can call reconnect() twice. If this happens, we
		// just break out of the reconnect loop
		if err == ErrWSAlreadyOpen {
			return
		}

		// don't reconnect if we shouldn't
		var closeError *websocket.CloseError
		if errors.As(err, &closeError) {
			closeCode := CloseEventCodeByCode(closeError.Code)
			if !closeCode.Reconnect {
				return
			}
		}

		<-time.After(wait * time.Second)
		wait *= 2
		if wait > 600 {
			wait = 600
		}
	}
}

func (client *Session) identifyNew() error {
	var status = "online"
	if client.Cache.Status != "" && client.Cache.Status != "unknown" {
		status = client.Cache.Status
	}

	if global.Config.Alts.ForceStatus {
		status = global.GetConfigAltsStatus()
	}

	identifyData := identifyPacketData{}
	identifyData.Token = client.Token
	identifyData.Capabilities = 30717

	identifyData.Properties.Os = "Windows"
	identifyData.Properties.Browser = "Chrome"
	identifyData.Properties.Device = ""
	identifyData.Properties.SystemLocale = "en-US"
	identifyData.Properties.BrowserUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
	identifyData.Properties.BrowserVersion = "126.0.0.0"
	identifyData.Properties.OsVersion = "10"
	identifyData.Properties.Referrer = ""
	identifyData.Properties.ReferringDomain = ""
	identifyData.Properties.ReferrerCurrent = ""
	identifyData.Properties.ReferringDomainCurrent = ""
	identifyData.Properties.ReleaseChannel = "stable"
	identifyData.Properties.ClientBuildNumber = global.DiscordBuildNumber
	identifyData.Properties.ClientEventSource = nil
	identifyData.Properties.DesignId = 0

	identifyData.Presence.Status = status
	identifyData.Presence.Since = 0
	identifyData.Presence.Activities = []interface{}{}
	identifyData.Presence.Afk = false

	identifyData.Compress = false

	identifyData.ClientState.GuildVersions = map[string]string{}

	return client.SendWSMessage(OpcodeIdentify, identifyData)
}

func (s *Session) resume() error {
	return s.SendWSMessage(OpcodeResume, resumePacketData{
		Token:     s.Token,
		SessionID: s.SessionID,
		Seq:       int(s.lastSequenceReceived.Load()),
	})
}

func (client *Session) SubscribeToGuild(guildID string) error {
	if guildID == "" {
		return fmt.Errorf("no guild id provided")
	}

	if client.Cache.HasSubscribedGuild(guildID) {
		return nil
	}

	type unknownGuildSubscriptionData struct {
		GuildID string `json:"guild_id"`
	}

	if err := client.SendWSMessage(Opcode(36), unknownGuildSubscriptionData{
		GuildID: guildID,
	}); err != nil {
		return err
	}

	type guildDataSub struct {
		Typing     bool `json:"typing"`
		Activities bool `json:"activities"`
		Threads    bool `json:"threads"`
	}

	type GuildSubscriptionData struct {
		Guilds map[string]guildDataSub `json:"subscriptions"`
	}

	if err := client.SendWSMessageWithTimeout(Opcode(37), GuildSubscriptionData{
		Guilds: map[string]guildDataSub{
			guildID: {
				Typing:     true,
				Activities: true,
				Threads:    true,
			},
		},
	}, time.Second*30); err != nil {
		return err
	}

	client.Cache.OnSubscribeGuild(guildID)
	return nil
}

func (client *Session) sendClientData() {
	// {"op":3,"d":{"status":"unknown","since":0,"activities":[],"afk":false}}
	if client.Cache.Status != "" || global.Config.Alts.ForceStatus {
		type PresenceUpdateData struct {
			Status     string             `json:"status"`
			Since      int                `json:"since"`
			Activities []PresenceActivity `json:"activities"`
			Afk        bool               `json:"afk"`
		}

		var appliedStatus = client.Cache.Status
		if global.Config.Alts.ForceStatus {
			appliedStatus = global.GetConfigAltsStatus()
		}

		client.SendWSMessage(OpcodePresenceUpdate, PresenceUpdateData{
			Status:     appliedStatus,
			Since:      0,
			Activities: client.Cache.Activities,
			Afk:        false,
		})
	}

	// {"op":4,"d":{"guild_id":null,"channel_id":null,"self_mute":true,"self_deaf":false,"self_video":false,"flags":2}}
	type voiceStateUpdateData struct {
		GuildID   interface{} `json:"guild_id"`
		ChannelID interface{} `json:"channel_id"`
		SelfMute  bool        `json:"self_mute"`
		SelfDeaf  bool        `json:"self_deaf"`
		SelfVideo bool        `json:"self_video"`
		Flags     int         `json:"flags"`
	}

	client.SendWSMessage(OpcodeVoiceStateUpdate, voiceStateUpdateData{
		GuildID:   nil,
		ChannelID: nil,
		SelfMute:  true,
		SelfDeaf:  false,
		SelfVideo: false,
		Flags:     2,
	})
}

// var messageLimiter = rate.NewLimiter(rate.Limit(30), 5)

func (s *Session) SendWSMessage(op Opcode, data interface{}) error {
	// Wait for the limiter before sending the message
	// messageLimiter.Wait(context.Background())

	s.wsMutex.Lock()
	defer s.wsMutex.Unlock()

	if s.wsConn == nil {
		return ErrWSNotFound
	}

	jsData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// fmt.Println("sending ws message, OP:", op, "DATA:", string(jsData))

	return s.wsConn.WriteJSON(WSMessage{
		Op: op,
		D:  jsData,
	})
}

func (s *Session) SendWSMessageWithTimeout(op Opcode, data interface{}, timeout time.Duration) error {
	if timeout == 0 {
		return s.SendWSMessage(op, data)
	}

	// Wait for the limiter before sending the message
	// messageLimiter.Wait(context.Background())

	s.wsMutex.Lock()
	defer s.wsMutex.Unlock()

	if s.wsConn == nil {
		return ErrWSNotFound
	}

	jsData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() {
		done <- s.wsConn.WriteJSON(WSMessage{
			Op: op,
			D:  jsData,
		})
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return ErrWSTimeout
	}
}

func (client *Session) parseMessage(messageType int, message []byte) (*WSMessage, error) {
	var err error = nil
	var reader io.Reader = bytes.NewReader(message)

	// If this is a compressed message, uncompress it.
	// this is not needed since we don't pass the compress arg in the gateway url
	if messageType == websocket.BinaryMessage {
		z, err2 := zlib.NewReader(reader)
		if err2 != nil {
			// fmt.Println("[onMessage] error uncompressing websocket message,", err2)
			return nil, err2
		}

		defer z.Close()
		reader = z
	}

	// Decode the event into an Event struct.
	var e *WSMessage
	decoder := json.NewDecoder(reader)
	if err = decoder.Decode(&e); err != nil {
		// fmt.Println("[onMessage] error decoding websocket message,", err)
		return e, err
	}

	return e, nil
}

// func OverwriteFile(filePath string, Content string) {
// 	File, err := os.Create(filePath)
// 	if err != nil {
// 		return
// 	}
// 	defer File.Close()
// 	_, err = File.WriteString(Content)
// 	if err != nil {
// 		return
// 	}
// }

func (s *Session) handleMessage(messageType int, message []byte) (receivedMsg *WSMessage, err error) {
	receivedMsg, err = s.parseMessage(messageType, message)
	if err != nil {
		return
	}

	// OverwriteFile(string(receivedMsg.T)+".json", string(receivedMsg.D))

	switch receivedMsg.Op {
	case OpcodeDispatch:
		s.lastSequenceReceived.Store(int64(receivedMsg.S))

		// do our own stuff
		switch receivedMsg.T {
		case EventTypeReady:
			if ready, ok := receivedMsg.DataParsed.(EventReady); ok {
				s.SessionID = ready.SessionID
				s.AnalyticsToken = ready.AnalyticsToken
				s.gatewayURL = ready.ResumeGatewayURL
				s.Cache.OnReady(ready)
				s.sendClientData()
			}

		case EventTypeGuildCreate:
			if guild, ok := receivedMsg.DataParsed.(EventGuildCreate); ok {
				s.Cache.SetGuildName(guild.ID, guild.Properties.Name)
			}

		case EventTypeGuildUpdate:
			if guild, ok := receivedMsg.DataParsed.(EventGuildUpdate); ok {
				s.Cache.SetGuildName(guild.ID, guild.Name)
			}

		case EventTypeGuildDelete:
			if guild, ok := receivedMsg.DataParsed.(EventGuildDelete); ok {
				s.Cache.RemoveGuild(guild.ID)
			}

		}

		// call the event handlers
		s.EventManager.DispatchEvent(s, receivedMsg)
	case OpcodeHeartbeat:
		s.sendHeartbeat()

	case OpcodeReconnect:
		s.EventManager.DispatchEventSessionClose(s, fmt.Errorf("received OpcodeReconnect"), true)
		s.CloseWithCode(websocket.CloseServiceRestart)
		go s.reconnect()

	case OpcodeInvalidSession:
		// todo: see if this is right
		if canResume, ok := receivedMsg.DataParsed.(bool); ok {
			code := websocket.CloseNormalClosure
			if canResume {
				code = websocket.CloseServiceRestart
			} else {
				s.lastSequenceReceived.Store(0)
				s.SessionID = ""
				s.gatewayURL = ""
			}

			s.EventManager.DispatchEventSessionClose(s, fmt.Errorf("received OpcodeInvalidSession"), true)
			s.CloseWithCode(code)
			go s.reconnect()
		} else {
			// SHOULD NEVER HAPPEN!
			s.CloseWithCode(websocket.CloseServiceRestart)
			go s.reconnect()
		}

	case OpcodeHello:
		// this opcode should technically be almost always handled when called by Open()
		if hello, ok := receivedMsg.DataParsed.(HelloMessage); ok {
			s.heartbeatInterval = hello.HeartbeatInterval * time.Millisecond
			go s.heartbeat()

			if s.SessionID == "" && s.lastSequenceReceived.Load() == 0 {
				_ = s.identifyNew()
			} else {
				_ = s.resume()
				s.sendClientData()
			}
		} else {
			// SHOULD NEVER HAPPEN!
			s.CloseWithCode(websocket.CloseServiceRestart)
			go s.reconnect()
		}

		// case OpcodeHeartbeatACK:
		// 	client.Lock()
		// 	client.lastHeartbeatReceived = time.Now().UTC()
		// 	client.Unlock()

	}

	return
}

func (client *Session) heartbeat() {
	heartbeatTicker := time.NewTicker(client.heartbeatInterval)
	defer heartbeatTicker.Stop()

	// defer fmt.Println("exiting heartbeat goroutine...")

	for {
		select {
		case <-client.heartbeatChan:
			return

		case <-heartbeatTicker.C:
			client.sendHeartbeat()
		}
	}
}

func (s *Session) sendHeartbeat() {
	if err := s.SendWSMessage(OpcodeHeartbeat, HeartBeatMessage(s.lastSequenceReceived.Load())); err != nil {
		if err == ErrWSNotFound || errors.Is(err, syscall.EPIPE) {
			return
		}

		//client.Close()
		s.EventManager.DispatchEventSessionClose(s, fmt.Errorf("error sending heartbeat: %w", err), true)
		s.CloseWithCode(websocket.CloseServiceRestart)
		go s.reconnect()

		return
	}

	// client.lastHeartbeatSent = time.Now().UTC()
}

func (s *Session) listenForMessages(wsConn *websocket.Conn) {
	for {
		msgType, msg, err := wsConn.ReadMessage()
		if err != nil {
			s.RLock()
			sameConn := s.wsConn == wsConn
			s.RUnlock()

			if !sameConn {
				return
			}

			var reconnect = false
			var closeError *websocket.CloseError
			if strings.Contains(err.Error(), "connection reset by peer") {
				reconnect = true
			} else if errors.As(err, &closeError) {
				if closeError.Text == io.ErrUnexpectedEOF.Error() {
					reconnect = true
				} else {
					closeCode := CloseEventCodeByCode(closeError.Code)
					reconnect = closeCode.Reconnect

					if closeCode == CloseEventCodeInvalidSeq ||
						closeCode == CloseEventCodeSessionTimed ||
						closeCode == CloseEventCodeUnknown {
						s.Lock()
						s.lastSequenceReceived.Store(0)
						s.SessionID = ""
						s.gatewayURL = ""
						s.Unlock()
					}
				}
			}

			s.EventManager.DispatchEventSessionClose(s, err, reconnect)

			if reconnect {
				s.CloseWithCode(websocket.CloseServiceRestart)
				go s.reconnect()
			} else {
				s.Close()
			}

			// fmt.Println("[listenForMessages] There has been an error reading incomming message", err)
			return
		}

		select {
		case <-s.messagesChan:
			return
		default:
			s.handleMessage(msgType, msg)
		}
	}
}
