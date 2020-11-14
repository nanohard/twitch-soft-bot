package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/gempir/go-twitch-irc/v2"

	"github.com/nanohard/twitch-soft-bot/pkg/db"
	"github.com/nanohard/twitch-soft-bot/pkg/models"
)

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890")

func randomString() string {
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	min := 10
	max := 30
	n := r1.Intn(max-min) + min

	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}


func random(min int, max int) int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(max-min) + min
}


func remove(s []string, i int) []string {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}


func after(s string, sub string) string {
	// Get substring after a string.
	pos := strings.LastIndex(s, sub)
	if pos == -1 {
		return ""
	}
	adjustedPos := pos + len(sub)
	if adjustedPos >= len(s) {
		return ""
	}
	return s[adjustedPos:]
}


// Check if a string is in slice.
func contains(s []string, l string) bool {
	for _, v := range s {
		if v == l {
			return true
		}
	}
	return false
}


// Used for cron job to get channels that robo_nano has Mod status.
// Can also be used to consume other APIs.
func getJson(url string, target interface{}) error {
	client := &http.Client{Timeout: 10 * time.Second}
	r, err := client.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	return json.NewDecoder(r.Body).Decode(target)
}


func getUser(channel string) models.User {
	// Get user data from Twitch channel name.
	var user models.User
	if err := db.DB.One("TwitchChannel", channel, &user); err != nil {
		log.Println("helpers: getUser() db.DB.One()", err)
	}
	return user
}


func permission(chUser *twitch.User) bool {
	var moderator bool
	var broadcaster bool

	if _, ok := chUser.Badges["broadcaster"]; ok {
		broadcaster = true
	}
	if _, ok := chUser.Badges["moderator"]; ok {
		moderator = true
	}
	if broadcaster || moderator {
		return true
	}
	return false
}


func broadcaster(chUser *twitch.User) bool {
	if _, ok := chUser.Badges["broadcaster"]; ok {
		return true
	}
	return false
}


func say(channel string, msg string) {
	client.Say(channel, msg)
}
