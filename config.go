package main

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type config struct {
	Mongo        mongoConfig
	Bot          botConfig
	PCPartPicker pcpartpickerConfig `toml:"pcpartpicker"`
}

type botConfig struct {
	Token  string
	Prefix string
}

type mongoConfig struct {
	URI    string `toml:"uri"`
	DBName string `toml:"db_name"`
}

type pcpartpickerConfig struct {
	Affiliates []affiliate
	// unused... for now
	Proxies map[string]proxy
}

type affiliate struct {
	Name            string
	Code            string
	FullRegexp      string `toml:"full_regexp"`
	ExtractIDRegexp string `toml:"extract_id_regexp"`
}

type proxy struct {
	XCSRFtoken  string `toml:"xcsrftoken"`
	CfClearance string `toml:"cf_clearance"`
}

func getConfig(fileName string) *config {
	var conf config

	dat, _ := os.ReadFile(fileName)

	_, err := toml.Decode(string(dat), &conf)

	if err != nil {
		fmt.Println(err)
	}

	return &conf
}
