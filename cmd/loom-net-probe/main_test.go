package main

import (
	"bytes"
	"testing"
)

func TestParseProbeArgsSupportsSetupOnlyAndHelp(t *testing.T) {
	var output bytes.Buffer
	options, err := parseProbeArgs([]string{"--setup-only"}, &output)
	if err != nil || !options.setupOnly {
		t.Fatalf("parseProbeArgs() = %#v, %v", options, err)
	}
	if _, err := parseProbeArgs([]string{"--help"}, &output); err == nil {
		t.Fatal("help must stop normal probe execution")
	}
	if output.Len() == 0 {
		t.Fatal("help output missing")
	}
}

func TestValidContextTokenProjection(t *testing.T) {
	for _, test := range []struct {
		budget int
		count  int
		valid  bool
	}{
		{budget: 2_048, count: 7, valid: true},
		{budget: 0, count: 0, valid: false},
		{budget: 2_048, count: 0, valid: false},
		{budget: 2_048, count: 2_049, valid: false},
	} {
		if got := validContextTokenProjection(test.budget, test.count); got != test.valid {
			t.Fatalf("budget=%d count=%d valid=%t", test.budget, test.count, got)
		}
	}
}
