package models

import (
	"time"
)

type Channel struct {
	ID         int     `storm:"id,increment"`
	Name       string  `storm:"unique"`
	Quotes     []string
	Updates    []string
	Lurk       string
	Wotd       string
	WotdTimer  time.Time
}
