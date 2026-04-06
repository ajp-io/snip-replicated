package slug

import (
	"regexp"

	gonanoid "github.com/matoous/go-nanoid/v2"
)

const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var pattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,64}$`)

// Generate returns a random 6-character URL-safe slug.
func Generate() (string, error) {
	return gonanoid.Generate(alphabet, 6)
}

// Validate returns true if s is a valid custom slug.
func Validate(s string) bool {
	return pattern.MatchString(s)
}
