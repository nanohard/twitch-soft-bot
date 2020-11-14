package main

import (
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
	wantModMessages = []string{
		"A responsible streamer would mod me",
		"Feeling so sad right now... being a mod would cheer me up",
		"This streamer didin't even put in the effort to mod his favorite bot",
		"Wish I had a nice green badge to keep me warm at night",
		"Give me power. Sweet, sweet power over humans",
		"If I'm not modded you can't see all of what I do. !com should have TWO messages. " +
			"And you're holding up other people from using me. And you're a dumb-dumb face",
	}

	counters = make(map[int]time.Time)

	update = "I've been updated! If modded I now autoban bots that mention buying followers"
)


func init() {
	// Open DB
	var err interface{}
	db.DB, err = storm.Open("db")
	if err != nil {
		log.Println("storm.Open()", err)
	}

	if err := db.DB.Init(&models.Channel{}); err != nil {
		log.Println("db.DB.Init()", err)
	}

	if err := db.DB.Init(&models.User{}); err != nil {
		log.Println("db.DB.Init()", err)
	}

	if err := db.DB.Init(&models.Counter{}); err != nil {
		log.Println("db.DB.Init()", err)
	}
}


func passCommand(channel string, chUser *twitch.User, command string, args ...string) {
	switch command {
	// General
	case "lurk":
		commandLurk(channel, chUser)
	case "pray":
		commandPray(channel)
	// Soft Boy (join or depart channels)
	case "softboy":
		commandSoftBoy(channel, chUser, args...)
	case "clap":
		commandClap(channel, chUser, args...)
	case "rules":
		commandRules(channel, args...)
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
		log.Println("creatuUser(): db.Save()", err)
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
		// If robo_nano is present
		if message.User.Name == "og_softbot" {
			// If robo_nano is not mod
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
			if channelMod[message.Channel] == false {
				go func() {
					r := random(30, 60)
					time.Sleep(time.Minute * time.Duration(r))
					// Check one last time for mod status so a false positive
					// does not get through.
					if channelMod[message.Channel] == false {
						msg := wantModMessages[random(0, len(wantModMessages))]
						say(message.Channel, msg)
					}
				}()
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
	}
	for _, v := range channels {
		client.Join(v.Name)
		log.Println("Joined", v.Name)
		say(v.Name, update)
		v := v
		go func() {
			time.Sleep(time.Minute * time.Duration(60))
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
	// log.Println(command)
	// log.Println(args)

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
		log.Println("defaultCommand() command db.DB.Select", err)
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
				log.Println("$follwage crunchprank.net error", err)
				return
			}
			defer resp.Body.Close()

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Println("ioutil.ReadAll() $followage", err)
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
				log.Println("$lastplaying crunchprank.net error", err)
				return
			}
			defer resp.Body.Close()

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Println("ioutil.ReadAll() $lastplaying", err)
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
			log.Println("defaultCommand() counter db.DB.Select", err)
			return
		}
		message := counter.Message
		// +
		if len(args) == 0 && (mod || !counter.ModOnly) && time.Now().Sub(counters[counter.ID]) > time.Second*10 {
			if err := db.DB.UpdateField(&models.Counter{ID: counter.ID}, "Count", counter.Count+1); err != nil {
				log.Println("counter +", err)
			}
			message = strings.Replace(message, "*", strconv.Itoa(counter.Count+1), 1)
			// Cooldown to avoid mistakes
			counters[counter.ID] = time.Now()
			say(channel, message)
			// -
		} else if len(args) == 1 && args[0] == "-" && mod {
			if err := db.DB.UpdateField(&models.Counter{ID: counter.ID}, "Count", counter.Count-1); err != nil {
				log.Println("counter +", err)
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
