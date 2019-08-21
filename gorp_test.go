package main

import (
	"regexp"
	"testing"
)

func Test_handleLine(t *testing.T) {
	str := "33333"
	pattern := "12\\d4"

	opts := meta{
		folderMode:  true,
		invertMatch: false,
		abs:         false,
		colorFunc:   func(t string) string { return t },
	}

	re, _ := regexp.Compile(pattern)
	res := handleLine(str, re, 5, opts)

	if res != "1234" {
		t.Error(res)
	}
}
