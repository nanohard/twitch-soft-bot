package models

type Command struct {
	ID      int    `storm:"id,increment"`
	Name    string
	Channel string
	Message string
	ModPerm bool
}
