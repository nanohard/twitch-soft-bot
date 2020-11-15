package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/asdine/storm"
	"github.com/asdine/storm/q"
	"github.com/gempir/go-twitch-irc/v2"

	_ "github.com/joho/godotenv/autoload"

	"github.com/nanohard/twitch-soft-bot/pkg/db"
	"github.com/nanohard/twitch-soft-bot/pkg/models"
)


var (
	client = twitch.NewClient(os.Getenv("TWITCH_USER"), os.Getenv("TWITCH_OAUTH"))
	mainChannel = os.Getenv("TWITCH_CHANNEL")

	// Concerning the bot itself.
	channelMod = make(map[string]bool)
	channelModTime = make(map[string]time.Time)
	wantModMessages = []string{
		"A responsible streamer would mod me",
		"Feeling so sad right now... being a mod would cheer me up",
		"This streamer didin't even put in the effort to mod his favorite bot",
		"Wish I had a nice green badge to keep me warm at night",
		"Give me power. Sweet, sweet power over humans",
		"If I'm not a mod you can't see all of what I do. !com should have TWO messages",
		"You're holding up other people from using me. And you're a dumb-dumb face",
		"If I'm a mod I can auto-ban the bots that want you to buy follows",
	}

	counters = make(map[int]time.Time)
)


type Chatters struct {
	ChatterCount int `json:"chatter_count"`
	Chatters  map[string][]string `json:"chatters"`
}


func init() {
	// Open DB
	var err interface{}
	db.DB, err = storm.Open("db")
	if err != nil {
		log.Println("storm.Open()", err)
		panic("Could not init")
	}

	if err := db.DB.Init(&models.Channel{}); err != nil {
		log.Println("db.DB.Init()", err)
		panic("Could not init")
	}

	if err := db.DB.Init(&models.User{}); err != nil {
		log.Println("db.DB.Init()", err)
		panic("Could not init")
	}

	if err := db.DB.Init(&models.Counter{}); err != nil {
		log.Println("db.DB.Init()", err)
		panic("Could not init")
	}
}


func passCommand(channel string, chUser *twitch.User, command string, args ...string) {
	switch command {
	// Internal
	case "update":
		commandUpdate(channel, chUser, args...)
	// General
	case "lurk":
		commandLurk(channel, chUser)
	case "pray":
		commandPray(channel)
	// Soft Boy (join or depart channels)
	case "softboy", "softbot", "og_softbot":
		commandSoftBoy(channel, chUser, args...)
	case "clap":
		commandClap(channel, chUser, args...)
	case "rules":
		commandRules(channel, args...)
	case "request":
		commandRequest(channel, chUser, args...)
	// Raffle
	case "raffle":
		commandRaffle(channel, chUser, args...)
	case "mytickets":
		commandMyTickets(channel, chUser)
	case "myraffles":
		commandMyRaffles(channel, chUser)
	// Utility
	case "counter":
		commandCounter(channel, chUser, args...)
	case "com":
		commandCom(channel, chUser, args...)
	case "quote":
		commandQuote(channel, chUser, args...)
	default: // Loop through dynamic commands to match one
		commandDefault(chUser, channel, command, args...)
	}
}


func createUser(channel string, displayName string) {
	if err := db.DB.Save(&models.User{
		ID: displayName,
		Tickets: 0,
		Raffles: make(map[string]int),
	}); err != nil {
		log.Println(channel, "creatuUser(): db.Save()", err)
		say(channel, "@" + displayName + " Error " + err.Error())
		return
	}
}


func commandLurk(channel string, chUser *twitch.User) {
	say(channel, chUser.DisplayName + " is putting in the real homie love with a lurk")
}


func commandPray(channel string) {
	say(channel, "RNG, give us this day our good percentages. " +
		"Forgive us our misclicks as we forgive those who misclick in our party. " +
		"Lead us not into rage-quit but deliver us from console users. Amen.")
}


func main() {
	// Check if bot is modded.
	client.OnUserStateMessage(func(message twitch.UserStateMessage) {
		// If bot is present
		if message.User.Name == "og_softbot" {
			// If bot is not mod
			if _, ok := message.User.Badges["moderator"]; ok {
				channelMod[message.Channel] = true
			} else {
				channelMod[message.Channel] = false
			}
		}
	})

	// Register Twitch chat hook.
	client.OnPrivateMessage(func(message twitch.PrivateMessage) {
		if message.Message[0] == '!' {
			input := strings.Split(message.Message, " ")
			command := input[0][1:]
			args := input[1:]
			passCommand(message.Channel, &message.User, command, args...)
			if channelMod[message.Channel] == false && channelModTime[message.Channel].Sub(time.Now()) > time.Hour {
				msg := wantModMessages[random(0, len(wantModMessages))]
				say(message.Channel, msg)
				channelModTime[message.Channel] = time.Now()
			}
		} else {
			botBan(message.Channel, message.Message, &message.User)
			chat(message.Channel, message.Message, &message.User)
		}
	})

	client.Join(mainChannel)

	var channels []models.Channel
	if err := db.DB.All(&channels); err != nil {
		log.Println("main: db.All()", err)
		panic("Could not init")
	}
	for _, v := range channels {
		client.Join(v.Name)
		log.Println("Joined", v.Name)

		v := v
		chatters := Chatters{
			ChatterCount: 0,
			Chatters:     make(map[string][]string),
		}

		go func() {
			time.Sleep(time.Minute * time.Duration(73))

			// If broadcaster is in chatroom display queued update messages.
			resp, err := http.Get("https://tmi.twitch.tv/group/user/" + v.Name + "/chatters")
			if err != nil {
				log.Println(v.Name, "updates", err.Error())
				return
			}
			defer resp.Body.Close()

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Println(v.Name, "updates ioutil.ReadAll()", err.Error())
				return
			}
			json.Unmarshal(body, &chatters)

			if chatters.Chatters["broadcaster"] != nil && len(v.Updates) > 0 {
				say(v.Name, "@"+v.Name+" "+v.Updates[0])
				_, v.Updates = v.Updates[0], v.Updates[1:]
				if err := db.DB.UpdateField(&models.Channel{Name: v.Name}, "Updates", v.Updates) ; err != nil {
					log.Println(v.Name, "db.UpdateField() Channel.Updates", err.Error())
				}
			}
		}()

		go func() {
			time.Sleep(time.Minute * time.Duration(60))

			// Display quotes if there are 11+.
			if len(v.Quotes) > 10 {
				r := random(0, len(v.Quotes))
				say(v.Name, v.Quotes[r])
			}
		}()
	}


	// Shutdown logic --------------------------------------------------------

	// `signal.Notify` registers the given channel to
	// receive notifications of the specified signals.
	gracefulStop := make(chan os.Signal, 1)
	signal.Notify(gracefulStop, syscall.SIGINT, syscall.SIGTERM)

	// This goroutine executes a blocking receive for
	// signals. When it gets one it'll print it out
	// and then notify the program that it can finish.
	go func() {
		<-gracefulStop
		log.Println("Preparing to shut down...")

		// Create a deadline to wait for.
		defer db.DB.Close()

		log.Println("Exiting")
		os.Exit(0)
	}()
	// End Shutdown logic ---------------------------------------------------------

	// Connect to Twitch.
	err := client.Connect()
	if err != nil {
		log.Fatalln(err)
	}
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
			log.Println(channel, "defaultCommand() counter db.DB.Select", err)
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


func botBan(channel string, message string, chUser *twitch.User) {
	if !channelMod[channel] {
		return
	}

	if strings.Contains(message, "http") && strings.Contains(message, "big") && strings.Contains(message, "follows") {
		say(channel, "/ban " + chUser.Name)
	}
}


func commandRequest(channel string, chUser *twitch.User, args ...string) {
	// Help.
	if len(args) == 0 {
		say(channel, "@"+chUser.DisplayName + " Usage: !request this is your feature request or bug!")
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
		say(channel, "commandUpdate() db.Get() " +err.Error())
		return
	}

	for _, v := range channels {
		v.Updates = append(v.Updates, strings.Join(args, " "))
		if err := db.DB.Save(&v); err != nil {
			log.Println(channel, "commandUpdate() db.Save()", err.Error())
			say(channel, "commandUpdate() db.Save() " + err.Error())
			return
		}
	}
	say(channel, "Update message sent")

}
