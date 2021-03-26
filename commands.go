package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/asdine/storm/q"
	"github.com/gempir/go-twitch-irc/v2"

	"github.com/nanohard/twitch-soft-bot/pkg/db"
	"github.com/nanohard/twitch-soft-bot/pkg/models"
)


func commandWOTD(channel string, chUser *twitch.User, args ...string) {
	if permission(chUser) {
		switch l := len(args); {
		case l == 0:
			say(channel, "Type one word to replace \"clap\" for 24 hours. Ex: !wotd hug")
		case l > 1:
			say(channel, "Only one word allowed. Try again.")
		case l == 1:
			wotd[channel] = args[0]
			wotdTimer[channel] = time.Now()
			say(channel, "Word has been changed to: "+wotd[channel])
		}
	}
}


func commandLurk(channel string, chUser *twitch.User, args ...string) {
	if broadcaster(chUser) && len(args) == 0 {
		say(channel, "If you're lurking then who is driving the bus?!")
	} else if permission(chUser) && len(args) > 0 {
		lurkMessage[channel] = " " + strings.Join(args, " ")
		c := models.Channel{}
		if err := db.DB.Select(q.Eq("Name", channel)).First(&c); err != nil {
			log.Println("commandLurk() db.Find()", err)
			say(channel, "Error")
			return
		}
		if err := db.DB.UpdateField(&models.Channel{ID: c.ID}, "Lurk", lurkMessage[channel]); err != nil {
			log.Println("commandLurk() db.UpdateField()", err)
			say(channel, "Error")
			return
		}
	} else {
		lurkList[chUser.DisplayName] = channel
		say(channel, chUser.DisplayName+lurkMessage[channel])
	}
}


func commandRequest(channel string, chUser *twitch.User, args ...string) {
	// Help.
	if len(args) == 0 {
		say(channel, "@"+chUser.DisplayName+" Usage: !request this is your feature request or bug!")
		return
	}

	createIssue(channel, chUser, args...)
}


func commandUpdate(channel string, chUser *twitch.User, args ...string) {
	if chUser.Name != "nanohard_" {
		return
	}
	var channels []models.Channel

	if err := db.DB.All(&channels); err != nil {
		log.Println(channel, "commandUpdate() db.Get()", err.Error())
		say(channel, "commandUpdate() db.Get() "+err.Error())
		return
	}

	for _, v := range channels {
		v.Updates = append(v.Updates, strings.Join(args, " "))
		if err := db.DB.Save(&v); err != nil {
			log.Println(channel, "commandUpdate() db.Save()", err.Error())
			say(channel, "commandUpdate() db.Save() "+err.Error())
			return
		}
	}
	say(channel, "Update message sent")
}


func commandPray(channel string) {
	say(channel, "RNG, give us this day our good percentages. "+
		"Forgive us our misclicks as we forgive those who misclick in our party. "+
		"Lead us not into rage-quit but deliver us from console users. Amen.")
}


func commandDefault(chUser *twitch.User, channel string, com string, args ...string) {
	// vars: $streamer, $user, $target, $followage, $lastplaying

	permission := permission(chUser)
	mod := true
	if !permission && chUser.Name != "nanohard_" {
		mod = false
	}
	// var user models.User
	// if err := db.DB.One("TwitchChannel", channel, &user); err != nil {
	// 	log.Println("commandDefault: db.DB.One()", err)
	// }

	com = strings.Replace(com, "!", "", 1)
	var foundCommand bool

	// Commands.
	var command models.Command
	if err := db.DB.Select(q.Eq("Channel", channel), q.Eq("Name", com)).First(&command); err != nil {
		foundCommand = false
	} else {
		foundCommand = true
		message := command.Message
		// $user
		message = strings.Replace(message, "$user", chUser.DisplayName, -1)
		// $streamer
		message = strings.Replace(message, "$streamer", channel, -1)
		// $uptime (requires Helix API)
		// if strings.Contains(message, "$uptime") {
		// 	params := &helix.StreamsParams{UserLogins: []string{channel}}
		// 	stream, err := twitchHelix.GetStreams(params)
		// 	if err != nil {
		// 		log.Println("command twitchHelix.GetStreams()", err)
		// 		return
		// 	}
		// 	if len(stream.Data.Streams) > 0 {
		// 		uptime := time.Now().UTC().Sub(stream.Data.Streams[0].StartedAt)
		// 		uptimeStr := uptime.String()
		// 		msg := strings.SplitAfter(uptimeStr, "m")
		// 		message = strings.Replace(message, "$uptime", msg[0], -1)
		// 	}
		// }
		// $followage
		if strings.Contains(message, "$followage") {
			resp, err := http.Get("https://api.crunchprank.net/twitch/followage/" + channel + "/" + chUser.Name)
			if err != nil {
				log.Println(channel, "$follwage crunchprank.net error", err)
				return
			}
			defer resp.Body.Close()

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Println(channel, "ioutil.ReadAll() $followage", err)
				return
			}
			followage := string(body)
			message = strings.Replace(message, "$followage", followage, -1)
		}
		// $lastplaying
		if strings.Contains(message, "$lastplaying") {
			if !strings.Contains(message, "$target") {
				say(channel, "Error: $lastplaying needs $target")
				return
			}
			// We are assuming this is only being used for !so, so the first arg should be a Twitch username.
			target := args[0]
			target = strings.ToLower(target)
			target = strings.Replace(target, "@", "", 1)
			resp, err := http.Get("https://api.crunchprank.net/twitch/game/" + target)
			if err != nil {
				log.Println(channel, "$lastplaying crunchprank.net error", err)
				return
			}
			defer resp.Body.Close()

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Println(channel, "ioutil.ReadAll() $lastplaying", err)
				return
			}
			lastPlaying := string(body)
			message = strings.Replace(message, "$lastplaying", lastPlaying, -1)
		}

		// $target
		if len(args) > 0 {
			message = strings.Replace(message, "$target", args[0], -1)
		}

		if (command.ModPerm && permission) || !command.ModPerm {
			say(channel, message)
		}
	}

	if !foundCommand {
		// Counters.
		var counter models.Counter
		if err := db.DB.Select(q.Eq("Channel", channel), q.Eq("Name", com)).First(&counter); err != nil {
			return
		}
		message := counter.Message
		// +
		if len(args) == 0 && (mod || !counter.ModOnly) && time.Now().Sub(counters[counter.ID]) > time.Second*10 {
			if err := db.DB.UpdateField(&models.Counter{ID: counter.ID}, "Count", counter.Count+1); err != nil {
				log.Println(channel, "counter +", err)
				return
			}
			message = strings.Replace(message, "*", strconv.Itoa(counter.Count+1), 1)
			// Cooldown to avoid mistakes
			counters[counter.ID] = time.Now()
			say(channel, message)
			// -
		} else if len(args) == 1 && args[0] == "-" && mod {
			if err := db.DB.UpdateField(&models.Counter{ID: counter.ID}, "Count", counter.Count-1); err != nil {
				log.Println(channel, "counter +", err)
				return
			}
			message = strings.Replace(message, "*", strconv.Itoa(counter.Count-1), 1)
			say(channel, message)
			// No permission to +, just print.
		} else {
			message = strings.Replace(message, "*", strconv.Itoa(counter.Count), 1)
			say(channel, message)
		}
	}
}
