package redact

import (
	"encoding/json"
	"testing"
)

func TestMasksKeepReportsRedacted(t *testing.T) {
	cases := []struct{ got, want string }{
		{Account("123456789"), "123***789"},
		{Account("abc"), "a*c"},
		{Text("测试用户"), "测**户"},
		{Text("ab"), "a*"},
		{Text(""), "unknown"},
		{MAC("02:00:00:00:00:01"), "02:00:xx:xx:xx:01"},
		{MAC("not-a-mac"), "redacted"},
		{MAC(""), "unknown"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}

func TestNormalizeCannotStartANewRecord(t *testing.T) {
	if got := Normalize("service\nstate=injected\x1b[31m\r\t x"); got != "service state=injected [31m x" {
		t.Fatalf("Normalize = %q", got)
	}
	if got := Field("<nil>", "unknown"); got != "unknown" {
		t.Fatalf("Field(<nil>) = %q", got)
	}
}

func TestUntypedValues(t *testing.T) {
	if AnyUnix(json.Number("1")) != Unix(1) || AnyUnix(json.Number("1.5")) != "unknown" {
		t.Fatal("AnyUnix mishandled a JSON number")
	}
	if AnyUnix("1") != "unknown" || AnyUnix(float64(0)) != "unknown" || AnyUnix(float64(1)) == "unknown" {
		t.Fatal("AnyUnix accepted a non-numeric or non-positive timestamp")
	}
	if Presence(nil) != "none" || Presence("") != "none" || Presence("2001:db8::1") != "present" {
		t.Fatal("Presence misclassified a value")
	}
}
