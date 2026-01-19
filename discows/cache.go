package discows

import (
	"sync"

	"golang.org/x/exp/slices"
)

type ClientCache struct {
	sync.Mutex

	User       User
	Status     string
	Activities []PresenceActivity
	Guilds     map[string]string // Guilds[id] = name

	SubscribedGuilds []string
}

func (cache *ClientCache) Reset() {
	cache.Lock()
	defer cache.Unlock()

	if cache.Guilds != nil {
		cache.Guilds = make(map[string]string)
	}

	cache.SubscribedGuilds = []string{}
}

func (cache *ClientCache) Init() {
	cache.Lock()
	defer cache.Unlock()

	if cache.Guilds == nil {
		cache.Guilds = make(map[string]string)
	}

	cache.SubscribedGuilds = []string{}
}

func (cache *ClientCache) OnReady(ready EventReady) {
	cache.Lock()
	defer cache.Unlock()

	if cache.Guilds == nil {
		cache.Guilds = make(map[string]string)
	}

	cache.User = ready.User
	for _, guild := range ready.Guilds {
		cache.Guilds[guild.ID] = guild.Properties.Name
	}

	cache.Status = "unknown"
	// if !global.Config.Alts.ForceStatus {
	for _, session := range ready.Sessions {
		if session.SessionID == "all" || session.Active {
			cache.Status = session.Status
			cache.Activities = session.Activities
			break
		}
	}
	// }

	cache.SubscribedGuilds = []string{}
}

func (cache *ClientCache) SetGuildName(guildID, guildName string) {
	cache.Lock()
	defer cache.Unlock()

	if cache.Guilds == nil {
		cache.Guilds = make(map[string]string)
	}

	cache.Guilds[guildID] = guildName
}

func (cache *ClientCache) RemoveGuild(guildID string) {
	cache.Lock()
	defer cache.Unlock()

	if cache.Guilds == nil {
		return
	}

	delete(cache.Guilds, guildID)
}

func (cache *ClientCache) GetGuildName(guildID string) string {
	cache.Lock()
	defer cache.Unlock()

	if cache.Guilds == nil {
		return ""
	}

	if value, contains := cache.Guilds[guildID]; contains {
		return value
	}

	return ""
}

func (cache *ClientCache) ClearSubscriptionsGuilds() {
	cache.Lock()
	defer cache.Unlock()

	cache.SubscribedGuilds = []string{}
}

func (cache *ClientCache) GetSubscribedGuildsList() []string {
	cache.Lock()
	defer cache.Unlock()

	return cache.SubscribedGuilds
}

func (cache *ClientCache) HasSubscribedGuild(guildID string) bool {
	cache.Lock()
	defer cache.Unlock()

	return slices.Contains(cache.SubscribedGuilds, guildID)
}

func (cache *ClientCache) OnSubscribeGuild(guildID string) {
	cache.Lock()
	defer cache.Unlock()

	cache.SubscribedGuilds = append(cache.SubscribedGuilds, guildID)
}
