# templ-component-opts

This project is designed to help generate code to simplify passing options to a templ component.

## Installation

You can install `templ-component-opts` using the `go get` command:

```sh
$ go get -u github.com/codekoala/templ-component-opts
```

This will download and install the executable in your `$GOPATH/bin` directory.

## Usage

To use `templ-component-opts`, simply create a struct with the various options that you may want to pass to a templ component and include the `//templ:component-opts` directive:

https://github.com/codekoala/templ-component-opts/blob/main/example/view/component/book/book.go

Run the `templ-component-opts` tool pointing to the project directory:

```sh
$ templ-component-opts .
```

This will produce

## License

This project is licensed under the MIT License. See the [LICENSE](./LICENSE) file for details.