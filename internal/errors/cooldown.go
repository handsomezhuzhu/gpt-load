package errors

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// maxCooldown caps the parsed cooldown to avoid absurd values.
const maxCooldown = 30 * 24 * time.Hour

var durationPartRe = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*(months?|weeks?|days?|hours?|hrs?|h\b|minutes?|mins?|m\b|seconds?|secs?|s\b|ms\b)`)

// statusCodeInMessageRe matches the "[status 429]" prefix that channel
// validation errors carry (e.g. openai_channel.go: "[status %d] %s").
var statusCodeInMessageRe = regexp.MustCompile(`\[status\s+(\d{3})\]`)

// ParseStatusCodeFromError extracts an HTTP status code embedded in an error
// message produced by channel validation ("[status 429] ..."). Returns 0 when
// no status code is present.
func ParseStatusCodeFromError(message string) int {
	if message == "" {
		return 0
	}
	m := statusCodeInMessageRe.FindStringSubmatch(message)
	if len(m) != 2 {
		return 0
	}
	code, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return code
}

var resetKeywords = []string{
	"resets in ",
	"reset in ",
	"retry after ",
	"retry in ",
	"try again in ",
	"try again after ",
	"please retry in ",
	"request in ",
}

// ParseCooldownDuration extracts a cooldown duration from an upstream error
// message. It handles many different rate-limit / quota-exhausted formats:
//
//	"Weekly usage limit reached. Resets in 5 days."
//	"Monthly usage limit reached. Resets in 26 days."
//	"5-hour usage limit reached. Resets in 3hr 10min."
//	"Rate limit reached. Please try again in 20s."
//	"Retry after 30 seconds."
//	"Too many requests, request in 1 hour 5 minutes."
//
// Returns 0 when no cooldown can be parsed (caller decides the fallback).
func ParseCooldownDuration(message string) time.Duration {
	if message == "" {
		return 0
	}
	lower := strings.ToLower(message)

	for _, kw := range resetKeywords {
		if idx := strings.Index(lower, kw); idx >= 0 {
			if d := parseDurationParts(lower[idx+len(kw):]); d > 0 {
				return d
			}
		}
	}

	// Fallback: scan the whole message for duration parts ("in 5 days" without
	// the explicit keyword, e.g. "quota will reset after 12 hours").
	if d := parseDurationParts(lower); d > 0 {
		return d
	}
	return 0
}

// parseDurationParts parses one or more "N unit" groups from a string and
// sums them. Handles compound durations like "2hr 60min" or "3 days 2 hours".
func parseDurationParts(s string) time.Duration {
	var total time.Duration
	matched := false
	for _, m := range durationPartRe.FindAllStringSubmatch(s, -1) {
		val, err := strconv.ParseFloat(m[1], 64)
		if err != nil || val < 0 {
			continue
		}
		unit := strings.TrimSpace(m[2])
		var d time.Duration
		switch unit {
		case "ms":
			d = time.Duration(val * float64(time.Millisecond))
		case "s", "sec", "secs", "second", "seconds":
			d = time.Duration(val * float64(time.Second))
		case "m", "min", "mins", "minute", "minutes":
			d = time.Duration(val * float64(time.Minute))
		case "h", "hr", "hrs", "hour", "hours":
			d = time.Duration(val * float64(time.Hour))
		case "d", "day", "days":
			d = time.Duration(val * 24 * float64(time.Hour))
		case "w", "week", "weeks":
			d = time.Duration(val * 7 * 24 * float64(time.Hour))
		case "month", "months":
			d = time.Duration(val * 30 * 24 * float64(time.Hour))
		default:
			continue
		}
		if d > 0 {
			matched = true
			total += d
		}
	}
	if !matched {
		return 0
	}
	if total > maxCooldown {
		return maxCooldown
	}
	return total
}
