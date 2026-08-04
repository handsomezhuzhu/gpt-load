package errors

import (
	"net/http"
	"strings"
)

// fatalErrorSubstrings contains error message patterns that indicate
// a key is permanently (or long-term) unusable: quota exhausted, rate
// limited, authentication failed, etc.
var fatalErrorSubstrings = []string{
	"rate limit",
	"rate_limit",
	"usage limit",
	"quota",
	"exhausted",
	"invalid api key",
	"authentication failed",
	"insufficient quota",
	"insufficient_quota",
	"billing",
}

// IsFatalKeyError determines whether an upstream error means the key itself
// is unusable (quota exhausted, rate limited, forbidden, unauthorized...).
// Fatal errors should blacklist the key immediately instead of waiting for
// the accumulated failure count to reach the blacklist threshold, and the
// key should not be retried again in the same request cycle.
func IsFatalKeyError(statusCode int, errorMessage string) bool {
	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests:
		return true
	}

	lower := strings.ToLower(errorMessage)
	for _, pattern := range fatalErrorSubstrings {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}
