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

func TestSourceLocationsAreAvailableOutsidePackage(t *testing.T) {
	source := &ast.Source{Name: "query.graphql"}
	err := &gqlerror.Error{}
	err.AddLocation(gqlerror.Location{Line: 1, Column: 2}, source)

	locations := err.SourceLocations()
	require.Len(t, locations, 1)
	require.Equal(t, gqlerror.Location{Line: 1, Column: 2}, locations[0].Location)
	require.Same(t, source, locations[0].Source)
}

func TestErrorSupportsKeyedLiteral(t *testing.T) {
	err := gqlerror.Error{Message: "kabloom"}

	require.Equal(t, "kabloom", err.Message)
}
