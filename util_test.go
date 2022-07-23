libstandard

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type stringTestData struct {
	input    string
	expected string
}

type sliceTestData struct {
	input    []string
	expected []string
}

type sliceStringTestData struct {
	input    []string
	expected string
}

func TestUnescape(t *testing.T) {
	tests := []stringTestData{
		{
			input:    "This is a test",
			expected: "This is a test",
		},
		{
			input:    "",
			expected: "",
		},
		{
			input:    "This is \"a\" test",
			expected: "This is a test",
		},
		{
			input:    "This \\is a test",
			expected: "This is a test",
		},
		{
			input:    "This is \\\"a\"\\ test",
			expected: "This is a test",
		},
	}

	for _, v := range tests {
		t.Run("", func(t *testing.T) {
			out := Unescape(v.input)
			assert.Equal(t, v.expected, out)
		})
	}
}

func TestUnique(t *testing.T) {
	tests := []sliceTestData{
		{
			input:    []string{},
			expected: []string{},
		},
		{
			input:    []string{"", ""},
			expected: []string{""},
		},
		{
			input:    []string{"a", "b", "a"},
			expected: []string{"a", "b"},
		},
		{
			input:    []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
	}

	for _, v := range tests {
		t.Run("", func(t *testing.T) {
			out := Unique(v.input)
			assert.Equal(t, v.expected, out)
		})
	}
}

func TestFirstOrEmpty(t *testing.T) {
	tests := []sliceStringTestData{
		{
			input:    []string{},
			expected: "",
		},
		{
			input:    []string{"b", "a"},
			expected: "b",
		},
		{
			input:    []string{"a", "b", "a"},
			expected: "a",
		},
		{
			input:    []string{"", "", "c"},
			expected: "",
		},
	}

	for _, v := range tests {
		t.Run("", func(t *testing.T) {
			out := FirstOrEmpty(v.input)
			assert.Equal(t, v.expected, out)
		})
	}
}
