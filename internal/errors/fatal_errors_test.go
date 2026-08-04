package errors

import "testing"

func TestIsFatalKeyError(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		message  string
		expected bool
	}{
		{"429 status is fatal", 429, "", true},
		{"401 status is fatal", 401, "", true},
		{"403 status is fatal", 403, "", true},
		{"500 status is not fatal", 500, "upstream error", false},
		{"200 status is not fatal", 200, "", false},
		{"400 status is not fatal by code", 400, "bad request", false},
		{"quota message is fatal", 500, "Insufficient quota", true},
		{"usage limit message is fatal", 500, "Weekly usage limit reached", true},
		{"rate limit message is fatal", 500, "rate limit exceeded", true},
		{"exhausted message is fatal", 500, "resource has been exhausted", true},
		{"invalid api key message is fatal", 500, "Invalid API key provided", true},
		{"generic 500 message is not fatal", 500, "Router.Unavailable", false},
		{"empty message with 500 is not fatal", 500, "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsFatalKeyError(tc.status, tc.message)
			if got != tc.expected {
				t.Fatalf("IsFatalKeyError(%d, %q) = %v, want %v", tc.status, tc.message, got, tc.expected)
			}
		})
	}
}
