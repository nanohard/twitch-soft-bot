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
		say(channel, "@"+chUser.DisplayName+" !quote <username> <message> | !quote remove <#>")
		return
	}

	var ch models.Channel
	if err := db.DB.One("Name", channel, &ch); err != nil {
		log.Println(channel + " quote db.Get channel " + err.Error())
		say(channel, "Error "+err.Error())
		return
	}

	// If user trying to print 1 quote
	if number, err := strconv.Atoi(args[0]); err == nil && length == 1 && number <= len(ch.Quotes) {
		say(channel, ch.Quotes[number-1])
		return
	}

	// !quote name[0] [message]
	rem := []string{"rem", "del", "remove", "delete"}
	if length == 2 && permission(chUser) && contains(rem, args[0]) {
		if n, err := strconv.Atoi(args[1]); err != nil && len(ch.Quotes) < n {
			ch.Quotes = remove(ch.Quotes, n-1)

			if err := db.DB.Update(&ch); err != nil {
				log.Println(channel, "quote remove: db.Update()", err)
				say(channel, "@"+chUser.DisplayName+" Error")
				return
			}
			say(channel, "@"+chUser.DisplayName+" Quote #"+args[1]+" has been removed.")
		}
	} else if length > 1 {
		name := args[0]

		// Generate random time.
		min := time.Date(2000, 1, 0, 0, 0, 0, 0, time.UTC).Unix()
		max := time.Now().UTC().Unix()
		delta := max - min
		sec := rand.Int63n(delta) + min

		t := time.Unix(sec, 0).Format("Jan 2 2006")

		// Only the message should be left at this point.
		message := strings.Join(args[1:], " ")
		message = message + " (" + name + " - " + t + ")"
		ch.Quotes = append(ch.Quotes, message)
		num := len(ch.Quotes)

		if err := db.DB.Update(&ch); err != nil {
			log.Println(channel, "quote add: db.Update()", err)
			say(channel, "@"+chUser.DisplayName+" Error")
			return
		}

		say(channel, "@"+chUser.DisplayName+" Quote #" + strconv.Itoa(num) + " has been added.")
		return
	}
}
