package sniper

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"sniper/global"
	"sniper/logger"
	"sniper/request"
	"strconv"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
)

var (
	claimedDescriptions = []string{
		"What competition 😉",
		"Another One! :smiling_face_with_3_hearts:",
		"Ah shit, here we go again :rolling_eyes:",
		"I'm on fire 🔥",
		"I bet he's sorry he sent that 😉",
	}

	missedDescriptions = []string{
		"I was on the toilet 🚽",
		"Sorry, I accidentally fell asleep :sleeping:",
		"I'm sorry, I'll do better next time :heart_hands:",
		"Don't give up on me, I won't let you down :pensive:",
		"I'm just warming up :index_pointing_at_the_viewer:",
		"Let's not talk about this :rolling_eyes:",
		"Don't worry, be happy :smile:",
		"I'm in my mom's car, vroom vroom :red_car:",
	}
)

func getRandomClaimedDescription() string {
	return claimedDescriptions[rand.Intn(len(claimedDescriptions))]
}

func getRandomMissedDescription() string {
	return missedDescriptions[rand.Intn(len(missedDescriptions))]
}

func getDiscordFileBuildNumber(body string) (splitData string, file_with_build_num string, discordBuildFiles []string) {
	discordBuildFiles = regexp.MustCompile(`assets/+([a-z0-9]+)\.js`).FindAllString(body, -1)
	if len(discordBuildFiles) >= 2 {
		splitData = "Build Number:"
		file_with_build_num = "https://discord.com/" + discordBuildFiles[len(discordBuildFiles)-2]
		return
	}

	discordBuildFiles = regexp.MustCompile(`assets/([0-9]+\.[a-z0-9]+)\.js`).FindAllString(body, -1)
	if len(discordBuildFiles) >= 2 {
		splitData = "buildNumber"
		file_with_build_num = "https://discord.com/" + discordBuildFiles[len(discordBuildFiles)-1]
		return
	}

	discordBuildFiles = regexp.MustCompile(`assets/sentry.([a-z0-9]+)\.js`).FindAllString(body, -1)
	if len(discordBuildFiles) != 0 {
		splitData = "buildNumber"
		file_with_build_num = "https://discord.com/" + discordBuildFiles[len(discordBuildFiles)-1]
		return
	}

	return "", "", nil
}

func GetDiscordBuildNumber() (int, error) {
	makeGetReq := func(urlStr string) ([]byte, error) {
		ReqUrl, err := url.Parse(strings.TrimSpace(urlStr))
		if err != nil {
			return nil, err
		}

		client := &http.Client{
			Timeout: time.Duration(10 * time.Second),
			Transport: &http.Transport{
				DisableKeepAlives: true,
				IdleConnTimeout:   0,
			},
		}

		res, err := client.Get(ReqUrl.String())
		if err != nil {
			return nil, err
		}

		defer res.Body.Close()

		bodyBytes, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}

		client.CloseIdleConnections()
		return bodyBytes, nil
	}

	responeBody, err := makeGetReq("https://discord.com/app")
	if err != nil {
		return 0, err
	}

	splitData, file_with_build_num, _ := getDiscordFileBuildNumber(string(responeBody))
	if file_with_build_num == "" {
		return 0, fmt.Errorf("failed to find discord build files")
	}

	responeBody, err = makeGetReq(file_with_build_num)
	if err != nil {
		return 0, err
	}

	bodySplit := strings.Split(string(responeBody), splitData)
	if len(bodySplit) <= 1 {
		return 0, fmt.Errorf("failed to find client build number in build file: Build Number not found")
	}

	all_possible_client_build_numbers := regexp.MustCompile(`"[0-9]{6}"`).FindAllString(bodySplit[1], -1)
	if len(all_possible_client_build_numbers) <= 0 {
		return 0, fmt.Errorf("failed to find client build number in build file")
	}

	client_build_number, err := strconv.Atoi(strings.Replace(all_possible_client_build_numbers[0], "\"", "", -1))
	if err != nil {
		return 0, err
	}

	return client_build_number, nil
}

type GiftData struct {
	GotData    bool
	StatusCode int
	Body       string
	End        time.Time
}

func CheckGiftLink(code string) (giftData GiftData) {
	var err error = nil
	giftData.StatusCode, giftData.Body, giftData.End, err = request.ClaimCode(code)
	giftData.GotData = (err == nil)
	if err != nil {
		fmt.Println(err)
	}

	return
}

type IancuSniperDataHeader struct {
	ApiKey       string        `json:"api_key"`            // API key (this is the sniper's license key), recommended that you check it in your API
	Code         string        `json:"code"`               // The gift code
	Delay        time.Duration `json:"delay"`              // Sniping delay in nanoseconds: type Duration int64
	Claimed      bool          `json:"claimed"`            // Is the code claimed?
	Type         string        `json:"type,omitempty"`     // The type of gift it was (only available when "claimed" is true)
	Response     string        `json:"response,omitempty"` // The response of the gift claim (only available when "claimed" is false)
	SniperToken  string        `json:"sniper_token"`       // Last 5 characters of the token of the sniper that saw the code
	ClaimerToken string        `json:"claimer_token"`      // Last 5 characters of the token of the user who claimed the code
	Sender       string        `json:"sender"`             // The username of user who sent the gift
	GuildID      string        `json:"guild_id"`           // The ID of the guild where the gift was sent
	GuildName    string        `json:"guild_name"`         // The name of the guild where the gift was sent
}

type embedFieldStruct struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type EmbedStruct struct {
	Color       int                `json:"color"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Timestamp   time.Time          `json:"timestamp,omitempty"`
	Fields      []embedFieldStruct `json:"fields"`
	Thumbnail   struct {
		URL string `json:"url,omitempty"`
	} `json:"thumbnail"`
	Footer struct {
		Text    string `json:"text"`
		IconUrl string `json:"icon_url,omitempty"`
	} `json:"footer"`
}

type WebhookData struct {
	Content interface{}   `json:"content"`
	Embeds  []EmbedStruct `json:"embeds"`
}

func isImageURL(urlStr string) bool {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	ext := filepath.Ext(parsedURL.Path)
	ext = strings.ToLower(ext)

	// Check if the file extension corresponds to an embeddable image type
	switch ext {
	case ".png", ".gif", ".jpg", ".jpeg", ".bmp", ".tiff", ".webp", ".svg", ".ico":
		return true
	default:
		return false
	}
}

func WebhookSuccess(Code string, Delay time.Duration, sniperToken, Type, Sender, GuildID, GuildName string) {
	if global.Config.Discord.Webhooks.Successful == "" {
		return
	}

	embedMedia := "https://i.imgur.com/QAeVbCz.gif"
	if len(global.Config.Discord.Webhooks.EmbedMedia) > 0 && isImageURL(global.Config.Discord.Webhooks.EmbedMedia) {
		embedMedia = global.Config.Discord.Webhooks.EmbedMedia
	}

	// YYYY-MM-DDTHH:MM:SS.MSSZ
	//timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.999999999Z07:00")

	data := WebhookData{}
	data.Content = nil

	embedData := EmbedStruct{}
	embedData.Color = 13900828 // #D41C1C
	embedData.Title = "CoolService Sniper"
	embedData.Description = ":white_check_mark: | " + getRandomClaimedDescription()
	embedData.Timestamp = time.Now()

	embedData.Fields = []embedFieldStruct{
		{
			Name:   "Code",
			Value:  "`" + Code + "`",
			Inline: true,
		},
		{
			Name:   "Delay",
			Value:  "`" + fmt.Sprintf("%f", Delay.Seconds()) + "s`",
			Inline: true,
		},
		{
			Name:   "Type",
			Value:  "`" + Type + "`",
			Inline: true,
		},
		{
			Name:   "Sniper",
			Value:  "`" + sniperToken[len(sniperToken)-5:] + "`",
			Inline: true,
		},
		{
			Name:   "Sender",
			Value:  "`" + Sender + "`",
			Inline: true,
		},
		{
			Name:   "Guild",
			Value:  "`" + GuildID + " | " + GuildName + "`",
			Inline: true,
		},
		{
			Name:   "Claimer",
			Value:  "`" + global.SnipingToken[len(global.SnipingToken)-5:] + "`",
			Inline: true,
		},
	}

	embedData.Thumbnail.URL = embedMedia

	embedData.Footer.Text = global.Hostname
	embedData.Footer.IconUrl = embedMedia

	data.Embeds = append(data.Embeds, embedData)

	body, _ := json.Marshal(data)

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(res)

	req.Header.SetContentType("application/json")
	req.SetBody(body)
	req.Header.SetMethod(fasthttp.MethodPost)
	req.SetRequestURI(global.Config.Discord.Webhooks.Successful)
	req.SetTimeout(time.Minute)

	var snipeDataHeader, _ = json.Marshal(IancuSniperDataHeader{
		ApiKey:       global.Config.License.Key,
		Code:         Code,
		Delay:        Delay,
		Claimed:      true,
		Type:         Type,
		Response:     "",
		SniperToken:  sniperToken[len(sniperToken)-5:],
		ClaimerToken: global.SnipingToken[len(global.SnipingToken)-5:],
		Sender:       Sender,
		GuildID:      GuildID,
		GuildName:    GuildName,
	})

	req.Header.Set("X-CoolService Snipers-Data", base64.RawStdEncoding.EncodeToString(snipeDataHeader))

	if err := fasthttp.Do(req, res); err != nil {
		logger.Error("Failed to send webhook (success)", logger.FieldAny("error", err))
		return
	}
}

func WebhookFail(Code string, Delay time.Duration, sniperToken, Sender, GuildID, GuildName, Response string) {
	if global.Config.Discord.Webhooks.Missed == "" {
		return
	}

	embedMedia := "https://i.imgur.com/QAeVbCz.gif"
	if len(global.Config.Discord.Webhooks.EmbedMedia) > 0 && isImageURL(global.Config.Discord.Webhooks.EmbedMedia) {
		embedMedia = global.Config.Discord.Webhooks.EmbedMedia
	}

	// YYYY-MM-DDTHH:MM:SS.MSSZ
	//timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.999999999Z07:00")

	data := WebhookData{}
	data.Content = nil

	embedData := EmbedStruct{}
	embedData.Color = 13900828 // #D41C1C
	embedData.Title = "CoolService Sniper"
	embedData.Description = ":x: | " + getRandomMissedDescription()
	embedData.Timestamp = time.Now()

	embedData.Fields = []embedFieldStruct{
		{
			Name:   "Code",
			Value:  "`" + Code + "`",
			Inline: true,
		},
		{
			Name:   "Delay",
			Value:  "`" + fmt.Sprintf("%f", Delay.Seconds()) + "s`",
			Inline: true,
		},
		{
			Name:   "Sniper",
			Value:  "`" + sniperToken[len(sniperToken)-5:] + "`",
			Inline: true,
		},
		{
			Name:   "Sender",
			Value:  "`" + Sender + "`",
			Inline: true,
		},
		{
			Name:   "Guild",
			Value:  "`" + GuildID + " | " + GuildName + "`",
			Inline: true,
		},
		{
			Name:   "Claimer",
			Value:  "`" + global.SnipingToken[len(global.SnipingToken)-5:] + "`",
			Inline: true,
		},
		{
			Name:   "Response",
			Value:  "```" + Response + "```",
			Inline: false,
		},
	}

	embedData.Thumbnail.URL = embedMedia

	embedData.Footer.Text = global.Hostname
	embedData.Footer.IconUrl = embedMedia

	data.Embeds = append(data.Embeds, embedData)

	body, _ := json.Marshal(data)

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(res)

	req.Header.SetContentType("application/json")
	req.SetBody(body)
	req.Header.SetMethod(fasthttp.MethodPost)
	req.SetRequestURI(global.Config.Discord.Webhooks.Missed)
	req.SetTimeout(time.Minute)

	var snipeDataHeader, _ = json.Marshal(IancuSniperDataHeader{
		ApiKey:       global.Config.License.Key,
		Code:         Code,
		Delay:        Delay,
		Claimed:      false,
		Type:         "",
		Response:     Response,
		SniperToken:  sniperToken[len(sniperToken)-5:],
		ClaimerToken: global.SnipingToken[len(global.SnipingToken)-5:],
		Sender:       Sender,
		GuildID:      GuildID,
		GuildName:    GuildName,
	})

	req.Header.Set("X-Coolservices-Data", base64.RawStdEncoding.EncodeToString(snipeDataHeader))

	if err := fasthttp.Do(req, res); err != nil {
		logger.Error("Failed to send webhook (miss)", logger.FieldAny("error", err))
		return
	}
}
