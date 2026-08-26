package gqlerror

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/vektah/gqlparser/v2/ast"
)

type testError struct {
	message string
}

func (e testError) Error() string {
	return e.message
}

var (
	underlyingError = testError{
		"Underlying error",
	}

	error1 = &Error{
		Message: "Some error 1",
	}
	error2 = &Error{
		Err:     underlyingError,
		Message: "Some error 2",
	}
)

func TestErrorFormatting(t *testing.T) {
	t.Run("without filename", func(t *testing.T) {
		err := ErrorLocf("", 66, 2, "kabloom")

		require.Equal(t, `input:66:2: kabloom`, err.Error())
		require.Nil(t, err.Extensions["file"])
	})

	t.Run("with filename", func(t *testing.T) {
		err := ErrorLocf("schema.graphql", 66, 2, "kabloom")

		require.Equal(t, `schema.graphql:66:2: kabloom`, err.Error())
		require.Equal(t, "schema.graphql", err.Extensions["file"])
	})

	t.Run("with path", func(t *testing.T) {
		err := ErrorPathf(
			ast.Path{ast.PathName("a"), ast.PathIndex(1), ast.PathName("b")},
			"kabloom",
		)

		require.Equal(t, `input: a[1].b kabloom`, err.Error())
	})

	t.Run("with multiple sources", func(t *testing.T) {
		err := &Error{
			Message: "kabloom",
			Locations: []Location{
				{Line: 1, Column: 2},
				{Line: 3, Column: 4},
			},
			LocationSources: []*ast.Source{
				{Name: "first.graphql"},
				{Name: "second.graphql"},
			},
			Extensions: map[string]any{"file": "second.graphql"},
		}

		require.Equal(t, `first.graphql:1:2: kabloom`, err.Error())
	})

	t.Run("with unnamed primary source", func(t *testing.T) {
		err := &Error{
			Message: "kabloom",
			Locations: []Location{
				{Line: 1, Column: 2},
				{Line: 3, Column: 4},
			},
			LocationSources: []*ast.Source{
				{},
				{Name: "second.graphql"},
			},
			Extensions: map[string]any{"file": "second.graphql"},
		}

		require.Equal(t, `input:1:2: kabloom`, err.Error())
	})
}

func TestErrorPosition(t *testing.T) {
	t.Run("with nil position", func(t *testing.T) {
		err := ErrorLocf("", -1, -1, "kabloom")
		errNilPosition := ErrorPosf(nil, "%s", "kabloom")

		require.Equal(t, `input:-1:-1: kabloom`, err.Error())
		require.Equal(t, errNilPosition.Error(), err.Error())
		require.Nil(t, err.Extensions["file"])
		require.Nil(t, errNilPosition.Extensions["file"])
	})

	t.Run("retains source", func(t *testing.T) {
		source := &ast.Source{Name: "query.graphql", Input: "query Q { field }"}
		position := &ast.Position{Line: 1, Column: 11, Src: source}

		err := ErrorPosf(position, "kabloom")

		require.Len(t, err.Locations, 1)
		require.Len(t, err.LocationSources, 1)
		require.Same(t, source, err.LocationSources[0])
	})
}

func TestLocationSourcesAreNotSerialized(t *testing.T) {
	source := &ast.Source{Name: "query.graphql", Input: "query Q { field }"}
	err := &Error{
		Message: "kabloom",
		Locations: []Location{{
			Line: 1, Column: 11,
		}},
		LocationSources: []*ast.Source{source},
	}

	encoded, errEncoding := json.Marshal(err)
	require.NoError(t, errEncoding)
	require.JSONEq(t, `{"message":"kabloom","locations":[{"line":1,"column":11}]}`, string(encoded))
}

func TestList_As(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		errs        List
		target      any
		wantsTarget any
		targetFound bool
	}{
		{
			name: "Empty list",
			errs: List{},
		},
		{
			name:        "List with one error",
			errs:        List{error1},
			target:      new(*Error),
			wantsTarget: &error1,
			targetFound: true,
		},
		{
			name:        "List with multiple errors 1",
			errs:        List{error1, error2},
			target:      new(*Error),
			wantsTarget: &error1,
			targetFound: true,
		},
		{
			name:        "List with multiple errors 2",
			errs:        List{error1, error2},
			target:      new(testError),
			wantsTarget: &underlyingError,
			targetFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			targetFound := tt.errs.As(tt.target)

			if targetFound != tt.targetFound {
				t.Errorf("List.As() = %v, want %v", targetFound, tt.targetFound)
			}

			if tt.targetFound && !reflect.DeepEqual(tt.target, tt.wantsTarget) {
				t.Errorf("target = %v, want %v", tt.target, tt.wantsTarget)
			}
		})
	}
}

func TestList_Is(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		errs             List
		target           error
		hasMatchingError bool
	}{
		{
			name:             "Empty list",
			errs:             List{},
			target:           new(Error),
			hasMatchingError: false,
		},
		{
			name: "List with one error",
			errs: List{
				error1,
			},
			target:           error1,
			hasMatchingError: true,
		},
		{
			name: "List with multiple errors 1",
			errs: List{
				error1,
				error2,
			},
			target:           error2,
			hasMatchingError: true,
		},
		{
			name: "List with multiple errors 2",
			errs: List{
				error1,
				error2,
			},
			target:           underlyingError,
			hasMatchingError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			hasMatchingError := tt.errs.Is(tt.target)
			if hasMatchingError != tt.hasMatchingError {
				t.Errorf("List.Is() = %v, want %v", hasMatchingError, tt.hasMatchingError)
			}
			if hasMatchingError && tt.target == nil {
				t.Errorf("List.Is() returned nil target, wants concrete error")
			}
		})
	}
}

func BenchmarkError(b *testing.B) {
	list := List([]*Error{error1, error2})
	for range b.N {
		_ = underlyingError.Error()
		_ = error1.Error()
		_ = error2.Error()
		_ = list.Error()
	}
}
