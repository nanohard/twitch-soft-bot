package main

import (
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/asdine/storm"
	"github.com/asdine/storm/q"
	"github.com/gempir/go-twitch-irc/v2"
	_ "github.com/joho/godotenv/autoload"

	"github.com/nanohard/twitch-soft-bot/pkg/db"
	"github.com/nanohard/twitch-soft-bot/pkg/models"

	"github.com/nicklaw5/helix"
)


var (
	ircClient   = twitch.NewClient(os.Getenv("TWITCH_USER"), os.Getenv("TWITCH_OAUTH"))
	helixClient = &helix.Client{}
	mainChannel = os.Getenv("TWITCH_CHANNEL")

	// Concerning the bot itself.
	channelOffline = make(map[string]chan struct{})
	done           sync.WaitGroup

	channelMod = make(map[string]bool)
	allChannels []string

	// channelModTime = make(map[string]time.Time)
	// wantModMessages = []string{
	// 	"A responsible streamer would mod me",
	// 	"Feeling so sad right now... being a mod would cheer me up",
	// 	"This streamer didin't even put in the effort to mod his favorite bot",
	// 	"Wish I had a nice green badge to keep me warm at night",
	// 	"Give me power. Sweet, sweet power over humans",
	// 	"If I'm not a mod you can't see all of what I do. !com should have TWO messages",
	// 	"You're holding up other people from using me. And you're a dumb-dumb face",
	// 	"If I'm a mod I can auto-ban the bots that want you to buy follows",
	// }

	counters = make(map[int]time.Time)
)


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
	// Start callback API for TwitchAPI
	var wait time.Duration
	flag.DurationVar(&wait, "graceful-timeout", time.Second*15, "the duration for which the server gracefully wait for existing connections to finish - e.g. 15s or 1m")
	flag.Parse()

	// Check if bot is modded.
	ircClient.OnUserStateMessage(func(message twitch.UserStateMessage) {
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
	ircClient.OnPrivateMessage(func(message twitch.PrivateMessage) {
		if message.Message[0] == '!' {
			input := strings.Split(message.Message, " ")
			command := input[0][1:]
			args := input[1:]
			passCommand(message.Channel, &message.User, command, args...)
		} else {
			botBan(message.Channel, message.Message, &message.User)
			chat(message.Channel, message.Message, &message.User)
		}
	})

	ircClient.Join(mainChannel)
	log.Println("joined main channel")

	var channels []models.Channel
	if err := db.DB.All(&channels); err != nil {
		log.Println("main: db.All()", err)
	}
	// Load global vars on program start.
	for _, v := range channels {
		allChannels = append(allChannels, v.Name)
	}
	writeChannels()  // write list of channels, for my personal use

	// Get app access token for TwitchAPI.
	// Token expires in 10 days, renew every 7 days.
	go func() {
		for {
			var err error
			helixClient, err = helix.NewClient(&helix.Options{
				ClientID:     os.Getenv("TWITCH_CLIENT_ID"),
				ClientSecret: os.Getenv("TWITCH_CLIENT_SECRET"),
			})
			if err != nil {
				log.Fatalln("Could not get twitch app access token phase 1: " + err.Error())
			}

			resp, err := helixClient.RequestAppAccessToken([]string{"user:read:email"})
			if err != nil {
				log.Fatalln("Could not get twitch app access token phase 2: " + err.Error())
			}
			// log.Printf("%+v\n", resp)

			// Set the access token on the helixClient
			helixClient.SetAppAccessToken(resp.Data.AccessToken)
			time.Sleep(time.Hour * 168)
		}
	}()

	// Get stream status (online/offline)
	go func() {
		for {
			// Comapare live channels to all channels and depart offline channels
			offlineChannels := allChannels
			for _, name := range allChannels {
				stream, err := helixClient.GetStreams(&helix.StreamsParams{
					First:      0,
					Type:       "",
					UserIDs:    nil,
					UserLogins: []string{name},
				})
				if err != nil {
					log.Println("get stream status error", err)
				}
				// Channel is live, join it and run processes.
				if len(stream.Data.Streams) > 0 {
					log.Println("channel is live, removing from offline list", name)
					// Remove channel from list of offline channels.
					for i, v := range offlineChannels {
						if name == v {
							offlineChannels = remove(offlineChannels, i)
							log.Println("removed channel from offline list", name)
						}
					}
					// Disregard if we already know the channel is live.
					if _, exist := channelOffline[name]; exist {
						log.Println("channel is already live, skipping")
						continue
					}

					channelOffline[name] = make(chan struct{})
					done.Add(1)

					ircClient.Join(name)
					run(name)
					log.Println("joined", name)
				}
				// Twitch allows 800 requests per minute.
				// This will allow us up to 600 channels per minute
				time.Sleep(time.Millisecond * 100)
			}
			log.Println(len(offlineChannels), "are offline")
			// Depart offline channels and stop processes from run().
			for _, v := range offlineChannels {
				if _, exist := channelOffline[v]; exist {
					log.Println("departing offline channel", v)
					ircClient.Depart(v)
					log.Println("departed", v)
					if _, ok := <-channelOffline[v]; ok {
						log.Println("closing processes for offline channel", v)
						close(channelOffline[v])
						delete(channelOffline, v)
						log.Println("processes closed for offline channel", v)
					}
				}
			}
			// Run every 5 minutes
			time.Sleep(time.Minute * 5)
		}
	}()




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

		// Helix connection
		// Create a deadline to wait for.
		// ctx, cancel := context.WithTimeout(context.Background(), wait)
		// defer cancel()
		// Doesn't block if no connections, but will otherwise wait
		// until the timeout deadline.
		// srv.Shutdown(ctx)

		// Local connection
		// Create a deadline to wait for.
		defer db.DB.Close()

		log.Println("Exiting")
		os.Exit(0)
	}()
	// End Shutdown logic ---------------------------------------------------------

	// Connect to Twitch.
	err := ircClient.Connect()
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
	if strings.Contains(message, "http") && strings.Contains(message, "big") && strings.Contains(message, "follows") {
		if !channelMod[channel] {
			say(channel, "I could have banned that user if I was a mod")
			return
		}
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


func run(channel string)  {
	var c models.Channel
	if err := db.DB.One("Name", channel, &c); err != nil {
		log.Println("run() db.One()", err)
	}

	go func() {
		run:
		for {
			select {
			case <-channelOffline[channel]:
				done.Done()
				break run
			default:
				time.Sleep(time.Minute * time.Duration(73))
				if len(c.Updates) > 0 {
					say(channel, "@"+channel+" "+c.Updates[0])
					_, c.Updates = c.Updates[0], c.Updates[1:]
					if err := db.DB.UpdateField(&models.Channel{ID: c.ID}, "Updates", c.Updates); err != nil {
						log.Println(c.Name, "db.UpdateField() Channel.Updates", err.Error())
					}
				}
			}
		}
	}()

	go func() {
		run:
		for {
			select {
			case <-channelOffline[channel]:
				done.Done()
				break run
			default:
				time.Sleep(time.Minute * time.Duration(60))
				// Display quotes if there are 11+.
				if len(c.Quotes) > 10 {
					r := random(0, len(c.Quotes))
					say(c.Name, c.Quotes[r])
				}
			}
		}
	}()
}
