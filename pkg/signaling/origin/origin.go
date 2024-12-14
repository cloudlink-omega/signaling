package origin

import (
	"regexp"
	"strings"
)

// Compile the allowed hosts into regex patterns.
func CompilePatterns(allowedHosts []string) []*regexp.Regexp {
	var patterns []*regexp.Regexp
	for _, host := range allowedHosts {

		// Convert wildcard '*' to regex pattern.
		// Escape other special regex characters.
		pattern := "^" + regexp.QuoteMeta(host) + "$"
		pattern = strings.ReplaceAll(pattern, `\*`, `.*`)

		// Compile the pattern and add it to the list.
		regex, err := regexp.Compile(pattern)
		if err == nil {
			patterns = append(patterns, regex)
		}
	}
	return patterns
}

// Check if a given URL matches any of the allowed host patterns.
func IsAllowed(host string, origins []*regexp.Regexp) bool {
	// Test the host against each compiled regex pattern.
	for _, pattern := range origins {
		if pattern.MatchString(host) {
			return true
		}
	}
	return false
}
