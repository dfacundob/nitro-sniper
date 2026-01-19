package sniper

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"sniper/discows"
	"sniper/files"
	"sniper/global"
	"sniper/logger"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/exp/slices"
)

var (
	reGiftLink   = regexp.MustCompile("(?i)(cord.gifts?/|cord.com/gifts/|promos.discord.gg/|cord.com/billing/promotions/)([a-zA-Z0-9]+)")
	reInviteLink = regexp.MustCompile("(discord.gg/|discord.com/invite/)([0-9a-zA-Z]+)")
	//reNitroType  = regexp.MustCompile(` "name": "([ a-zA-Z]+)", "interval"`) // reNitroType = regexp.MustCompile(` "name": "([ a-zA-Z]+)", "features"`)
)

func checkIfDupeCode(code string) bool {
	for _, _code := range global.DetectedNitros {
		if code == _code {
			return true
		}
	}

	return false
}

func getNitroGift(content string) (has bool, giftId string) {
	// TIP: using this can be faster but less safe
	// var lowContent string = strings.ToLower(content)
	// has = strings.Contains(lowContent, "cord.gift") || strings.Contains(lowContent, "promos.discord.gg")
	// if has {
	// 	var gift []string = strings.Split(content, "/")
	// 	giftId = strings.Split(strings.Split(gift[len(gift)-1], "\n")[0], " ")[0]
	// 	giftId, _ = strings.CutSuffix(giftId, "#")
	// }

	has = reGiftLink.MatchString(content)
	if has {
		code := reGiftLink.FindStringSubmatch(content)
		if len(code) < 2 {
			has = false
			return
		}

		giftId = code[2]
	}

	return
}

type Sniper struct {
	client *discows.Session
	Token  string
	Loaded bool
	Guilds int

	// if we're inside the Init() function
	inInit bool
}

func (sniper *Sniper) Init() (err error) {
	sniper.inInit = true
	defer func() {
		sniper.inInit = false
	}()

	sniper.client = discows.NewSession(sniper.Token)

	sniper.client.EventManager.RunEventsRoutine = true
	sniper.client.EventManager.AddHandler(sniper.onClose)
	sniper.client.EventManager.AddHandler(sniper.onReady)
	sniper.client.EventManager.AddHandler(sniper.onResumed)
	sniper.client.EventManager.AddHandler(sniper.onMessageCreate)

	err = sniper.client.Open()
	if err != nil {
		var closeError *websocket.CloseError
		if errors.As(err, &closeError) {
			closeCode := discows.CloseEventCodeByCode(closeError.Code)
			if closeCode.Code == discows.CloseEventCodeAuthenticationFailed.Code {
				atomic.AddUint64(&global.TotalAlts, ^uint64(0))
				atomic.AddUint64(&global.DeadAlts, uint64(1))

				if tokenFull := global.GetTokenFull(sniper.Token); len(tokenFull) > 0 {
					files.AppendFile("data/dead_alts.txt", tokenFull)
					logger.Fail("Dead token", logger.FieldString("token", tokenFull))
				} else {
					files.AppendFile("data/dead_alts.txt", sniper.Token)
					logger.Fail("Dead token", logger.FieldString("token", sniper.Token))
				}

				global.RemoveAltToken(sniper.Token)
				return nil
			}
		}

		return
	}

	return
}

func (sniper *Sniper) Close() {
	if sniper.client != nil {
		sniper.client.Close()
	}
}

func (sniper *Sniper) onClose(s *discows.Session, event discows.EventSessionClose) {
	if sniper.inInit {
		return
	}

	if sniper.Loaded /*&& !event.Reconnect*/ {
		atomic.AddUint64(&global.LoadedAlts, ^uint64(0))
		atomic.AddUint64(&global.LoadedServers, ^uint64(sniper.Guilds-1))
		sniper.Loaded = false
	}

	if event.Error != nil {
		var closeError *websocket.CloseError
		if errors.As(event.Error, &closeError) {
			closeCode := discows.CloseEventCodeByCode(closeError.Code)
			if closeCode.Code == discows.CloseEventCodeAuthenticationFailed.Code {
				atomic.AddUint64(&global.TotalAlts, ^uint64(0))
				atomic.AddUint64(&global.DeadAlts, uint64(1))

				if tokenFull := global.GetTokenFull(sniper.Token); len(tokenFull) > 0 {
					files.AppendFile("data/dead_alts.txt", tokenFull)
					logger.Fail("Dead token", logger.FieldString("token", tokenFull))
				} else {
					files.AppendFile("data/dead_alts.txt", sniper.Token)
					logger.Fail("Dead token", logger.FieldString("token", sniper.Token))
				}

				global.RemoveAltToken(sniper.Token)
				return
			}
		}

		if !event.Reconnect {
			logger.Error("Session close", logger.FieldString("error", event.Error.Error()), logger.FieldAny("token", global.HideTokenLog(sniper.Token)))
		}
	}

}

func (sniper *Sniper) onReady(s *discows.Session, ready discows.EventReady) {
	// it RE-LOADED then
	if sniper.Loaded {
		atomic.AddUint64(&global.LoadedAlts, ^uint64(0))
		atomic.AddUint64(&global.LoadedServers, ^uint64(sniper.Guilds-1))
	}

	sniper.Loaded = true
	sniper.Guilds = len(ready.Guilds)

	atomic.AddUint64(&global.LoadedAlts, 1)
	atomic.AddUint64(&global.LoadedServers, uint64(len(ready.Guilds)))

	// subscribe to every large guild
	go func(guilds []discows.Guild) {
		slices.SortFunc(guilds, func(a discows.Guild, b discows.Guild) int {
			// ascending: a-b
			// descending: b-a
			return a.MemberCount - b.MemberCount
			// return b.MemberCount - a.MemberCount
		})

		for _, guild := range guilds {
			if global.ShouldKill {
				return
			}

			if sniper.client == nil {
				return
			}

			if !guild.Large {
				continue
			}

			// i've seen we don't get incoming messages from the guilds with COMMUNITY_EXP_LARGE_UNGATED
			// but we must see if it also happens on other ones, so for now we handle both
			// COMMUNITY_EXP_LARGE_UNGATED and COMMUNITY_EXP_LARGE_GATED
			if !slices.ContainsFunc(guild.Properties.Features, func(feature string) bool {
				return strings.Contains(feature, "COMMUNITY_EXP_LARGE")
			}) {
				continue
			}

			global.QueueFunctionsPtr.Queue(false, func(a ...any) {
				if global.ShouldKill || sniper == nil || sniper.client == nil {
					return
				}

				sniper.client.SubscribeToGuild(a[0].(string))
				time.Sleep(time.Second * time.Duration(5+rand.Intn(3)))
			}, guild.ID)
		}
	}(ready.Guilds)
}

func (sniper *Sniper) onResumed(s *discows.Session, _ discows.EventResumed) {
	// logger.Success("Session resumed", logger.FieldString("token", global.HideTokenLog(sniper.Token)))

	if sniper.Loaded {
		atomic.AddUint64(&global.LoadedAlts, ^uint64(0))
		atomic.AddUint64(&global.LoadedServers, ^uint64(sniper.Guilds-1))
	}

	sniper.Loaded = true
	sniper.Guilds = len(s.Cache.Guilds)

	atomic.AddUint64(&global.LoadedAlts, 1)
	atomic.AddUint64(&global.LoadedServers, uint64(len(s.Cache.Guilds)))
}

func (sniper *Sniper) checkIfInviteLink(messageContent string) {
	// removed check for stats
	// if !global.Config.Sniper.SaveInvites {
	// 	return
	// }

	if !reInviteLink.MatchString(messageContent) {
		return
	}

	code := reInviteLink.FindStringSubmatch(messageContent)

	if len(code) < 2 {
		return
	}

	if len(code[2]) < 1 {
		return
	}

	if global.Config.Sniper.SaveInvites {
		global.Invites = append(global.Invites, code[2])
	}

	atomic.AddUint64(&global.APIStatsInvites, 1)
	atomic.AddUint64(&global.FoundInvites, 1)
}

func (sniper *Sniper) checkIfPromocode(giftCode, giftResponse string) {
	// removed check for stats
	// if !global.Config.Sniper.SavePromoCodes {
	// 	return
	// }

	if !strings.Contains(giftResponse, `"Payment source required to redeem gift."`) &&
		!strings.Contains(giftResponse, `Cannot redeem gift`) {
		return
	}

	if global.Config.Sniper.SavePromoCodes {
		global.Promocodes = append(global.Promocodes, giftCode)
	}

	atomic.AddUint64(&global.FoundPromocodes, 1)
}

type nitroClaimedStruct struct {
	// ID               string      `json:"id"`
	// SkuID            string      `json:"sku_id"`
	// ApplicationID    string      `json:"application_id"`
	// UserID           string      `json:"user_id"`
	// PromotionID      interface{} `json:"promotion_id"`
	// Type             int         `json:"type"`
	// Deleted          bool        `json:"deleted"`
	// GiftCodeFlags    int         `json:"gift_code_flags"`
	// Consumed         bool        `json:"consumed"`
	// GifterUserID     string      `json:"gifter_user_id"`
	SubscriptionPlan struct {
		// ID            string      `json:"id"`
		Name string `json:"name"`
		// Interval      int         `json:"interval"`
		// IntervalCount int         `json:"interval_count"`
		// TaxInclusive  bool        `json:"tax_inclusive"`
		// SkuID         string      `json:"sku_id"`
		// Currency      string      `json:"currency"`
		// Price         int         `json:"price"`
		// PriceTier     interface{} `json:"price_tier"`
	} `json:"subscription_plan"`
	Sku struct {
		// 	ID             string        `json:"id"`
		// 	Type           int           `json:"type"`
		// 	DependentSkuID interface{}   `json:"dependent_sku_id"`
		// 	ApplicationID  string        `json:"application_id"`
		// 	ManifestLabels interface{}   `json:"manifest_labels"`
		// 	AccessType     int           `json:"access_type"`
		Name string `json:"name"`
		// 	Features       []interface{} `json:"features"`
		// 	ReleaseDate    interface{}   `json:"release_date"`
		// 	Premium        bool          `json:"premium"`
		// 	Slug           string        `json:"slug"`
		// 	Flags          int           `json:"flags"`
		// 	ShowAgeGate    bool          `json:"show_age_gate"`
	} `json:"sku"`
	StoreListing struct {
		Sku struct {
			Name string `json:"name"`
		} `json:"sku"`
	} `json:"store_listing"`
}

func (sniper *Sniper) onGiftMiss(startTime time.Time, giftId, delay string) {
	global.DetectedNitros = append(global.DetectedNitros, giftId)

	// TODO: you can add API implementation here
}

func (sniper *Sniper) onGiftClaim(startTime time.Time, giftId, nitroType, delay string) {
	global.DetectedNitros = append(global.DetectedNitros, giftId)

	// TODO: you can add API implementation here
}

func (sniper *Sniper) snipeDiscordMessage(message discows.Message) {
	if containsNitro, giftId := getNitroGift(message.Content); containsNitro {
		if len(giftId) >= 16 {
			if !checkIfDupeCode(giftId) {
				var spamIdentifier string = message.GuildID
				if len(spamIdentifier) < 2 {
					spamIdentifier = message.Author.ID
				}

				if !global.SpamDetectorPtr.IsSpam(spamIdentifier) {
					var startTime = time.Now()
					giftData := CheckGiftLink(giftId)
					if !giftData.GotData {
						guildID := "Unknown"
						if len(message.GuildID) > 2 {
							guildID = message.GuildID
						}

						logger.Error("Failed to get gift data (request failed)", logger.FieldString("code", giftId), logger.FieldString("author", message.Author.String()), logger.FieldString("guild_id", guildID))
						return
					}

					var authorName string = message.Author.String()
					var guildId string
					var guildName string = "Unknown"

					if len(message.GuildID) > 2 {
						guildId = message.GuildID
						if sniper.client != nil {
							if tempGuildName := sniper.client.Cache.GetGuildName(guildId); len(tempGuildName) > 0 {
								guildName = tempGuildName
							}
						}
					} else {
						guildId = "DMs"
						guildName = "DMs"
					}

					timeDiff := giftData.End.Sub(startTime)
					delayFormatted := fmt.Sprintf("%f", timeDiff.Seconds()) + "s"

					switch giftData.StatusCode {
					case 0:
						logger.Error("Error sniping", logger.FieldString("code", giftId), logger.FieldString("delay", delayFormatted), logger.FieldString("response", giftData.Body))

					case 200:
						var nitroType string = "Unknown"

						var claimResponse nitroClaimedStruct
						if json.Unmarshal([]byte(giftData.Body), &claimResponse) == nil {
							if len(claimResponse.SubscriptionPlan.Name) >= 3 {
								nitroType = claimResponse.SubscriptionPlan.Name
							} else if len(claimResponse.StoreListing.Sku.Name) >= 3 {
								nitroType = claimResponse.StoreListing.Sku.Name
							} else if len(claimResponse.Sku.Name) >= 3 {
								nitroType = claimResponse.Sku.Name
							}
						}

						go WebhookSuccess(giftId, timeDiff, sniper.Token, nitroType, authorName, guildId, guildName)
						go sniper.onGiftClaim(startTime, giftId, nitroType, delayFormatted)

						atomic.AddUint64(&global.TotalClaimed, 1)

						logger.Success("Claimed Nitro!", logger.FieldString("type", nitroType), logger.FieldString("code", giftId), logger.FieldString("delay", delayFormatted), logger.FieldString("claimToken", global.HideTokenLog(global.SnipingToken)))

					case 400:
						go WebhookFail(giftId, timeDiff, sniper.Token, authorName, guildId, guildName, giftData.Body)
						go sniper.onGiftMiss(startTime, giftId, delayFormatted)

						// only doing that here, on miss
						go sniper.checkIfPromocode(giftId, giftData.Body)

						logger.Fail("Missed gift", logger.FieldString("code", giftId), logger.FieldString("delay", delayFormatted), logger.FieldString("guild_id", guildId))

						atomic.AddUint64(&global.TotalMissed, 1)

					case 401:
						go WebhookFail(giftId, timeDiff, sniper.Token, authorName, guildId, guildName, giftData.Body)
						go sniper.onGiftMiss(startTime, giftId, delayFormatted)

						logger.Fail("Unauthorized claimToken", logger.FieldString("code", giftId), logger.FieldString("delay", delayFormatted), logger.FieldString("claimToken", global.HideTokenLog(global.SnipingToken)))

						atomic.AddUint64(&global.TotalMissed, 1)

					case 403:
						go WebhookFail(giftId, timeDiff, sniper.Token, authorName, guildId, guildName, giftData.Body)
						go sniper.onGiftMiss(startTime, giftId, delayFormatted)

						logger.Fail("Account is locked", logger.FieldString("code", giftId), logger.FieldString("delay", delayFormatted), logger.FieldString("claimToken", global.HideTokenLog(global.SnipingToken)))

						atomic.AddUint64(&global.TotalMissed, 1)

					case 404:
						go sniper.onGiftMiss(startTime, giftId, delayFormatted)

						if strings.Contains(giftData.Body, "Cannot redeem gift") {
							go WebhookFail(giftId, timeDiff, sniper.Token, authorName, guildId, guildName, giftData.Body)
							go sniper.checkIfPromocode(giftId, giftData.Body)

							logger.Fail("Cannot redeem gift code (most likely promo code)", logger.FieldString("code", giftId), logger.FieldString("delay", delayFormatted))

							atomic.AddUint64(&global.TotalMissed, 1)
						} else {
							logger.Fail("Unknown gift code", logger.FieldString("code", giftId), logger.FieldString("delay", delayFormatted))

							atomic.AddUint64(&global.TotalInvalid, 1)
						}

					case 429:
						go WebhookFail(giftId, timeDiff, sniper.Token, authorName, guildId, guildName, giftData.Body)
						go sniper.onGiftMiss(startTime, giftId, delayFormatted)

						logger.Fail("Rate limit", logger.FieldString("code", giftId), logger.FieldString("delay", delayFormatted))

						atomic.AddUint64(&global.TotalMissed, 1)

					default:
						logger.Error("Unknown snipe status", logger.FieldString("code", giftId), logger.FieldString("delay", delayFormatted), logger.FieldString("response", giftData.Body))
					}

					// increase attempts
					atomic.AddUint64(&global.TotalAttempts, 1)
				} else {
					logger.Warn("Spam detected!", logger.FieldString("guildId", message.GuildID), logger.FieldAny("count", global.SpamDetectorPtr.GetCounter(spamIdentifier)), logger.FieldAny("id", spamIdentifier))
				}
			}

			//return
		}
	}
}

func (sniper *Sniper) onMessageCreate(s *discows.Session, e discows.EventMessageCreate) {
	sniper.snipeDiscordMessage(e.Message)
	sniper.checkIfInviteLink(e.Content)

	if e.Content != "" {
		atomic.AddUint64(&global.APIStatsMessages, 1)
		atomic.AddUint64(&global.FoundMessages, 1)
	}
}
