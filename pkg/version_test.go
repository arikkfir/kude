package kude

import (
	"testing"
)

func TestGetVersion(t *testing.T) {
	if GetVersion().String() != version.String() {
		t.Errorf("Expected version to be 0.0.0-dev+unknown, got %s", GetVersion())
	}
}
