package cfgx_test

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/epistemer/cfgx"
)

// A Secret prints as a mask however it's formatted, logged or encoded.
// Reveal gives the value to the code that needs it.
func ExampleSecret() {
	token := cfgx.NewSecret("hunter2")

	fmt.Println(token)
	fmt.Printf("%v %s %q %#v\n", token, token, token, token)
	b, _ := json.Marshal(map[string]any{"token": token})
	fmt.Println(string(b))
	slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	})).Info("connecting", "token", token)

	fmt.Println(token.Reveal())
	// Output:
	// ******
	// ****** ****** ****** ******
	// {"token":"******"}
	// level=INFO msg=connecting token=******
	// hunter2
}

// Equal checks a candidate, such as a token from a request, in constant
// time. An unset Secret equals nothing.
func ExampleSecret_Equal() {
	token := cfgx.NewSecret("hunter2")
	fmt.Println(token.Equal("hunter2"), token.Equal("hunter3"))

	var unset cfgx.Secret
	fmt.Println(unset.IsSet(), unset.Equal(""))
	// Output:
	// true false
	// false false
}

// Loading reports every problem at once, as Errors.
func ExampleErrors() {
	var err error = cfgx.Errors{
		{Key: "database.name", Message: "is required"},
		{Key: "server.port", Message: `"http" is not an integer`, Source: "env ZP_SERVER_PORT"},
	}
	fmt.Println(err)
	// Output:
	// invalid configuration (2 problems)
	//   database.name: is required
	//   server.port: "http" is not an integer (env ZP_SERVER_PORT)
}
