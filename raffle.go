package main

import (
	"log"
	"strconv"
	"strings"

	"github.com/asdine/storm"
	"github.com/asdine/storm/q"
	"github.com/gempir/go-twitch-irc/v2"

	_ "github.com/joho/godotenv/autoload"

	"github.com/nanohard/twitch-soft-bot/pkg/db"
	"github.com/nanohard/twitch-soft-bot/pkg/models"
)

func init() {
	if err := db.DB.Init(&models.Raffle{}); err != nil {
		log.Fatalln("db.DB.Init()", err)
	}
}


/*
// *done* !raffle create <name> [max tickets allowed]
// *done* !raffle remove/delete/rem/rm/del <name>
// *done* !raffle pick <name> == pick winner and delete raffle
// *done* !raffle give @username #tickets
//
// *done* !raffle <list> == list all raffle names
// *done* !raffle enter <name> [# tickets]
// *done* !mytickets == [number of tickets]
// *done* !myraffles [raffle - number of tickets entered ...]
//
*/


func commandRaffle(channel string, chUser *twitch.User, args ...string) {
	if len(args) == 0 {
		say(channel,"@" + chUser.DisplayName + " Options: create, delete, pick, give, list, enter")
		return
	}
	switch args[0] {
	// case "":
	// 	say("@" + chUser.DisplayName + " Options: create, delete, pick, give, list, enter")
	// 	return
	case "create":
		if !broadcaster(chUser) {
			return
		}
		var name string
		var maxTickets int

		if len(args) == 2 {
			name = strings.ToLower(args[1])
		} else {
			say(channel,"@"+chUser.DisplayName+" Usage: !raffle create <name> [max # tickets]")
			return
		}

		if len(args) == 3 {
			var err error
			maxTickets, err = strconv.Atoi(args[2])
			if err != nil {
				say(channel,"@" + chUser.DisplayName + " Usage: !raffle create <name> [max # tickets]")
				return
			}
		} else {
			maxTickets = 1  // Max tickets 1 per user is a sane default
		}

		if err := db.DB.Save(&models.Raffle{
			ID:         name,
			MaxTickets: maxTickets,
		}); err != nil {
			log.Println(channel, "raffle create: db.Save()", err)
			say(channel, "@"+chUser.DisplayName+" Error "+err.Error())
			return
		}
	case "delete", "remove", "rem", "rm", "del":
		if !broadcaster(chUser) {
			return
		}
		if len(args) < 2 {
			say(channel,"@" + chUser.DisplayName + " Usage: !raffle delete <name>")
			return
		}
		var raffle models.Raffle
		if err := db.DB.Select(q.Eq("ID", strings.ToLower(args[1]))).First(&raffle); err != nil {
			log.Println(channel, "raffle delete: db.Select()", err)
			say(channel, "@"+chUser.DisplayName+" Error "+err.Error())
			return
		}
		if err := db.DB.DeleteStruct(&raffle); err != nil {
			log.Println(channel, "raffle delete: db.DeleteStruct()", err)
			say(channel, "@"+chUser.DisplayName+" Error "+err.Error())
			return
		}

		// Delete map from all users
		var users []models.User
		if err := db.DB.All(&users); err != nil {
			log.Println(channel, "raffle delete: db.All()", err)
			say(channel, "@" + chUser.DisplayName + " Error " + err.Error())
			return
		}

		for i := 0; i < len(users); i++ {
			delete(users[i].Raffles, raffle.ID)
			if err := db.DB.Update(users[i]); err != nil {
				log.Println(channel, "raffle delete: db.Update()", err)
				say(channel, "@" + chUser.DisplayName + " Error " + err.Error())
				return
			}
		}
	case "pick":
		if !broadcaster(chUser) {
			return
		}
		if len(args) != 2 {
			say(channel,"@" + chUser.DisplayName + " Usage: !raffle pick <name>")
			return
		}
		var raffle models.Raffle
		if err := db.DB.Select(q.Eq("ID", strings.ToLower(args[1]))).First(&raffle); err != nil {
			log.Println(channel, "raffle pick: db.Select()", err)
			say(channel, "@" + chUser.DisplayName + " Error " + err.Error())
			return
		}
		r := random(0, len(raffle.Users))
		winner := raffle.Users[r]
		say(channel,"@" + chUser.DisplayName + " The winner is " + winner + "! Suck it nerds!")
		remove(raffle.Users, r)

		if err := db.DB.Save(&raffle); err != nil {
			log.Println(channel, "raffle pick: db.Save()", err)
			say(channel, "@" + chUser.DisplayName + " Error " + err.Error())
			return
		}
	case "give":
		if !broadcaster(chUser) {
			return
		}
		if len(args) < 3 {
			say(channel,"@" + chUser.DisplayName + " Usage: !raffle give <@name> <#>")
			return
		}

		userRaw := strings.TrimPrefix(args[1], "@")

		// Get or create User
		var user models.User
		if err := db.DB.Select(q.Eq("ID", userRaw)).First(&user); err != nil {
			if err == storm.ErrNotFound {
				createUser(channel, userRaw)
				if err := db.DB.Select(q.Eq("ID", userRaw)).First(&user); err != nil {
					log.Println(channel, "raffle give: db.Select()", err)
					say(channel,"@" + chUser.DisplayName + " Error " + err.Error())
					return
				}
			} else {
				log.Println(channel, "raffle give: db.Select()", err)
				say(channel, "@" + chUser.DisplayName + " Error " + err.Error())
				return
			}
		}

		// Give tickets
		tickets, err := strconv.Atoi(args[2])
		if err != nil {
			log.Println(channel, "raffle give: strconv.Atoi()", err)
			say(channel, "@" + chUser.DisplayName + " Error " + err.Error())
			return
		}

		ticketsNow := user.Tickets + tickets
		if ticketsNow >= 0 {
			if err := db.DB.UpdateField(&models.User{ID: user.ID}, "Tickets", ticketsNow); err != nil {
				log.Println(channel, "raffle give: db.UpdateField()", err)
				say(channel, "@" + chUser.DisplayName + " Error " + err.Error())
				return
			}
		}
	case "list":
		var raffles []models.Raffle
		if err := db.DB.All(&raffles); err != nil {
			log.Println(channel, "raffle list: db.All()", err)
			say(channel, "@" + chUser.DisplayName + " Error " + err.Error())
			return
		}

		var raffleList string
		for i := 0; i < len(raffles); i++ {
			raffleList += raffles[i].ID + ", "
			// Output the last bit that will be less than or equal to 10
			if i + 1 == len(raffles) {
				say(channel, "@" + chUser.DisplayName + " Raffle List " + raffleList)
			// Output every 10 names
			} else if i%10 == 0 {
				say(channel, "@" + chUser.DisplayName + " Raffle List " + raffleList)
				raffleList = ""
			}
		}
	case "enter":
		if len(args) < 2 {
			say(channel, "@" + chUser.DisplayName + " Usage: !raffle enter <raffleName> [# tickets]")
			return
		}

		// Get or create User
		var user models.User
		if err := db.DB.Select(q.Eq("ID", chUser.DisplayName)).First(&user); err != nil {
			if err == storm.ErrNotFound {
				createUser(channel, chUser.DisplayName)
				say(channel, "@" + chUser.DisplayName + " You have no tickets. Get fucked.")
			} else {
				log.Println(channel, "raffle enter: db.Select()", err)
				say(channel, "@" + chUser.DisplayName + " Error " + err.Error())
			}
			return
		}
		if user.Tickets == 0 {
			say(channel, "@" + chUser.DisplayName + " You have no tickets. Get fucked.")
			return
		}

		// Get Raffle
		var raffle models.Raffle
		if err := db.DB.Select(q.Eq("ID", strings.ToLower(args[1]))).First(&raffle); err != nil {
			if err == storm.ErrNotFound {
				say(channel, "@" + chUser.DisplayName + " Spell much? That raffle doesn't exist.")
			} else {
				log.Println(channel, "raffle enter: db.Select()", err)
				say(channel, "@" + chUser.DisplayName + " Error " + err.Error())
			}
			return
		}

		// Adjust user ticket amount
		var ticketsEntered int
		var err error
		if len(args) == 3 {
			ticketsEntered, err = strconv.Atoi(args[2])
			if err != nil {
				log.Println(channel, "raffle give: strconv.Atoi()", err)
				say(channel, "@" + chUser.DisplayName + " Error " + err.Error())
				return
			}
		} else {
			ticketsEntered = 1
		}

		ticketsNow := user.Tickets - ticketsEntered
		if ticketsNow >= 0 {
			if err := db.DB.UpdateField(&models.User{ID: user.ID}, "Tickets", ticketsNow); err != nil {
				log.Println(channel, "raffle enter: db.UpdateField()", err)
				say(channel, "@" + chUser.DisplayName + " Error " + err.Error())
				return
			}
		} else {
			say(channel, "@" + chUser.DisplayName + " You tried using more tickets than you have. " +
				"Nice try asshole. Deleting all tickets.")
			return
		}

		// Append User to Raffle list i times where i = ticketsEntered
		for i := 0; i < ticketsEntered; i++ {
			raffle.Users = append(raffle.Users, chUser.DisplayName)
		}
		if err := db.DB.Save(&raffle); err != nil {
			log.Println(channel, "raffle enter: db.Save()", err)
			say(channel, "@" + chUser.DisplayName + " Error " + err.Error())
			return
		}

		// Update !myraffles
		user.Raffles[raffle.ID] += ticketsEntered
		if err := db.DB.Update(user); err != nil {
			log.Println(channel, "raffle enter: db.Update()", err)
			say(channel, "@" + chUser.DisplayName + " Error " + err.Error())
			return
		}
	}
}


func commandMyTickets(channel string, chUser *twitch.User)  {
	// Get or create User
	var user models.User
	if err := db.DB.Select(q.Eq("ID", chUser.DisplayName)).First(&user); err != nil {
		if err == storm.ErrNotFound {
			createUser(channel, chUser.DisplayName)
			if err := db.DB.Select(q.Eq("ID", chUser.DisplayName)).First(&user); err != nil {
				log.Println(channel, "commandMyTickets: db.Select()", err)
				say(channel, "@" + chUser.DisplayName + " Error " + err.Error())
				return
			}
			say(channel, "@" + chUser.DisplayName + " You have no tickets. Get fucked.")
			return
		} else {
			log.Println(channel, "commandMyTickets: db.Select()", err)
			say(channel, "@" + chUser.DisplayName + " Error " + err.Error())
			return
		}
	}
	say(channel, "@" + chUser.DisplayName + " You have "+ strconv.Itoa(user.Tickets) +" tickets.")
}


func commandMyRaffles(channel string, chUser *twitch.User) {
	// Get or create User
	var user models.User
	if err := db.DB.Select(q.Eq("ID", chUser.DisplayName)).First(&user); err != nil {
		if err == storm.ErrNotFound {
			createUser(channel, chUser.DisplayName)
			if err := db.DB.Select(q.Eq("ID", chUser.DisplayName)).First(&user); err != nil {
				log.Println(channel, "commandMyRaffles: db.Select()", err)
				say(channel, "@" + chUser.DisplayName + " Error " + err.Error())
				return
			}
		} else {
			log.Println(channel, "commandMyRaffles: db.Select()", err)
			say(channel, "@" + chUser.DisplayName + " Error " + err.Error())
			return
		}
	}

	var raffleList string
	for k, v := range user.Raffles {
		raffleList += k + ":" + strconv.Itoa(v) + " "
	}

	say(channel, "@"+chUser.DisplayName+" "+raffleList)
}
