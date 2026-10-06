package main

import (
	"testing"
)

func TestMatchUnorderedLines(t *testing.T) {
	tests := []struct {
		name     string
		actual   string
		expected string
		want     bool
	}{
		{
			name:     "identical order",
			actual:   "apple\nbanana\ncherry\n",
			expected: "apple\nbanana\ncherry\n",
			want:     true,
		},
		{
			name:     "permuted order",
			actual:   "cherry\napple\nbanana\n",
			expected: "apple\nbanana\ncherry\n",
			want:     true,
		},
		{
			name:     "whitespace and trailing newlines",
			actual:   "  cherry \n\n apple\nbanana\n\n",
			expected: "apple\nbanana\ncherry",
			want:     true,
		},
		{
			name:     "count mismatch - missing element",
			actual:   "apple\nbanana\n",
			expected: "apple\nbanana\ncherry\n",
			want:     false,
		},
		{
			name:     "count mismatch - extra element",
			actual:   "apple\nbanana\ncherry\ndate\n",
			expected: "apple\nbanana\ncherry\n",
			want:     false,
		},
		{
			name:     "duplicate elements match count",
			actual:   "foo\nbar\nfoo\n",
			expected: "bar\nfoo\nfoo\n",
			want:     true,
		},
		{
			name:     "duplicate elements mismatch count",
			actual:   "foo\nbar\nbar\n",
			expected: "foo\nfoo\nbar\n",
			want:     false,
		},
		{
			name:     "both empty",
			actual:   "",
			expected: "",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchUnorderedLines(tt.actual, tt.expected)
			if got != tt.want {
				t.Errorf("matchUnorderedLines(%q, %q) = %v, want %v", tt.actual, tt.expected, got, tt.want)
			}
		})
	}
}
