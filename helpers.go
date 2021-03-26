package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/mail"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gempir/go-twitch-irc/v2"
	"github.com/google/go-github/v32/github"

	"github.com/nanohard/twitch-soft-bot/pkg/db"
	"github.com/nanohard/twitch-soft-bot/pkg/models"

	"golang.org/x/oauth2"
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
		log.Println(channel, "helpers: getUser() db.DB.One()", err)
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
	ircClient.Say(channel, msg)
}


func createIssue(channel string, chUser *twitch.User, args ...string) {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB")},
	)
	tc := oauth2.NewClient(ctx, ts)

	git := github.NewClient(tc)

	var title string
	body := strings.Join(args, " ")
	state := "open"
	labels := []string{"twitch request"}
	if len(body) > 60 {
		title = body[:60]
	} else {
		title = body
	}

	issue := github.IssueRequest{
		Title:     &title,
		Body:      &body,
		Labels:    &labels,
		Assignee:  nil,
		State:     &state,
		Milestone: nil,
		Assignees: nil,
	}

	iss, _, err := git.Issues.Create(ctx, "nanohard", "twitch-soft-bot", &issue)
	if err != nil {
		say(channel, "Error " + err.Error())
		log.Println(channel, "createIssue()", err.Error())
		return
	}
	num := strconv.Itoa(iss.GetNumber())
	say(channel, "@"+chUser.DisplayName + " Issue #"+ num + " has been created")
}


func sendEmail(toAddr string, subject string, msg string) {
	from := mail.Address{Address: os.Getenv("BOT_EMAIL")}
	to := mail.Address{Address: toAddr}

	header := make(map[string]string)
	header["From"] = from.String()
	header["To"] = to.String()
	header["Subject"] = subject
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = "text/plain; charset=\"utf-8\""
	header["Content-Transfer-Encoding"] = "base64"

	message := ""
	for k, v := range header {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + base64.StdEncoding.EncodeToString([]byte(msg))

	// Set up authentication information.
	auth := smtp.PlainAuth(
		"",
		os.Getenv("SES_USER"),
		os.Getenv("SES_PASS"),
		"email-smtp.us-east-1.amazonaws.com",
	)
	// Connect to the server, authenticate, set the sender and recipient,
	// and send the email all in one step.
	err := smtp.SendMail(
		"email-smtp.us-east-1.amazonaws.com:587",
		auth,
		from.Address,
		[]string{to.Address},
		[]byte(message),
	)
	if err != nil {
		log.Println("smtp.SendMail():", err)
	}
}


func writeChannels() {
	// Write channels to txt file.
	f, err := os.Create("channels.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	for _, word := range allChannels {
		_, err := f.WriteString(word + "\n")
		if err != nil {
			log.Fatal(err)
		}
	}
}

func lurkReturn(user string) string {
	var s string
	switch r := random(0, 4); r {
	case 0:
		s = user + " is back like they never left!"
	case 1:
		s = user + " is back. Congratulate them on a successful fap!"
	case 2:
		s = "I'm done with " + user + ". You can have back what's left of them."
	case 3:
		s = user + " is all sweaty from their lurk. Nice"
	}
	return s
}
