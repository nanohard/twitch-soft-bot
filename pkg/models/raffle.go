package models

type Raffle struct {
	ID            string `storm:"id"`  // raffle name, primary key
	MaxTickets    int
	Users         []string             // chUser.Displayname
}
