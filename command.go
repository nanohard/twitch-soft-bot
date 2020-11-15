package main

import (
	"log"
	"strings"

	"github.com/asdine/storm/q"
	"github.com/gempir/go-twitch-irc/v2"

	"github.com/nanohard/twitch-soft-bot/pkg/db"
	"github.com/nanohard/twitch-soft-bot/pkg/models"
)

// vars:
// $user, $mychannel, $targetname

func commandCom(channel string, chUser *twitch.User, request ...string) {
	// Check if chUser is broadcaster or moderator for later use.
	permission := permission(chUser)

	if permission == false && chUser.Name != "nanohard_" {
		return
	}

	// Get user data from Twitch channel name.
	// var user models.User
	// if err := db.DB.One("TwitchChannel", channel, &user); err != nil {
	// 	log.Println("commandCommand: db.DB.One()", err)
	// }

	length := len(request)
	// !command with no arguments, print help
	if length == 0 {
		say(channel, "@"+chUser.Name+" !com add <name> <input> ... or ... " +
			"!com add <+name> <input> for a mod-only command")
		say(channel, "@"+chUser.Name+" Built-in commands: !quote !counter !raffle !lurk !pray !rules !request ... " +
			"Use w/o arguments for help.")
		return
	}

	// !counter add[0] badpun[1] [message] count
	switch request[0] {
	case "add":
		name := request[1]
		var modPerm bool
		if strings.HasPrefix(name, "+") {
			name = name[1:]
			modPerm = true
		}

		// Remove first 2 elements.
		copy(request[0:], request[2:]) // Shift a[i+1:] left two indices.
		request[len(request)-2] = ""     // Erase last two elements (write zero value).
		request = request[:len(request)-2]  // Truncate slice.

		// Replace "$streamer" with streamer name.
		// for i, v := range request {
		// 	if v == "$streamer" {
		// 		request[i] = user.TwitchChannel
		// 	}
		// }

		// Only the message should be left at this point.
		message := strings.Join(request, " ")

		// rp, err := regexp.Compile("([a-z]+) ([a-z]+)")
		// if err != nil {
		// 	log.Println(err)
		// }
		// rp.ReplaceAllLiteralString(message, "$ $1") // "def abc ghi"
		// id, err := uuid.NewV4()
		// if err != nil {
		// 	log.Panic("failed to generate UUID", err)
		// }

		if err := db.DB.Save(&models.Command{
			Name: name,
			Channel: channel,
			Message: message,
			ModPerm: modPerm,
		}); err != nil {
			log.Println("command add: db.DB.Save()", err)
			say(channel, "@"+chUser.DisplayName+" Error " + err.Error())
			return
		}

		say(channel, "@"+chUser.DisplayName+" Command !" + name + " has been added.")
		return

	case "rm", "remove", "del", "delete":
		var command models.Command
		if err := db.DB.Select(q.Eq("Channel", channel), q.Eq("Name", request[1])).First(&command); err != nil {
			log.Println("command rm: db.DB.Select()", err)
			say(channel, "@"+chUser.DisplayName+" Command does not exist")
			return
		}

		if err := db.DB.DeleteStruct(&command); err != nil {
			log.Println("command rm: db.DB.DeleteStruct()", err)
			say(channel, "@"+chUser.DisplayName+" Error")
			return
		}

		say(channel, "@"+chUser.DisplayName+" Command "+command.Name+" has been removed.")
	}
}

// func printCommands(channel string) {
// 	var user models.User
// 	if err := db.DB.One("TwitchChannel", channel, &user); err != nil {
// 		log.Println("printCommands() user db.DB.One()", err)
// 	}
//
// 	var commands []models.Command
// 	if err := db.DB.Select(q.Eq("UserID", user.ID)).Find(&commands); err != nil {
// 		log.Println("printCommands() commands db.DB.Select", err)
// 	}
//
// 	message := "Commands: !clip"
// 	if user.PublicEnabled {
// 		message = message + " !sr"
// 	}
// 	for _, command := range commands {
// 		message = message+" !"+command.Name
// 	}
// 	say(channel, message)
// }
