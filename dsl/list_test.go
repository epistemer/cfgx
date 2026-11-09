package dsl

import (
	"reflect"
	"testing"

	"github.com/epistemer/cfgx/engine"
)

// A list default's text reads back as the same items with the engine's
// splitter, whatever the items hold.
func TestCSVJoinRoundTrip(t *testing.T) {
	for _, items := range [][]string{
		{"a", "b"}, {""}, {"", ""}, {"a,b", `say "hi"`}, {" lead", "trail "}, {"line\nbreak"}, {"x=y"},
	} {
		got, err := engine.SplitCSV(csvJoin(items))
		if err != nil || !reflect.DeepEqual(got, items) {
			t.Errorf("%q: written %q, read back %q, %v", items, csvJoin(items), got, err)
		}
	}
	if got := mapText(map[string]string{"b": "2", "a": "x,y"}); got != `"a=x,y",b=2` {
		t.Errorf("mapText = %q", got)
	}
}
