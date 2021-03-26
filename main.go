package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/asdine/storm"
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
	endChannel = make(map[string]chan struct{})
	// done       sync.WaitGroup

	channelMod = make(map[string]bool)
	allChannels []string
	// channelsMap = make(map[string][]models.Channel)

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
		commandLurk(channel, chUser, args...)
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
	case "wotd", "wod", "word":
		commandWOTD(channel, chUser, args...)
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


func main() {
	// Start callback API for TwitchAPI
	var wait time.Duration
	flag.DurationVar(&wait, "graceful-timeout", time.Second*15, "the duration for which the server gracefully wait for existing connections to finish - e.g. 15s or 1m")
	flag.Parse()

	// Check if bot is modded
	// for botBan()
	ircClient.OnUserStateMessage(func(message twitch.UserStateMessage) {
		// If bot is present and is not mod
		if _, ok := message.User.Badges["moderator"]; ok && message.User.Name == "og_softbot" {
			channelMod[message.Channel] = true
		} else {
			channelMod[message.Channel] = false
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
			if v, ok := lurkList[message.User.DisplayName]; ok && v == message.Channel {
				say(message.Channel, lurkReturn(message.User.DisplayName))
				delete(lurkList, message.User.DisplayName)
			}
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
		// Temp fix to add Lurk.
		if v.Lurk != "" {
			lurkMessage[v.Name] = v.Lurk
		} else {
			lurkMessage[v.Name] = " is putting in the real homie love with a lurk"
		}
		// Temp fix for quotes.
		// for i, qt := range v.Quotes {
		// 	if idx := strings.Index(qt, ")"); idx != -1 {
		// 		v.Quotes[i] = qt[:idx+1]
		// 	}
		// }
		// if err := db.DB.Update(&channels[c]); err != nil {
		// 	log.Println("quote fix: db.Update()", err)
		// }
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
		time.Sleep(time.Second * 3)
		for {
			// Compare live channels to all channels and depart offline channels
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
					// Disregard if we already know the channel is live.
					if _, exist := endChannel[name]; exist {
						continue
					}

					endChannel[name] = make(chan struct{})
					// done.Add(1)

					ircClient.Join(name)
					run(name)
					log.Println("joined", name)

				} else if _, exist := endChannel[name]; exist {
					// Depart offline channels and stop processes from run().
					ircClient.Depart(name)
					close(endChannel[name])
					delete(endChannel, name)
					log.Println("departed", name)
				}
				// Twitch allows 800 requests per minute.
				// This will allow us up to 600 channels per minute
				time.Sleep(time.Millisecond * 100)
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


func botBan(channel string, message string, chUser *twitch.User) {
	if strings.Contains(message, "http") && strings.Contains(message, "big") && strings.Contains(message, "follows") {
		if !channelMod[channel] {
			say(channel, "I could have banned that user if I was a mod")
			return
		}
		say(channel, "/ban " + chUser.Name)
	}
}


func run(channel string)  {
	var c models.Channel
	if err := db.DB.One("Name", channel, &c); err != nil {
		log.Println("run() db.One()", err)
	}

	go func() {
		t := time.NewTicker(time.Minute * 73)
		for {
			select {
			case <-endChannel[channel]:
				t.Stop()
				return
			case <-t.C:
				if len(c.Updates) > 0 {
					say(channel, "@"+channel+" "+c.Updates[0])
					c.Updates = c.Updates[1:]
					if err := db.DB.UpdateField(&models.Channel{ID: c.ID}, "Updates", c.Updates); err != nil {
						log.Println(c.Name, "db.UpdateField() Channel.Updates", err.Error())
					}
				}
			}
		}
	}()

	go func() {
		t := time.NewTicker(time.Minute*60)
		for {
			select {
			case <-endChannel[channel]:
				t.Stop()
				return
			case <-t.C:
				// Display quotes if there are 11+.
				if len(c.Quotes) > 10 {
					r := random(0, len(c.Quotes))
					say(c.Name, "Quote #" + strconv.Itoa(r+1) + " " + c.Quotes[r])
				}
			}
		}
	}()
}
