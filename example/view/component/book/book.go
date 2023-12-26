package book

import "time"

// Opts defines options for the Book templ component.
//
//templ:component-opts
type Opts struct {
	Title     string
	Author    string
	Published time.Time
	Display   bool `default:"true"`
}
