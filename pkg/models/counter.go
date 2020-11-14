package models

type Counter struct {
	ID   int    `storm:"id,increment"`
	Name string `storm:"index"`
	Channel string `storm:"index"`
	Message string
	ModOnly bool
	Count int
}
