package serve

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Utility functions", func() {
	Context("deleteFirstElementIfFileName function", func() {
		It("should remove first element if it starts with @", func() {
			input := []string{"@filename", "service_log", "clusters"}
			result := deleteFirstElementIfFileName(input)

			Expect(result).To(HaveLen(2))
			Expect(result).To(ConsistOf("service_log", "clusters"))
		})

		It("should not modify slice if first element doesn't start with @", func() {
			input := []string{"service_log", "clusters", "upgrade_policies"}
			result := deleteFirstElementIfFileName(input)

			Expect(result).To(HaveLen(3))
			Expect(result).To(ConsistOf("service_log", "clusters", "upgrade_policies"))
		})

		It("should handle empty slice gracefully", func() {
			input := []string{}

			// This should not panic and should return the empty slice unchanged
			result := deleteFirstElementIfFileName(input)
			Expect(result).To(HaveLen(0))
			Expect(result).To(BeEmpty())
		})

		It("should handle single element slice with @", func() {
			input := []string{"@filename"}
			result := deleteFirstElementIfFileName(input)

			Expect(result).To(HaveLen(0))
			Expect(result).To(BeEmpty())
		})

		It("should handle single element slice without @", func() {
			input := []string{"service_log"}
			result := deleteFirstElementIfFileName(input)

			Expect(result).To(HaveLen(1))
			Expect(result).To(ConsistOf("service_log"))
		})

		It("should handle @ symbol in middle of first element", func() {
			input := []string{"service@log", "clusters"}
			result := deleteFirstElementIfFileName(input)

			Expect(result).To(HaveLen(2))
			Expect(result).To(ConsistOf("service@log", "clusters"))
		})

		It("should handle first element that is just @", func() {
			input := []string{"@", "service_log"}
			result := deleteFirstElementIfFileName(input)

			Expect(result).To(HaveLen(1))
			Expect(result).To(ConsistOf("service_log"))
		})

		It("should preserve order of remaining elements", func() {
			input := []string{"@filename", "first", "second", "third"}
			result := deleteFirstElementIfFileName(input)

			Expect(result).To(HaveLen(3))
			Expect(result[0]).To(Equal("first"))
			Expect(result[1]).To(Equal("second"))
			Expect(result[2]).To(Equal("third"))
		})

		It("should handle complex filenames with special characters", func() {
			input := []string{"@/path/to/file-name_with.special@chars", "service_log"}
			result := deleteFirstElementIfFileName(input)

			Expect(result).To(HaveLen(1))
			Expect(result).To(ConsistOf("service_log"))
		})
	})
})

// Test the function directly for better performance testing
func TestDeleteFirstElementIfFileName(t *testing.T) {
	testCases := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "Remove file reference",
			input:    []string{"@file", "a", "b"},
			expected: []string{"a", "b"},
		},
		{
			name:     "Keep non-file reference",
			input:    []string{"service", "a", "b"},
			expected: []string{"service", "a", "b"},
		},
		{
			name:     "Single file element",
			input:    []string{"@file"},
			expected: []string{},
		},
		{
			name:     "Single non-file element",
			input:    []string{"service"},
			expected: []string{"service"},
		},
		{
			name:     "Empty slice",
			input:    []string{},
			expected: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := deleteFirstElementIfFileName(tc.input)
			if len(result) != len(tc.expected) {
				t.Errorf("Expected length %d, got %d", len(tc.expected), len(result))
			}
			for i, expected := range tc.expected {
				if i >= len(result) || result[i] != expected {
					t.Errorf("Expected %v, got %v", tc.expected, result)
					break
				}
			}
		})
	}
}
