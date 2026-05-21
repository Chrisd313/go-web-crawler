package crawler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalise_PreservesHTTPSURLs(t *testing.T) {
	// Act
	got := normalise("https://example.com/page")

	// Assert
	assert.Equal(t, "https://example.com/page", got)
}

func TestNormalise_ConvertsHTTPToHTTPS(t *testing.T) {
	// Act
	got := normalise("http://example.com/page")

	// Assert
	assert.Equal(t, "https://example.com/page", got)
}

func TestNormalise_StripsFragments(t *testing.T) {
	// Act
	got := normalise("https://example.com/page#section")

	// Assert
	assert.Equal(t, "https://example.com/page", got)
}

func TestNormalise_StripsFragmentsAndConvertsScheme(t *testing.T) {
	// Act
	got := normalise("http://example.com/page#frag")

	// Assert
	assert.Equal(t, "https://example.com/page", got)
}

func TestNormalise_PreservesQueryStrings(t *testing.T) {
	// Act
	got := normalise("https://example.com/page?q=1")

	// Assert
	assert.Equal(t, "https://example.com/page?q=1", got)
}

func TestNormalise_HandlesMalformedURL(t *testing.T) {
	// Arrange
	input := "://bad-url"

	// Act
	got := normalise(input)

	// Assert - should return as-is on parse error
	assert.Equal(t, input, got)
}
