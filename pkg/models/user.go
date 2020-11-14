package models

type User struct {
	ID      string `storm:"id"` // Twitch chUser.DisplayName
	Tickets int                  // # of tickets
	Raffles map[string]int
}
