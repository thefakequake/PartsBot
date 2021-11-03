package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	accent = 0x1e807c
)

var (
	conf    = getConfig("config.toml")
	router  = newRouter()
	ctx     = context.TODO()
	db      *mongo.Database
	botPing string
)

type guild struct {
	ID       string `bson:"id"`
	Settings int
	Requests int
}

func getGuildState(ID string) guild {
	var g guild
	doc := db.Collection("guilds").FindOne(ctx, bson.M{
		"id": ID,
	})
	doc.Decode(&g)
	return g
}

func main() {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(conf.Mongo.URI))
	if err != nil {
		log.Fatal(err)
	}
	db = client.Database(conf.Mongo.DBName)

	dg, err := discordgo.New("Bot " + conf.Bot.Token)
	if err != nil {
		log.Fatal(err)
	}

	dg.AddHandler(messageCreate)
	dg.AddHandler(interactionCreate)
	dg.AddHandler(guildCreate)

	dg.Identify.Intents = discordgo.IntentsAll

	err = dg.Open()
	if err != nil {
		log.Fatal(err)
	}

	botPing = fmt.Sprintf("<@%s>", dg.State.User.ID)
	log.Printf("Bot logged in as %s#%s.\n", dg.State.User.Username, dg.State.User.Discriminator)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop

	log.Println("Bot is shutting down...")
	dg.Close()
}

// Handles message create events for commands
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}
	ok := processCommands(s, m)
	if ok {
		return
	}
	processPCPP(s, m)
}

// Handles interaction events for components
func interactionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var handler func(s *discordgo.Session, i *discordgo.InteractionCreate) = nil
	switch i.Type {
	case 3:
		handler = router.getSubhandler(int(i.Type), i.MessageComponentData().CustomID)
	}
	if handler == nil {
		return
	}
	handler(s, i)
}

// Handles guild create events for adding guilds to the database upon joining
func guildCreate(s *discordgo.Session, g *discordgo.GuildCreate) {
	doc := db.Collection("guilds").FindOne(ctx, bson.M{
		"id": g.ID,
	})
	var dbGuild guild
	if err := doc.Decode(&dbGuild); errors.Is(err, mongo.ErrNoDocuments) {
		log.Printf("Joined a new server: \"%s\" ID: %s\n", g.Name, g.ID)
		db.Collection("guilds").InsertOne(ctx, guild{
			ID:       g.ID,
			Settings: defaultSettings,
			Requests: 0,
		})
	}
}
