package main

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/dlclark/regexp2"
	"github.com/gocolly/colly"
	collyProxy "github.com/gocolly/colly/proxy"
	"github.com/quakecodes/gopartpicker"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type region struct {
	code string
	name string
}

var (
	scraper           = gopartpicker.NewScraper()
	productURLRegexp  = regexp2.MustCompile(`([a-z]{2}\.)?(pcpartpicker|partpicker).com\/product\/[a-zA-Z0-9]{4,8}\/`, 0)
	affiliateIDRegexp = regexp2.MustCompile(`(?<=\/mr\/)[a-zA-Z]*\/[a-zA-Z0-9]{4,8}`, 0)
	regions           = []region{}
)

func init() {
	scraper.SetHeaders("global", map[string]string{
		"User-Agent":                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/93.0.4577.82 Safari/537.36",
		"accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9",
		"accept-encoding":           "gzip, deflate, br",
		"accept-language":           "en-GB,en-US;q=0.9,en;q=0.8",
		"cache-control":             "no-cache",
		"pragma":                    "no-cache",
		"sec-fetch-dest":            "document",
		"sec-fetch-mode":            "navigate",
		"sec-fetch-site":            "none",
		"sec-fetch-user":            "1",
		"sec-gpc":                   "1",
		"upgrade-insecure-requests": "1",
	})
	if len(conf.PCPartPicker.Proxies) > 0 {
		proxyAddrs := []string{}
		for k := range conf.PCPartPicker.Proxies {
			proxyAddrs = append(proxyAddrs, k)
		}
		switcher, err := collyProxy.RoundRobinProxySwitcher(proxyAddrs...)
		if err != nil {
			log.Printf("Failed to start proxy rotation: %s", err.Error())
		}
		scraper.Collector.SetProxyFunc(switcher)
		// scraper.Collector.OnRequest(func(r *colly.Request) {
		// 	proxy, ok := conf.PCPartPicker.Proxies[r.ProxyURL]
		// 	if !ok {
		// 		return
		// 	}
		// 	r.Headers.Set("")
		// })
	}
	regions = getRegions()

	router.addCommand(
		command{
			name:        "Price",
			description: "Fetches pricing information for a part.",
			args:        []string{"<partName>"},
			handler:     priceCommand,
			aliases:     []string{"partprice", "pricepart"},
		},
	)
	router.addCommand(
		command{
			name:        "Specs",
			description: "Fetches specifications for a part.",
			args:        []string{"<partName>"},
			handler:     specsCommand,
			aliases:     []string{"partspecs", "specspart"},
		},
	)
	router.addSubhandler(3, "partselect", partSelectHandler)

	router.addCommand(
		command{
			name:        "Regions",
			description: "Shows all available PCPartPicker regions.",
			handler:     regionsCommand,
			aliases:     []string{"region"},
		},
	)
}

func extractBaseProductURL(URL string) string {
	match, _ := productURLRegexp.FindStringMatch(URL)
	return match.String()
}

func extractAffiliateID(URL string) string {
	match, _ := affiliateIDRegexp.FindStringMatch(URL)
	return match.String()
}

func incRequests(guildID string) {
	db.Collection("guilds").UpdateOne(ctx, bson.M{
		"id": guildID,
	}, bson.M{
		"$inc": bson.M{
			"requests": 1,
		},
	})
}

func getAffiliate(vendor gopartpicker.Vendor, aff affiliate) string {
	var doc bson.M
	urlId := extractAffiliateID(vendor.URL)
	res := db.Collection("urls").FindOne(ctx, bson.M{
		"id": urlId,
	})
	res.Decode(&doc)
	noDocuments := mongo.ErrNoDocuments
	if !errors.As(res.Err(), &noDocuments) {
		return strings.ReplaceAll(doc["url"].(string), "{{}}", aff.Code)
	}

	var redirectURL string

	scraper.Collector.OnResponse(func(r *colly.Response) {
		redirectURL = r.Request.URL.String()
	})

	err := scraper.Collector.Visit(vendor.URL)
	scraper.Collector.Wait()

	if err != nil {
		log.Printf("Failed to convert affiliate link: %s\n", vendor.URL)
		return vendor.URL
	}

	baseURL, _ := regexp2.MustCompile(aff.FullRegexp, 0).FindStringMatch(redirectURL)
	url := fmt.Sprintf("%s?%s", baseURL.String(), aff.Code)

	db.Collection("urls").InsertOne(ctx, bson.M{
		"id":  urlId,
		"url": url,
	})

	return url
}

func getRegions() []region {
	regions := []region{}

	scraper.Collector.OnHTML(".language-selector", func(selector *colly.HTMLElement) {
		if len(regions) > 0 {
			return
		}
		selector.ForEach("option", func(i int, opt *colly.HTMLElement) {
			regions = append(regions, region{
				code: opt.Attr("value"),
				name: opt.Text,
			})
		})
	})

	scraper.Collector.Visit("https://pcpartpicker.com/")
	scraper.Collector.Wait()

	return regions
}

func displayPart(infoType string, URL string, s *discordgo.Session, m *discordgo.Message) {
	s.ChannelMessageEditComplex(&discordgo.MessageEdit{
		Embed: &discordgo.MessageEmbed{
			Title: "Fetching part...",
			Color: accent,
		},
		ID:      m.ID,
		Channel: m.ChannelID,
	})

	incRequests(m.GuildID)
	part, err := scraper.GetPart(URL)
	if err != nil {
		fmt.Printf("Failed to fetch part: %s\n", URL)
		return
	}

	desc := ""

	switch infoType {
	case "price":
		inStock := []string{}
		notInStock := []string{}

		if !(len(part.Vendors) > 0) {
			desc = "No pricing available."
		} else {
			desc = fmt.Sprintf("Available at %v retailer(s):", len(part.Vendors))
			for _, vendor := range part.Vendors {
				vendorURL := vendor.URL
				for _, aff := range conf.PCPartPicker.Affiliates {
					if strings.Contains(strings.ToLower(vendor.Name), aff.Name) {
						vendorURL = getAffiliate(vendor, aff)
					}
				}
				line := fmt.Sprintf("[%s](%s): %s", vendor.Name, vendorURL, vendor.Price.TotalString)
				if vendor.InStock {
					inStock = append(inStock, line)
				} else {
					notInStock = append(notInStock, line)
				}
			}
		}

		fields := []*discordgo.MessageEmbedField{}

		if len(inStock) > 0 {
			if len(inStock) > 10 {
				inStock = append(inStock[:10], fmt.Sprintf("*+%v more...*", len(inStock)-10))
			}
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:  "In stock",
				Value: strings.Join(inStock, "\n"),
			})
		}
		if len(notInStock) > 0 {
			if len(notInStock) > 10 {
				notInStock = append(notInStock[:10], fmt.Sprintf("*+%v more...*", len(notInStock)-10))
			}
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:  "Out of stock",
				Value: strings.Join(notInStock, "\n"),
			})
		}

		embed := &discordgo.MessageEmbed{
			Title:       fmt.Sprintf("Pricing for '%s':", part.Name),
			URL:         URL,
			Color:       accent,
			Description: desc,
			Fields:      fields,
		}

		if len(part.Images) > 0 {
			embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
				URL: part.Images[0],
			}
		}

		s.ChannelMessageEditComplex(&discordgo.MessageEdit{
			Embed:   embed,
			ID:      m.ID,
			Channel: m.ChannelID,
		})
	case "specs":
		if len(part.Specs) > 0 {
			for _, spec := range part.Specs {
				desc += fmt.Sprintf("**%s**: %s\n", spec.Name, strings.Join(spec.Values, ", "))
			}
		} else {
			desc = "No specs available."
		}

		embed := &discordgo.MessageEmbed{
			Title:       fmt.Sprintf("Specs for '%s':", part.Name),
			URL:         URL,
			Color:       accent,
			Description: desc,
		}

		if len(part.Images) > 0 {
			embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
				URL: part.Images[0],
			}
		}

		s.ChannelMessageEditComplex(&discordgo.MessageEdit{
			Embed:   embed,
			ID:      m.ID,
			Channel: m.ChannelID,
		})
	}
}

func priceCommand(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	g := getGuildState(m.GuildID)
	if g.Settings&settingFlags["price"] == 0 {
		return
	}

	partName := args[0]
	region := ""

	for _, reg := range regions {
		split := strings.Split(args[0], " ")
		if reg.code == strings.ToLower(split[0]) {
			partName = strings.Join(split[1:], " ")
			region = strings.ToLower(split[0])
		}
	}

	mes, _ := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Embed: &discordgo.MessageEmbed{
			Title: fmt.Sprintf("Searching for '%s'...", partName),
			Color: accent,
		},
		Reference: m.Reference(),
	})

	incRequests(m.GuildID)
	parts, err := scraper.SearchParts(partName, region)
	if region == "" {
		region = "US"
	} else {
		region = strings.ToUpper(region)
	}

	_, ok := err.(*gopartpicker.RedirectError)
	if ok {
		displayPart("price", err.Error(), s, mes)
		return
	} else if err != nil {
		sendError(s, err.Error(), m.ChannelID)
		return
	} else if len(parts) == 0 {
		s.ChannelMessageEditEmbed(mes.ChannelID, mes.ID, &discordgo.MessageEmbed{
			Title: fmt.Sprintf("Couldn't find part '%s'", args[0]),
			Color: accent,
		})
		return
	} else if len(parts) == 1 {
		displayPart("price", parts[0].URL, s, mes)
		return
	}
	menuOptions := []discordgo.SelectMenuOption{}

	if len(parts) > 20 {
		parts = parts[:20]
	}

	for _, part := range parts {
		var price string
		if !part.Vendor.InStock || len(part.Vendor.Price.TotalString) == 0 {
			price = "Out of stock."
		} else {
			price = part.Vendor.Price.TotalString
		}

		menuOptions = append(menuOptions, discordgo.SelectMenuOption{
			Label:       part.Name,
			Description: price,
			Value:       extractBaseProductURL(part.URL),
		})
	}

	menuOptions = append([]discordgo.SelectMenuOption{
		{
			Label: "Cancel",
			Value: "cancel",
			Emoji: discordgo.ComponentEmoji{
				Name: "❌",
			},
		},
	}, menuOptions...)

	_, editErr := s.ChannelMessageEditComplex(&discordgo.MessageEdit{
		Embed: &discordgo.MessageEmbed{
			Title: fmt.Sprintf("Search results for '%s' in %s:", partName, region),
			Color: accent,
		},
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						CustomID: "partSelect price",
						Options:  menuOptions,
					},
				},
			},
		},
		ID:      mes.ID,
		Channel: m.ChannelID,
	})

	if editErr != nil {
		fmt.Println(editErr)
	}
}

func specsCommand(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	g := getGuildState(m.GuildID)
	if g.Settings&settingFlags["specs"] == 0 {
		return
	}

	mes, err := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Embed: &discordgo.MessageEmbed{
			Title: fmt.Sprintf("Searching for '%s'...", args[0]),
			Color: accent,
		},
		Reference: m.Reference(),
	})
	if err != nil {
		return
	}

	incRequests(m.GuildID)
	parts, err := scraper.SearchParts(args[0], "")

	_, ok := err.(*gopartpicker.RedirectError)
	if ok {
		displayPart("specs", err.Error(), s, mes)
		return
	} else if err != nil {
		sendError(s, err.Error(), m.ChannelID)
		return
	} else if len(parts) == 0 {
		s.ChannelMessageEditEmbed(mes.ChannelID, mes.ID, &discordgo.MessageEmbed{
			Title: fmt.Sprintf("Couldn't find part '%s'", args[0]),
			Color: accent,
		})
		return
	} else if len(parts) == 1 {
		displayPart("specs", parts[0].URL, s, mes)
		return
	}

	menuOptions := []discordgo.SelectMenuOption{}

	if len(parts) > 20 {
		parts = parts[:20]
	}

	for _, part := range parts {
		var price string
		if !part.Vendor.InStock || len(part.Vendor.Price.TotalString) == 0 {
			price = "Out of stock."
		} else {
			price = part.Vendor.Price.TotalString
		}

		menuOptions = append(menuOptions, discordgo.SelectMenuOption{
			Label:       part.Name,
			Description: price,
			Value:       extractBaseProductURL(part.URL),
		})
	}

	menuOptions = append([]discordgo.SelectMenuOption{
		{
			Label: "Cancel",
			Value: "cancel",
			Emoji: discordgo.ComponentEmoji{
				Name: "❌",
			},
		},
	}, menuOptions...)

	_, editErr := s.ChannelMessageEditComplex(&discordgo.MessageEdit{
		Embed: &discordgo.MessageEmbed{
			Title: fmt.Sprintf("Search results for '%s':", args[0]),
			Color: accent,
		},
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						CustomID: "partSelect specs",
						Options:  menuOptions,
					},
				},
			},
		},
		ID:      mes.ID,
		Channel: m.ChannelID,
	})

	if editErr != nil {
		fmt.Println(editErr)
	}
}

func partSelectHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.MessageComponentData()

	if data.Values[0] == "cancel" {
		s.ChannelMessageDelete(i.ChannelID, i.Message.ID)
		return
	}

	partURL := "https://" + data.Values[0]

	displayPart(strings.Split(data.CustomID, " ")[1], partURL, s, i.Message)
}

func regionsCommand(s *discordgo.Session, m *discordgo.MessageCreate, _ []string) {
	desc := ""

	for _, reg := range regions {
		desc += fmt.Sprintf("**%s**: %s\n", reg.code, reg.name)
	}

	channel, err := s.UserChannelCreate(m.Author.ID)

	if err != nil {
		s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
			Embed: &discordgo.MessageEmbed{
				Title:       "Failed to DM",
				Description: "Make sure you don't have the bot blocked.",
				Color:       accent,
			},
			Reference: m.Reference(),
		})
		return
	}

	s.ChannelMessageSendEmbed(channel.ID, &discordgo.MessageEmbed{
		Title:       "Available regions",
		Description: desc,
		Color:       accent,
	})
	s.MessageReactionAdd(m.ChannelID, m.ID, "📨")
}

func processPCPP(s *discordgo.Session, m *discordgo.MessageCreate) {
	g := getGuildState(m.GuildID)
	if g.Settings&settingFlags["autopcpp"] == 0 {
		return
	}

	urlMatches := gopartpicker.ExtractPartListURLs(m.Content)
	if len(urlMatches) < 1 {
		return
	}
	URL := urlMatches[0]

	incRequests(m.GuildID)
	partList, err := scraper.GetPartList(URL)
	if err != nil {
		fmt.Println(err)
		return
	}

	desc := ""
	image := ""

	for i, part := range partList.Parts {
		if part.Image != "" && image == "" {
			image = part.Image
		}
		name := strings.TrimSuffix(part.Name, part.Type)
		var line string
		if part.Vendor.Price.TotalString == "" {
			line = fmt.Sprintf(
				"**%s:** %s\n",
				part.Type,
				name,
			)
		} else {
			line = fmt.Sprintf(
				"**%s:** %s ([%s](%s))\n",
				part.Type,
				name,
				part.Vendor.Price.TotalString,
				part.Vendor.URL,
			)
		}
		toSet := desc + line
		if len(toSet) >= 2000 {
			desc += fmt.Sprintf("*+%v more...*\n", len(partList.Parts)-i)
			break
		}
		desc = toSet

	}

	if desc != "" {
		desc += "\n"
	}

	desc += fmt.Sprintf(
		"**Compatibility Notes:** %v\n**Estimated Wattage:** %s\n**Total Price:** %s",
		len(partList.Compatibility),
		partList.Wattage,
		partList.Price.TotalString,
	)

	s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Embed: &discordgo.MessageEmbed{
			Description: desc,
			Color:       accent,
			Author: &discordgo.MessageEmbedAuthor{
				URL:     URL,
				Name:    fmt.Sprintf("Part List: %v parts", len(partList.Parts)),
				IconURL: image,
			},
		},
		Reference: m.Reference(),
	})
}
