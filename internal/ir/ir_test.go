package ir

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	def := "localhost"
	in := &App{
		Version: Version, Name: "zp", Desc: "ZaynPay backend", EnvPrefix: "ZP",
		Groups: []Group{{
			Name: "database", Desc: "Database connection", Pos: Pos{"schema.go", 3},
			Keys: []Key{{Name: "host", Desc: "Host", Kind: KindString, Default: &def, Pos: Pos{"schema.go", 4}}},
		}},
		Commands: []Command{{Name: "Serve", Uses: []string{"database"}, Pos: Pos{"schema.go", 9}}},
		Pos:      Pos{"schema.go", 7},
	}
	var buf bytes.Buffer
	if err := Encode(&buf, in); err != nil {
		t.Fatal(err)
	}
	out, err := Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("round trip changed the IR:\n in: %+v\nout: %+v", in, out)
	}
}

func TestDecodeRejectsOtherVersions(t *testing.T) {
	_, err := Decode(strings.NewReader(`{"version": 99, "name": "x"}`))
	if err == nil || !strings.Contains(err.Error(), "different cfgx versions") {
		t.Fatalf("got %v", err)
	}
}

func TestDecodeRejectsUnknownFields(t *testing.T) {
	_, err := Decode(strings.NewReader(`{"version": 1, "surprise": true}`))
	if err == nil {
		t.Fatal("want an error for an unknown field")
	}
}
