package main

import (
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gempir/go-twitch-irc/v2"

	"github.com/nanohard/twitch-soft-bot/pkg/db"
	"github.com/nanohard/twitch-soft-bot/pkg/models"
)

func commandQuote(channel string, chUser *twitch.User, args ...string) {
	// if !permission(chUser) && chUser.Name != "nanohard_" {
	// 	return
	// }

	length := len(args)
	// With no arguments, print help.
	if length == 0 {
		say(channel, "@"+chUser.DisplayName+" !quote <username> <message>")
		return
	}

	// !add add[0] name[1] [message]
	if len(args) > 1 {
		var ch models.Channel
		if err := db.DB.One("Name", channel, &ch); err != nil {
			log.Println(channel + " quote db.Get channel " + err.Error())
			say(channel, "Error " + err.Error())
			return
		}

		name := args[0]
		t := time.Now()
		ts, _ := time.Parse("Jan 2 2006", t.String())

		// Only the message should be left at this point.
		message := strings.Join(args[1:], " ")
		message = message + " (" + name + " - " + ts.String() + ")"
		ch.Quotes = append(ch.Quotes, message)
		num := len(ch.Quotes)

		if err := db.DB.Update(&ch); err != nil {
			log.Println("quote add: db.Save()", err)
			say(channel, "@"+chUser.DisplayName+" Error")
			return
		}

		say(channel, "@"+chUser.DisplayName+" Quote #" + strconv.Itoa(num) + " has been added.")
		return
	}
}
