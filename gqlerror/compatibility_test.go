package gqlerror_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

func TestLocationSupportsUnkeyedLiteral(t *testing.T) {
	location := gqlerror.Location{1, 2}

	require.Equal(t, 1, location.Line)
	require.Equal(t, 2, location.Column)
}

func TestLocationSourcesAreAvailableOutsidePackage(t *testing.T) {
	source := &ast.Source{Name: "query.graphql"}
	err := &gqlerror.Error{}
	err.AddLocation(gqlerror.Location{Line: 1, Column: 2}, source)

	require.Len(t, err.LocationSources, 1)
	require.Same(t, source, err.LocationSources[0])
	require.Equal(t, gqlerror.Location{Line: 1, Column: 2}, err.Locations[0])
}

func TestErrorSupportsKeyedLiteral(t *testing.T) {
	err := gqlerror.Error{Message: "kabloom"}

	require.Equal(t, "kabloom", err.Message)
}
