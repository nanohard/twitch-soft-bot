package main

import (
	"log"
	"math/rand"
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

	var ch models.Channel
	if err := db.DB.One("Name", channel, &ch); err != nil {
		log.Println(channel + " quote db.Get channel " + err.Error())
		say(channel, "Error "+err.Error())
		return
	}

	number, err := strconv.Atoi(args[0])

	log.Println(err)
	log.Println(number)
	log.Println(length)

	// If user trying to print 1 quote
	if number, err := strconv.Atoi(args[0]); err == nil && length == 1 && number <= length {
		say(channel, ch.Quotes[number-1])
		return
	}

	// !quote name[0] [message]
	if length > 1 {
		name := args[0]

		// Generate random time.
		min := time.Date(2000, 1, 0, 0, 0, 0, 0, time.UTC).Unix()
		max := time.Now().UTC().Unix()
		delta := max - min
		sec := rand.Int63n(delta) + min

		t := time.Unix(sec, 0).Format("Jan 2 2006")
		// t := time.Now().Format("Jan 2 2006")

		// Only the message should be left at this point.
		message := strings.Join(args[1:], " ")
		message = message + " (" + name + " - " + t + ")"
		ch.Quotes = append(ch.Quotes, message)
		num := len(ch.Quotes)

		if err := db.DB.Update(&ch); err != nil {
			log.Println(channel, "quote add: db.Save()", err)
			say(channel, "@"+chUser.DisplayName+" Error")
			return
		}

		say(channel, "@"+chUser.DisplayName+" Quote #" + strconv.Itoa(num) + " has been added.")
		return
	}
}
