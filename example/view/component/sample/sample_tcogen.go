// Code generated by templ-component-opts; DO NOT EDIT.
// This file contains functions and methods for use with Opts in templ components.
package sample

import "strconv"

type Opt func(*Opts)

func With(opts ...Opt) *Opts {
	out := &Opts{Happy: true}
	out.With(opts...)
	return out
}
func (o *Opts) With(opts ...Opt) *Opts {
	for _, opt := range opts {
		opt(o)
	}
	return o
}

func Name(in string) Opt {
	return func(opts *Opts) {
		opts.Name = in
	}
}

func Age(in int64) Opt {
	return func(opts *Opts) {
		opts.Age = in
	}
}

func (o *Opts) AgeStr() string {
	return strconv.FormatInt(o.Age, 10)
}

func Happy(in bool) Opt {
	return func(opts *Opts) {
		opts.Happy = in
	}
}

func (o *Opts) HappyStr() string {
	return strconv.FormatBool(o.Happy)
}
