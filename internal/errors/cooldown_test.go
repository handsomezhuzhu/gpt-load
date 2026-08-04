package errors

import (
	"testing"
	"time"
)

func TestParseCooldownDuration(t *testing.T) {
	cases := []struct {
		name    string
		message string
		want    time.Duration
	}{
		{"weekly usage limit days", "Weekly usage limit reached. Resets in 5 days. To continue using this model now, enable usage from your available balance: https://x", 5 * 24 * time.Hour},
		{"monthly usage limit 28 days", "Monthly usage limit reached. Resets in 28 days.", 28 * 24 * time.Hour},
		{"monthly usage limit 26 days", "Monthly usage limit reached. Resets in 26 days.", 26 * 24 * time.Hour},
		{"5-hour usage limit compound", "5-hour usage limit reached. Resets in 3hr 10min.", 3*time.Hour + 10*time.Minute},
		{"5-hour usage limit minutes", "5-hour usage limit reached. Resets in 6min.", 6 * time.Minute},
		{"compound hrs+min no space", "Resets in 2hr 60min", 3 * time.Hour},
		{"try again in seconds", "Rate limit reached for default model in organization. Please try again in 20s.", 20 * time.Second},
		{"retry after seconds", "Retry after 30 seconds.", 30 * time.Second},
		{"request in compound", "Too many requests, request in 1 hour 5 minutes.", 65 * time.Minute},
		{"weeks", "Quota resets in 2 weeks.", 14 * 24 * time.Hour},
		{"hours decimal", "try again in 1.5 hours", 90 * time.Minute},
		{"ms", "rate limited, retry in 500ms", 500 * time.Millisecond},
		{"no duration", "The latest version of this model is only available hosted in China", 0},
		{"generic upstream error", `{"type":"Router.Unavailable","modelID":"deepseek-v4-flash"}`, 0},
		{"empty", "", 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseCooldownDuration(tc.message)
			if got != tc.want {
				t.Fatalf("ParseCooldownDuration(%q) = %v, want %v", tc.message, got, tc.want)
			}
		})
	}
}

func TestParseCooldownDurationMaxCap(t *testing.T) {
	got := ParseCooldownDuration("Resets in 100 days.")
	if got != maxCooldown {
		t.Fatalf("expected cap at %v, got %v", maxCooldown, got)
	}
}
