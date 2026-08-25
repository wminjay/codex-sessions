package main

import (
	"bytes"
	"testing"
)

func TestParseYolo(t *testing.T) {
	var output bytes.Buffer
	options, err := parseOptions([]string{"-yolo", "--provider", "custom"}, &output)
	if err != nil {
		t.Fatal(err)
	}
	if !options.yolo || options.provider != "custom" {
		t.Fatalf("unexpected options: %#v", options)
	}
}

func TestParseRejectsUnknownProvider(t *testing.T) {
	var output bytes.Buffer
	if _, err := parseOptions([]string{"--provider", "other"}, &output); err == nil {
		t.Fatal("expected an error")
	}
}
