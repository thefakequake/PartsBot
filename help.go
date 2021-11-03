package main

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func init() {
	router.addCommand(
		command{
			name:        "Help",
			description: "Shows usage and description for a command. If no command is provided, shows all commands.",
			args:        []string{"[commandName]"},
			handler:     helpCommand,
			aliases:     []string{"commands"},
		},
	)
}

func helpCommand(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) > 0 {
		commName := strings.ToLower(args[0])
		comm := router.getCommand(commName)

		if comm == nil {
			s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
				Title: fmt.Sprintf("Couldn't find command '%s'.", args[0]),
			})
			return
		}

		fields := []*discordgo.MessageEmbedField{
			{
				Name:  "Usage",
				Value: fmt.Sprintf("`%s`", comm.Usage()),
			},
			{
				Name:  "Description",
				Value: comm.description,
			},
		}

		if len(comm.aliases) > 0 {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:  "Aliases",
				Value: strings.Join(comm.aliases, ", "),
			})
		}

		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Title:  comm.name,
			Color:  accent,
			Fields: fields,
		})
		return
	}

	commands := []string{}

	for n := range router.commands {
		commands = append(commands, n)
	}

	s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
		Title:       "Commands",
		Description: strings.Join(commands, ", "),
		Color:       accent,
	})
}
