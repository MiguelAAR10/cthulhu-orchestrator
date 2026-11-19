package cli

import (
	"bytes"
	"runtime/debug"
	"testing"
)

func TestVersionLine(t *testing.T) {
	vcs := &debug.BuildInfo{Settings: []debug.BuildSetting{
		{Key: "vcs", Value: "git"}, {Key: "vcs.revision", Value: "abc"}, {Key: "vcs.modified", Value: "true"},
	}}
	cases := []struct {
		bi   *debug.BuildInfo
		ok   bool
		want string
	}{
		{vcs, true, "1.0.0 revision=abc modified=true"},
		{nil, false, "1.0.0 revision=unknown modified=unknown"},
		{&debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "-trimpath", Value: "true"}}}, true, "1.0.0 revision=unknown modified=unknown"},
	}
	for _, c := range cases {
		if got := versionLine(c.bi, c.ok); got != c.want {
			t.Errorf("versionLine = %q, want %q", got, c.want)
		}
	}
}

func TestVersionCommand(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Run([]string{"version"}, nil, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if got := out.String(); got != "1.0.0 revision=unknown modified=unknown\n" {
		t.Fatalf("stdout %q", got)
	}
}
