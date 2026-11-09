package engine

import (
	"net/netip"
	"testing"
)

func TestParseText(t *testing.T) {
	var p netip.Prefix
	if err := ParseText("10.0.0.0/8", &p); err != nil || p.String() != "10.0.0.0/8" || !NonZero(p) {
		t.Errorf("ParseText = %v, %v", p, err)
	}
	if err := ParseText("", &p); err != nil || NonZero(p) {
		t.Errorf("ParseText(\"\") = %v, %v; want the zero value", p, err)
	}
	if err := ParseText("10.0.0.0", &p); err == nil || err.Error() != `"10.0.0.0" isn't valid: netip.ParsePrefix("10.0.0.0"): no '/'` {
		t.Errorf("ParseText(bad) = %v", err)
	}
}
