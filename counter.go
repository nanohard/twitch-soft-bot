package main

import (
	"log"
	"strconv"
	"strings"

	"github.com/asdine/storm/q"
	"github.com/gempir/go-twitch-irc/v2"

	"github.com/nanohard/twitch-soft-bot/pkg/db"
	"github.com/nanohard/twitch-soft-bot/pkg/models"
)

func commandCounter(channel string, chUser *twitch.User, args ...string) {
	if !permission(chUser) && chUser.Name != "nanohard_" {
		return
	}

	length := len(args)
	// !sr with no arguments, print help
	if length == 0 {
		say(channel, "@"+chUser.DisplayName+" !counter add death SS has died * times ("+
			"SS will be replaced by streamer name; * will be replaced by actual count)")
		return
	}

	// !counter add[0] badpun[1] [message] count
	switch args[0] {
	case "add":
		name := args[1]
		modOnly := true
		var count int

		// Check if last element is permission: nomod.
		// If so, assign it to a var and remove.
		if args[len(args)-1] == "nomod" {
			modOnly = false
			args[len(args)-1] = ""
			args = args[:len(args)-1]
		}

		// Check if last element is int.
		// If so, assign it to a var and remove.
		if countRaw, err := strconv.Atoi(args[len(args)-1]); err == nil {
			count = countRaw
			args[len(args)-1] = ""
			args = args[:len(args)-1]
		}

		// Remove first 2 elements.
		copy(args[0:], args[2:])  // Shift a[i+1:] left two indices.
		args[len(args)-2] = ""    // Erase last two elements (write zero value).
		args = args[:len(args)-2] // Truncate slice.

		// Replace "SS" with streamer name.
		for i, v := range args {
			if v == "SS" {
				args[i] = channel
			}
		}

		// Only the message should be left at this point.
		message := strings.Join(args, " ")

		if err := db.DB.Save(&models.Counter{
			Name:    name,
			Channel: channel,
			Message: message,
			ModOnly: modOnly,
			Count:   count,
		}); err != nil {
			log.Println("counter add: db.Save()", err)
			say(channel, "@"+chUser.DisplayName+" Error")
			return
		}

		say(channel, "@"+chUser.DisplayName+" Counter !"+name+" has been added.")
		return

	case "rm", "remove", "del", "delete":
		var counter models.Counter
		if err := db.DB.Select(q.Eq("Channel", channel), q.Eq("Name", args[1])).First(&counter); err != nil {
			log.Println("counter add: db.Select()", err)
			say(channel, "@"+chUser.DisplayName+" Error")
			return
		}

		if err := db.DB.DeleteStruct(&counter); err != nil {
			log.Println("counter add: db.DeleteStruct()", err)
			say(channel, "@"+chUser.DisplayName+" Error")
			return
		}

		say(channel, "@"+chUser.DisplayName+" Counter "+counter.Name+" has been removed.")
	}
}
