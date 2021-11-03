package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
)

type command struct {
	name        string
	description string
	handler     func(*discordgo.Session, *discordgo.MessageCreate, []string)
	args        []string
	aliases     []string
}

type commandRouter struct {
	aliases     map[string]string
	commands    map[string]command
	subhandlers map[int]map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate)
}

func newRouter() commandRouter {
	r := commandRouter{}
	r.commands = map[string]command{}
	r.aliases = map[string]string{}
	r.subhandlers = map[int]map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){}

	return r
}

func (r *commandRouter) addCommand(
	comm command,
) {
	lowerName := strings.ToLower(comm.name)

	for _, a := range append(comm.aliases, comm.name) {
		r.aliases[strings.ToLower(a)] = lowerName
	}

	r.commands[lowerName] = comm
}

func (r commandRouter) getCommand(name string) *command {
	baseCommName, ok := r.aliases[strings.ToLower(name)]
	if !ok {
		return nil
	}

	comm := r.commands[baseCommName]

	return &comm
}

func (r *commandRouter) addSubhandler(eventType int, name string, handler func(s *discordgo.Session, i *discordgo.InteractionCreate)) {
	if len(r.subhandlers[eventType]) == 0 {
		r.subhandlers[eventType] = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){}
	}
	r.subhandlers[eventType][name] = handler
}

func (r commandRouter) getSubhandler(eventType int, name string) func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	events, ok := r.subhandlers[eventType]
	if !ok {
		return nil
	}

	handler, ok := events[strings.ToLower(strings.Split(name, " ")[0])]
	if !ok {
		return nil
	}

	return handler
}

func processCommands(s *discordgo.Session, m *discordgo.MessageCreate) bool {
	if !strings.HasPrefix(m.Content, conf.Bot.Prefix) && !strings.HasPrefix(m.Content, botPing) {
		return false
	}

	splitParts := strings.Split(strings.TrimPrefix(strings.TrimPrefix(m.Content, conf.Bot.Prefix), botPing), " ")

	commName := splitParts[0]
	args := splitParts[1:]

	comm := router.getCommand(commName)

	if comm == nil {
		s.ChannelMessageSend(m.ChannelID, "couldn't find command")
		return true
	} else if len(args) < len(comm.args) && comm.args[len(comm.args)-1][0] != '[' {
		s.ChannelMessageSend(m.ChannelID, "incorrect command usage")
		return true
	} else if len(args) > len(comm.args) {
		args = append(args[:len(comm.args)-1], strings.Join(args[len(comm.args)-1:], " "))
	}

	for i, arg := range comm.args {
		if i+1 > len(args) {
			break
		}
		if strings.Contains(arg, "|") {
			options := strings.Split(strings.Trim(arg, "<>[]"), "|")
			found := false
			for _, opt := range options {
				if strings.ToLower(args[i]) == opt {
					found = true
				}
			}
			if !found {
				s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
					Embed: &discordgo.MessageEmbed{
						Title:       "Invalid argument!",
						Description: fmt.Sprintf("The correct usage for that command is:\n`%s`", comm.Usage()),
						Color:       accent,
					},
				})
				return true
			}
		}
	}

	go comm.handler(s, m, args)
	return true
}

func (c command) Usage() string {
	return conf.Bot.Prefix + strings.ToLower(c.name) + " " + strings.Join(c.args, " ")
}

func sendError(s *discordgo.Session, message string, channelID string) {
	s.ChannelMessageSendEmbed(channelID, &discordgo.MessageEmbed{
		Title: message,
		Color: accent,
	})
	log.Println("Error: " + message)
}
