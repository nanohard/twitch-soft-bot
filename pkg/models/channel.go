package models

type Channel struct {
	ID   int    `storm:"id,increment"`
	Name string `storm:"unique"`
	Quotes []string
}
