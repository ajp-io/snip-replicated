package slug_test

import (
	"testing"

	"github.com/ajp-io/snips-replicated/internal/slug"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerate_Length(t *testing.T) {
	s, err := slug.Generate()
	require.NoError(t, err)
	assert.Len(t, s, 6)
}

func TestGenerate_URLSafeChars(t *testing.T) {
	for i := 0; i < 100; i++ {
		s, err := slug.Generate()
		require.NoError(t, err)
		assert.Regexp(t, `^[a-zA-Z0-9]+$`, s)
	}
}

func TestGenerate_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		s, err := slug.Generate()
		require.NoError(t, err)
		assert.False(t, seen[s], "duplicate slug: %s", s)
		seen[s] = true
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"abc123", true},
		{"my-link", true},
		{"my_link", true},
		{"ABC", true},
		{"ab", false},
		{"a b", false},
		{"abc!", false},
		{"", false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.valid, slug.Validate(tt.input), "input: %q", tt.input)
	}
}
