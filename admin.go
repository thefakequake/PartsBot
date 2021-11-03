package main

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/bson"
)

func init() {
	router.addCommand(
		command{
			name: "cleardb",
			description: "Clears all items in the database for a specific collection.",
			args: []string{"<colName>"},
			handler: clearDb,
			aliases: []string{"dbclear"},
		},
	)
}

func clearDb(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if m.Author.ID != "405798011172814868" {
		s.ChannelMessageSend(m.ChannelID, "go away")
		return
	}
	res, _ := db.Collection(args[0]).DeleteMany(ctx, bson.M{})
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Deleted %v document(s)", res.DeletedCount))
}
