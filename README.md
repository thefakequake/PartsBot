# PartsBot

A bot that fetches information from PCPartPicker directly from Discord. It is currently not being hosted officially. It uses the [discordgo](https://github.com/bwmarrin/discordgo) API wrapper.

# Features
- Fetches pricing and specs via commands
- Auto formats PCPartPicker URLs
- Utilizes Discord API components
- Configurable using settings command
- Can be self hosted using config file

# Self hosting
To self host this bot, you will need to have Go 1.17 installed to compile the source code, access to a MongoDB instance and a `config.toml` file in the same directory as the `main.go` file structured as follows:
```toml
[bot]
token = "some token"
prefix = "."

[mongo]
uri = "mongodb+srv://someuser:somepass@somewhere.com"
db_name = "some database name"
```

# Monetization
I have also found some ways to monetize the bot via custom affiliate links, to enable this, you will need to add the following to your `config.toml` (example provided is Amazon):
```toml
[[pcpartpicker.affiliates]]
name = "amazon"
full_regexp = "https:\\/\\/(www.)?amazon.(com|co.[a-z]{2})\\/dp\\/[a-zA-Z0-9]{6,12}"
extract_id_regexp = "(?<=\\/dp\\/)[a-zA-Z0-9]{6,12}"
code = "tag=some-affiliate-code"
```
# Plans for the future
- Implement proxy rotation in the future to prevent being blocked by PCPartPicker
- Create some CI/CD routines in order to compile the source code automatically so that you don't need to install Go to run the bot