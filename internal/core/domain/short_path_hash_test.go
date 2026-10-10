package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShortPathHash_ProducesStable32CharHex(t *testing.T) {
	hash := ShortPathHash("some-repo", "main")

	assert.Len(t, hash, 32)
	assert.Equal(t, "59fff7f9d471b8034ac2b977ceb37072", hash)
}

func TestShortPathHash_DiffersByPartBoundary(t *testing.T) {
	assert.NotEqual(t, ShortPathHash("some-repo", "main"), ShortPathHash("some", "repo-main"))
}

func TestShortPathHash_DiffersByRef(t *testing.T) {
	assert.NotEqual(t, ShortPathHash("some-repo", "main"), ShortPathHash("some-repo", "feature/x"))
}

func TestShortPathHash_DiffersByRepo(t *testing.T) {
	assert.NotEqual(t, ShortPathHash("repo-a", "main"), ShortPathHash("repo-b", "main"))
}
