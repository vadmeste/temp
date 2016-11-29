package main

import (
	"testing"

	"github.com/surullabs/lint"
)

func TestLint(t *testing.T) {
	if err := lint.Default.Check("./..."); err != nil {
		t.Fatal("lint failures: %v", err)
	}
}
