package main

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/bson"
)

var (
	settingFlags = map[string]int{
		"autopcpp": 1,
		"price":    2,
		"specs":    4,
	}
	defaultSettings = settingFlags["autopcpp"] | settingFlags["price"] | settingFlags["specs"]
)

func init() {
	router.addCommand(command{
		name:        "settings",
		description: "Toggles a setting on or off. Use without any arguments to see list of settings and their values.",
		handler:     settingsCommand,
		args:        []string{"[settingName]", "[on|off]"},
	})
}

func settingsCommand(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 2 {
		g := getGuildState(m.GuildID)

		desc := ""
		for sett, flag := range settingFlags {
			var state string
			if g.Settings&flag != 0 {
				state = "on"
			} else {
				state = "off"
			}
			desc += fmt.Sprintf("**%s:** %s\n", sett, state)
		}

		s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
			Embed: &discordgo.MessageEmbed{
				Title:       "PartsBot settings",
				Description: desc,
				Color:       accent,
			},
			Reference: m.Reference(),
		})

		return
	}

	settingName := strings.ToLower(args[0])
	flag, ok := settingFlags[strings.ToLower(settingName)]
	if !ok {
		settings := []string{}

		for set := range settingFlags {
			settings = append(settings, fmt.Sprintf("`%s`", set))
		}

		s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
			Embed: &discordgo.MessageEmbed{
				Title:       "Invalid setting!",
				Description: fmt.Sprintf("Available settings are:\n%s", strings.Join(settings, ", ")),
				Color:       accent,
			},
			Reference: m.Reference(),
		})
		return
	}
	newState := strings.ToLower(args[1])
	g := getGuildState(m.GuildID)

	var newSettings int
	switch newState {
	case "on":
		newSettings = g.Settings | flag
	case "off":
		newSettings = g.Settings &^ flag
	}

	db.Collection("guilds").UpdateOne(ctx, bson.M{
		"id": m.GuildID,
	}, bson.M{
		"$set": bson.M{
			"settings": newSettings,
		},
	})

	s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Embed: &discordgo.MessageEmbed{
			Description: fmt.Sprintf("Set %s to **%s**.", settingName, newState),
			Color:       accent,
		},
		Reference: m.Reference(),
	})
}
