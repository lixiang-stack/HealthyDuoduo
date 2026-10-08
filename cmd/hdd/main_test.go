package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunStub(t *testing.T) {
	for _, cmd := range []string{"ingest", "reparse", "list", "show"} {
		var buf bytes.Buffer
		if code := run(&buf, []string{cmd}); code != 1 {
			t.Errorf("run(%q) exit = %d, want 1", cmd, code)
		}
		if !strings.Contains(buf.String(), "not implemented") {
			t.Errorf("run(%q) output = %q, want contains \"not implemented\"", cmd, buf.String())
		}
	}
}

func TestRunNoArgs(t *testing.T) {
	var buf bytes.Buffer
	if code := run(&buf, nil); code != 2 {
		t.Errorf("run() exit = %d, want 2", code)
	}
	if !strings.Contains(buf.String(), "usage:") {
		t.Errorf("run() output = %q, want usage hint", buf.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var buf bytes.Buffer
	if code := run(&buf, []string{"bogus"}); code != 2 {
		t.Errorf("run(bogus) exit = %d, want 2", code)
	}
	if !strings.Contains(buf.String(), `unknown command "bogus"`) {
		t.Errorf("run(bogus) output = %q, want unknown hint", buf.String())
	}
}
