package gqlerror_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/vektah/gqlparser/v2/gqlerror"
)

func TestLocationSupportsUnkeyedLiteral(t *testing.T) {
	location := gqlerror.Location{1, 2}

	require.Equal(t, 1, location.Line)
	require.Equal(t, 2, location.Column)
}
