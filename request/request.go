package request

import (
	"bytes"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"sniper/global"
	"strconv"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
	"golang.org/x/net/http2"
)

var (
	claimer ClaimRequester

	userAgent string

	DiscordHost     string = "canary.discord.com" // "ptb.discord.com" // "discord.com"
	APIVersion      string = "10"
	APIPath         string = "/api/v" + APIVersion
	FullDiscordHost string = "https://" + DiscordHost + APIPath
)

func getRawTlsConfig() *tls.Config {
	return &tls.Config{
		ClientSessionCache:     tls.NewLRUClientSessionCache(1000),
		SessionTicketsDisabled: false,
		MinVersion:             tls.VersionTLS13,
		MaxVersion:             tls.VersionTLS13,
		InsecureSkipVerify:     true,
	}
}

type ClaimRequester interface {
	// Just initialize
	Init(Token string)

	// Called when the token changes
	OnClaimTokenChange(Token string)

	// Return: {statusCode, responseBody, endTime, error}
	ClaimCode(code string) (int, string, time.Time, error)

	// Called to dial discord
	DialDiscord()
}

// called in main.go
func Init(UserAgent, Token string) {
	userAgent = UserAgent

	switch global.Config.Sniper.SnipeType {
	case 0:
		claimer = &fasthttpClaimRequester{}
	case 1:
		claimer = &nethttpClaimRequester{}
	case 2:
		claimer = &dialClaimRequester{}
	default:
		claimer = &fasthttpClaimRequester{}
	}

	// get APIVersion
	if num, err := strconv.Atoi(strings.TrimSpace(global.Config.Discord.APIVersion)); err == nil && num >= 6 && num <= 10 {
		APIVersion = strconv.Itoa(num)
	} else {
		APIVersion = "9"
	}

	// get DiscordHost
	if global.Config.Discord.HostSelection == nil {
		DiscordHost = "canary.discord.com"
	} else {
		switch *global.Config.Discord.HostSelection {
		case 0:
			DiscordHost = "discord.com"
		case 1:
			DiscordHost = "discordapp.com"
		case 2:
			DiscordHost = "ptb.discord.com"
		case 3:
			DiscordHost = "ptb.discordapp.com"
		case 4:
			DiscordHost = "canary.discord.com"
		case 5:
			DiscordHost = "canary.discordapp.com"
		case 6:
			DiscordHost = "canary-api.discordapp.com"
		default:
			DiscordHost = "canary.discord.com"
		}
	}

	// set full discord host, which we will use for sniping. this CAN not include api version
	APIPath = "/api"
	if global.Config.Discord.APIVersion != "" {
		APIPath = APIPath + "/v" + APIVersion
	}

	FullDiscordHost = "https://" + DiscordHost + APIPath

	// finally initialize it
	claimer.Init(Token)

	// create the DialDiscord goroutine
	go func() {
		for !global.ShouldKill {
			claimer.DialDiscord()
			time.Sleep(time.Second * 10)
		}
	}()
}

// called in main.go
func OnClaimTokenChange(Token string) {
	claimer.OnClaimTokenChange(Token)
}

// called by snipers
// Return: {statusCode, responseBody, endTime, error}
func ClaimCode(code string) (int, string, time.Time, error) {
	return claimer.ClaimCode(code)
}

type fasthttpClaimRequester struct {
	fasthttpClient  *fasthttp.Client
	fasthttpReq     *fasthttp.Request
	fasthttpDialReq *fasthttp.Request
}

func (c *fasthttpClaimRequester) Init(Token string) {
	c.fasthttpClient = &fasthttp.Client{
		Name: userAgent,
		Dial: func(addr string) (net.Conn, error) {
			return fasthttp.DialDualStackTimeout(addr, 10*time.Second)
		},
		MaxConnsPerHost:     10,
		MaxIdleConnDuration: 60 * time.Second,
		TLSConfig:           getRawTlsConfig(),
		/*ConfigureClient: func(hc *fasthttp.HostClient) error {
			hc.Addr = "discord.com:443"
			hc.MaxConns = 100
			hc.MaxIdleConnDuration = 60 * time.Second
			return nil
		},*/
	}

	c.fasthttpReq = fasthttp.AcquireRequest()
	c.fasthttpReq.SetBodyString("{}")
	c.fasthttpReq.Header.SetContentLength(2)
	c.fasthttpReq.Header.SetMethod(fasthttp.MethodPost)
	c.fasthttpReq.Header.SetContentType("application/json")
	c.fasthttpReq.Header.SetUserAgent(userAgent)
	c.fasthttpReq.Header.Set("Connection", "keep-alive")
	c.fasthttpReq.Header.Set("Authorization", Token)
	c.fasthttpReq.Header.Set("X-Discord-Locale", "en-US")
	c.fasthttpReq.SetRequestURI(FullDiscordHost + "/entitlements/gift-codes/" + "xxx" + "/redeem")

	c.fasthttpDialReq = fasthttp.AcquireRequest()
	c.fasthttpDialReq.Header.SetMethod(fasthttp.MethodGet)
	c.fasthttpDialReq.Header.SetContentType("application/json")
	c.fasthttpDialReq.Header.SetUserAgent(userAgent)
	c.fasthttpDialReq.Header.Set("Connection", "keep-alive")
	c.fasthttpDialReq.Header.Set("X-Discord-Locale", "en-US")
	c.fasthttpDialReq.SetRequestURI(FullDiscordHost + "/entitlements/gift-codes/" + "xxx" + "/redeem")
}

func (c *fasthttpClaimRequester) OnClaimTokenChange(Token string) {
	c.fasthttpReq.Header.Set("Authorization", Token)
}

// Return: {statusCode, responseBody, endTime, error}
func (c *fasthttpClaimRequester) ClaimCode(code string) (int, string, time.Time, error) {
	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(res)

	c.fasthttpReq.SetRequestURI(FullDiscordHost + "/entitlements/gift-codes/" + code + "/redeem")

	err := c.fasthttpClient.Do(c.fasthttpReq, res)
	endTime := time.Now()

	if err != nil {
		return 0, "", endTime, err
	}

	return res.StatusCode(), string(res.Body()), endTime, nil
}

func (c *fasthttpClaimRequester) DialDiscord() {
	var resp *fasthttp.Response = fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	_ = c.fasthttpClient.Do(c.fasthttpDialReq, resp)
}

type nethttpClaimRequester struct {
	httpClient         *http.Client
	claimHeaders       http.Header
	dialRequestHeaders http.Header
}

func (c *nethttpClaimRequester) Init(Token string) {
	var transport = &http.Transport{
		//TLSClientConfig:     &tls.Config{CipherSuites: []uint16{0x1301}, InsecureSkipVerify: true, PreferServerCipherSuites: true, MinVersion: 0x0304},
		DialContext: (&net.Dialer{
			Timeout:   0,
			KeepAlive: time.Minute,
		}).DialContext,
		TLSClientConfig:     getRawTlsConfig(),
		DisableKeepAlives:   false,
		MaxIdleConnsPerHost: 1000,
		ForceAttemptHTTP2:   true,
		DisableCompression:  false,
		IdleConnTimeout:     0,
		MaxIdleConns:        0,
		MaxConnsPerHost:     0,
		// TLSNextProto:        make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
	}

	http2.ConfigureTransport(transport)

	c.httpClient = &http.Client{
		Transport: transport,
		Timeout:   0,
	}

	// c.httpClient.Jar, _ = cookiejar.New(nil)

	c.claimHeaders = http.Header{
		"Content-Type":     {"application/json"},
		"Authorization":    {Token},
		"User-Agent":       {userAgent},
		"Connection":       {"keep-alive"},
		"X-Discord-Locale": {"en-US"},
	}

	c.dialRequestHeaders = http.Header{
		"Content-Type":     {"application/json"},
		"User-Agent":       {userAgent},
		"Connection":       {"keep-alive"},
		"X-Discord-Locale": {"en-US"},
	}
}

func (c *nethttpClaimRequester) OnClaimTokenChange(Token string) {
	c.claimHeaders = http.Header{
		"Content-Type":     {"application/json"},
		"Authorization":    {Token},
		"User-Agent":       {userAgent},
		"Connection":       {"keep-alive"},
		"X-Discord-Locale": {"en-US"},
	}

	c.dialRequestHeaders = http.Header{
		"Content-Type":     {"application/json"},
		"User-Agent":       {userAgent},
		"X-Discord-Locale": {"en-US"},
	}
}

// Return: {statusCode, responseBody, endTime, error}
func (c *nethttpClaimRequester) ClaimCode(code string) (int, string, time.Time, error) {
	// todo: we could improve this A LOT by preparing the "http.NewRequest" and the body buffer
	request, requestErr := http.NewRequest("POST", FullDiscordHost+"/entitlements/gift-codes/"+code+"/redeem", bytes.NewReader([]byte("{}")))
	if requestErr != nil {
		return 0, "", time.Now(), requestErr
	}

	request.Header = c.claimHeaders

	response, responseErr := c.httpClient.Do(request)
	endTime := time.Now()

	if responseErr != nil {
		return 0, "", endTime, responseErr
	}

	defer response.Body.Close()

	bodyBytes, _ := io.ReadAll(response.Body)
	return response.StatusCode, string(bodyBytes), endTime, nil
}

func (c *nethttpClaimRequester) DialDiscord() {
	request, requestErr := http.NewRequest("GET", FullDiscordHost+"/entitlements/gift-codes/"+"xxx"+"/redeem", nil)
	if requestErr != nil {
		return
	}

	request.Header = c.dialRequestHeaders

	response, responseErr := c.httpClient.Do(request)
	if responseErr != nil {
		return
	}

	response.Body.Close()
}

// tls.Conn (connected via tls.Dial)
type dialClaimRequester struct {
	claimToken string

	fallback *nethttpClaimRequester
	pool     *claimDialerConnPool
}

func (c *dialClaimRequester) Init(Token string) {
	c.claimToken = Token

	c.fallback = &nethttpClaimRequester{}
	c.fallback.Init(Token)

	c.pool = newClaimDialerConnPool(10, DiscordHost, getRawTlsConfig)

	// do a ping now
	// c.DialDiscord()
}

func (c *dialClaimRequester) OnClaimTokenChange(Token string) {
	c.claimToken = Token
	c.fallback.OnClaimTokenChange(Token)
}

// Return: {statusCode, responseBody, endTime, error}
func (c *dialClaimRequester) ClaimCode(code string) (int, string, time.Time, error) {
	dialer, err := c.pool.Get()
	if dialer == nil || err != nil {
		// logger.Info("Dial-Claim fallback to net/http", logger.FieldAny("err", err))
		// fallback to net/http
		return c.fallback.ClaimCode(code)
	}

	defer func() {
		dialer.ShouldRevive.Store(true) // todo: see if we actually need this
		c.pool.Release(dialer)
	}()

	response, err := dialer.MakeRequest([]byte("POST " + APIPath + "/entitlements/gift-codes/" + code + "/redeem HTTP/1.1\r\n" +
		"Host: " + DiscordHost + "\r\n" +
		"Connection: " + "keep-alive" + "\r\n" +
		"Authorization: " + c.claimToken + "\r\n" +
		"X-Discord-Locale: en-US\r\n" +
		"Content-Type: application/json\r\n" +
		"Content-Length: 2\r\n" +
		"\r\n{}"))

	endTime := time.Now()
	if err != nil {
		return 0, "", endTime, err
	}

	bodyBytes, _ := io.ReadAll(response.Body)
	response.Body.Close()

	return response.StatusCode, string(bodyBytes), endTime, nil
}

func (c *dialClaimRequester) DialDiscord() {
	dialerList := c.pool.GetForPing()
	for i := 0; i < len(dialerList); i++ {
		if dialerList[i].ShouldRevive.Load() {
			dialerList[i].EstablishConnection()
			dialerList[i].ShouldRevive.Store(false)
		}

		resp, _ := dialerList[i].MakeRequest([]byte("GET " + APIPath + "/entitlements/gift-codes/" + "xxx" + "/redeem HTTP/1.1\r\n" +
			"Host: " + DiscordHost + "\r\n" +
			"Connection: " + "keep-alive" + "\r\n" +
			"\r\n"))

		if resp != nil {
			resp.Body.Close()
		}

		c.pool.Release(dialerList[i])
	}

	c.fallback.DialDiscord()
}
