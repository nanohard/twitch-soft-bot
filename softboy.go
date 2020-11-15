package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gempir/go-twitch-irc/v2"

	"github.com/nanohard/twitch-soft-bot/pkg/db"
	"github.com/nanohard/twitch-soft-bot/pkg/models"
)

var (
	incomingChat = make(map[string][]string)
	sayChat = make(map[string][]string)
	randomChat = make(map[string][]string)
	ogChat = make(map[string][]string)
	ogQuestion = make(map[string][]string)

	incomingChatTime = make (map[string]time.Time)
	randomChatTime = make(map[string]time.Time)
	interval = time.Minute * time.Duration(20)

	clapTime = make(map[string]time.Time)
	clapInterval = time.Minute * time.Duration(20)
	clapBlacklist []string

	ogStr = regexp.MustCompile(`og`)
)

func init() {
	// Read in JSON files on init and every hour.
	go func() {
		read()
		time.Sleep(time.Hour * time.Duration(1))
	}()
}


func commandRules(channel string, args ...string) {
	if len(args) == 0 {
		r := random(0, len(sayChat["rules"]))
		say(channel, sayChat["rules"][r])
		return
	}

	if n, err := strconv.Atoi(args[0]); err == nil {
		if n <= len(sayChat["rules"]) && n > 0 {
			say(channel, sayChat["rules"][n-1])
		} else {
			say(channel, "There are only " + strconv.Itoa(len(sayChat["rules"])) + " rules")
		}
		return
	} else {
		say(channel, "That's not a number, genius")
	}

}


func commandClap(channel string, chUser *twitch.User, args ...string) {
	if len(args) == 0 {
		say(channel, "@"+chUser.DisplayName+" Usage: !clap @username")
		return
	}

	var target string
	if strings.HasSuffix(args[0], "s") {
		target = args[0] + "'"
	} else {
		target = args[0] + "'s"
	}
	target = strings.TrimPrefix(target, "@")

	say(channel, chUser.DisplayName + " claps " + target + " cheeks")
}


// Join or depart channel.
func commandSoftBoy(channel string, chUser *twitch.User, args ...string) {
	if len(args) == 0 {
		say(channel,"@" + chUser.DisplayName + " I can `join` or `leave`")
		return
	}
	switch args[0] {
	case "join":
		client.Join(chUser.Name)
		c := models.Channel{
			Name: chUser.Name,
		}
		if err := db.DB.Save(&c); err != nil {
			log.Println(channel, "softboy join: db.Save()", err)
			say(channel, "@"+chUser.DisplayName+" Error "+err.Error())
			return
		}
		// Set channel vars to avoid map lookup error.
		incomingChatTime[chUser.Name] = time.Time{}
		randomChatTime[chUser.Name] = time.Time{}
		say(channel, "The OG Soft Boy is in your channel homie. Welcome to the hug gang.")
	case "leave":
		if !broadcaster(chUser) {
			return
		}
		client.Depart(channel)
		if err := db.DB.DeleteStruct(models.Channel{Name: chUser.Name}); err != nil {
			log.Println(channel, "softboy leave: db.DeleteStruct()", err)
			say(channel, "@"+chUser.DisplayName+" Error "+err.Error())
			return
		}
		delete(incomingChatTime, channel)
		delete(randomChatTime, channel)
		say(channel, "You're too hard to be a Soft Boy. Open-mouth kisses. I'm out.")
	}

}


// Read in JSON files. Hard-coded.
func read() {
	// Input files.
	files := []string{"entering", "leaving"}
	for i := 0; i < len(files); i++ {
		name := files[i]
		data, err := ioutil.ReadFile("./pkg/dictionary/" + name + "Incoming.json")
		if err != nil {
			log.Println("read() input", err)
			return
		}

		var tmp []string
		err = json.Unmarshal(data, &tmp)
		if err != nil {
			log.Println("read() input json.Unmarshal", err)
			return
		}
		incomingChat[name] = tmp
	}

	// Output files.
	files = []string{"entering", "leaving", "rules"}
	for i := 0; i < len(files); i++ {
		name := files[i]
		data, err := ioutil.ReadFile("./pkg/dictionary/" + name + "Dict.json")
		if err != nil {
			log.Println("read() output", err)
			return
		}

		var tmp []string
		err = json.Unmarshal(data, &tmp)
		if err != nil {
			log.Println("read() output json.Unmarshal", err)
			return
		}
		sayChat[name] = tmp
	}

	data, err := ioutil.ReadFile("./pkg/dictionary/randomChat.json")
	if err != nil {
		log.Println("read() ioutil.Readfile", err)
		return
	}
	err = json.Unmarshal(data, &randomChat)
	if err != nil {
		log.Println("read() end json.Unmarshal", err)
		return
	}

	data, err = ioutil.ReadFile("./pkg/dictionary/ogChat.json")
	if err != nil {
		log.Println("read() ioutil.Readfile", err)
		return
	}
	err = json.Unmarshal(data, &ogChat)
	if err != nil {
		log.Println("read() end json.Unmarshal", err)
		return
	}

	data, err = ioutil.ReadFile("./pkg/dictionary/ogQuestion.json")
	if err != nil {
		log.Println("read() ioutil.Readfile", err)
		return
	}
	err = json.Unmarshal(data, &ogQuestion)
	if err != nil {
		log.Println("read() end json.Unmarshal", err)
		return
	}

	data, err = ioutil.ReadFile("./pkg/dictionary/clapBlacklist.json")
	if err != nil {
		log.Println("read() ioutil.Readfile", err)
		return
	}
	err = json.Unmarshal(data, &clapBlacklist)
	if err != nil {
		log.Println("read() end json.Unmarshal", err)
		return
	}
}


// Parse chat and respond accordingly.
func chat(channel string, message string, chUser *twitch.User) {
	msg := strings.ToLower(message)

	// Talking to OG directly.
	if ogStr.MatchString(msg) {
		count := strings.Count(msg, "og")
		msgCut := msg
		for i := 0; i < count; i++ {
			ind := strings.Index(msgCut, "og")
			afterWord := []rune(msgCut)
			var a rune
			beforeWord := []rune(msgCut)
			var b rune
			if ind+3 <= len(msgCut) {
				afterWord = afterWord[ind+2 : ind+3]
				a = afterWord[0]
			}
			if ind > 0 {
				beforeWord = beforeWord[ind-1 : ind]
				b = beforeWord[0]
			}
			if (unicode.IsLetter(a) || unicode.IsLetter(b)) || (unicode.IsPunct(a) && unicode.IsPunct(b)) { // limit false positives
				if len(msgCut) > ind+3 {
					msgCut = msgCut[ind+3:]
					continue
				}
			}
			if strings.Contains(msg, "tell") || strings.Contains(msg, "say") {
				var sayPerm bool
				if _, ok := chUser.Badges["broadcaster"]; ok {
					sayPerm = true
				}
				if _, ok := chUser.Badges["moderator"]; ok {
					sayPerm = true
				}
				if _, ok := chUser.Badges["subscriber"]; ok {
					sayPerm = true
				}
				if chUser.Name == "nanohard_" {
					sayPerm = true
				}
				if sayPerm == false {
					return
				}
				indTell := strings.Index(msg, "tell")
				indSay := strings.Index(msg, "say")
				var indCom int
				var comLen int
				if indTell == -1 && indSay > -1 {
					indCom = indSay
					comLen = 3
				} else if indSay == -1 && indTell > -1 {
					indCom = indTell
					comLen = 4
				} else {
					break
				}

				var com string
				indOG := strings.Index(msg, "og")
				if indOG > 0 && indCom == 0 {
					com = msg[comLen+1:indOG]
				} else if indOG == 0 && indCom == 3 {
					com = msg[comLen+4:]
				}
				say(channel, com)
				return
			}

			// ogQuestion or ogChat?
			if strings.HasSuffix(msg, "?") {
				// ogQuestion. "og" and "?" match, so now match the keyword.
				for k, v := range ogQuestion {
					if strings.Contains(msg, k) {
						ind := strings.Index(msg, k)
						afterWord := []rune(msg)
						var a rune
						beforeWord := []rune(msg)
						var b rune
						if len(msg) >= ind+len(k) {
							if ind+len(k)+1 <= len(msg) {
								afterWord = afterWord[ind+len(k) : ind+len(k)+1]
								a = afterWord[0]
							}
							if ind > 0 {
								beforeWord = beforeWord[ind-1 : ind]
								b = beforeWord[0]
							}
						}
						if (unicode.IsLetter(a) || unicode.IsLetter(b)) || (unicode.IsPunct(a) && unicode.IsPunct(b)) { // limit false positives
							continue
						}
						log.Println(channel, "Recognized word [", k, "] in msg", msg)
						r := random(0, len(ogQuestion[k]))
						say(channel, v[r])
						return
					}
				}
				r := random(0, len(ogQuestion["will"]))
				say(channel, ogQuestion["will"][r])
				return
			}
			// ogChat. "og" matches, so now match the keyword.
			for k, v := range ogChat {
				if strings.Contains(msg, k) {
					ind := strings.Index(msg, k)
					afterWord := []rune(msg)
					var a rune
					beforeWord := []rune(msg)
					var b rune
					if len(msg) >= ind+len(k) {
						if ind+len(k)+1 <= len(msg) {
							afterWord = afterWord[ind+len(k) : ind+len(k)+1]
							a = afterWord[0]
						}
						if ind > 0 {
							beforeWord = beforeWord[ind-1 : ind]
							b = beforeWord[0]
						}
					}
					if (unicode.IsLetter(a) || unicode.IsLetter(b)) || (unicode.IsPunct(a) && unicode.IsPunct(b)) { // limit false positives
						continue
					}
					log.Println(channel, "Recognized word [", k, "] in msg", msg)
					r := random(0, len(ogChat[k]))
					say(channel, v[r])
					return
				}
			}
		}
	}

	// Random chat key/value.
	lastRandom := time.Now().Sub(randomChatTime[channel])
	if lastRandom > interval {
		for k, v := range randomChat {
			if strings.Contains(msg, k) {
				ind := strings.Index(msg, k)
				afterWord := []rune(msg)
				var a rune
				beforeWord := []rune(msg)
				var b rune
				if len(msg) >= ind+len(k) {
					if ind + len(k)+1 <= len(msg) {
						afterWord = afterWord[ind+len(k) : ind+len(k)+1]
						a = afterWord[0]
					}
					if ind > 0 {
						beforeWord = beforeWord[ind-1 : ind]
						b = beforeWord[0]
					}
				}
				if (unicode.IsLetter(a) || unicode.IsLetter(b)) || (unicode.IsPunct(a) && unicode.IsPunct(b)) { // limit false positives
					continue
				}
				log.Println(channel, "Recognized word [", k, "] in msg", msg)
				r := random(0, len(randomChat[k]))
				say(channel, v[r])
				randomChatTime[channel] = time.Now()
				return
			}
		}
	}

	// Incoming chat (hi, bye).
	lastIncoming := time.Now().Sub(incomingChatTime[channel])
	if lastIncoming > interval {
		// k == filename, v == index
		for k, v := range incomingChat {
			for i := 0; i < len(v); i++ {
				if strings.Contains(msg, v[i]) {
					ind := strings.Index(msg, v[i])
					afterWord := []rune(msg)
					var a rune
					beforeWord := []rune(msg)
					var b rune
					if len(msg) >= ind+len(v[i]) {
						if ind+len(v[i])+1 <= len(msg) {
							afterWord = afterWord[ind+len(v[i]) : ind+len(v[i])+1]
							a = afterWord[0]
						}
						if ind > 0 {
							beforeWord = beforeWord[ind - 1 : ind]
							b = beforeWord[0]
						}
					}
					if (unicode.IsLetter(a) || unicode.IsLetter(b)) || (unicode.IsPunct(a) && unicode.IsPunct(b)) { // limit false positives
						continue
					}
					r := random(0, len(sayChat[k]))
					say(channel, sayChat[k][r])
					incomingChatTime[channel] = time.Now()
					return
				}
			}
		}
	}

	// I'd love to clap ...
	lastClap := time.Now().Sub(clapTime[channel])
	if lastClap > clapInterval {
		words := []string{
			" some ",
			" an ",
			" a ",
		}

		for _, v := range clapBlacklist {
			if strings.Contains(msg, v) {
				return
			}
		}
		for _, word := range words {
			if i := strings.LastIndex(msg, word); i != -1 {
				s := strings.Split(msg[i+1:], " ")
				if len(s) > 5 || len(s) == 2 && s[1] == "" {
					continue
				}
				say(channel, "I'd love to clap " + strings.Join(s, " "))
				clapTime[channel] = time.Now()
				return
			}
		}
	}
}
