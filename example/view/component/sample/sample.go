package sample

// Opts provides a series of options for the Sample templ component.
//
//templ:component-opts
type Opts struct {
	Name  string
	Age   int64
	Happy bool `default:"true"`
}
