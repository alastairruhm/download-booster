package proxy

import (
	"testing"
)

func TestGetFileName(t *testing.T) {
	var shouldSuccess = []struct {
		input    string // input
		expected string // expected result
	}{
		{"http://test.com/test.pdf", "test.pdf"},
		{"http://test.com/test.pdf?q=1&q=2", "test.pdf"},
	}
	for _, ts := range shouldSuccess {
		if actual := GetFileName(ts.input); actual != ts.expected {
			t.Fatalf("should be %s, but is:%s\n", ts.input, ts.expected, actual)
		}
	}
}
