package validator_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/vektah/gqlparser/v2/parser"
	"github.com/vektah/gqlparser/v2/validator"
	"github.com/vektah/gqlparser/v2/validator/rules"
)

func TestExtendingNonExistantTypes(t *testing.T) {
	s := gqlparser.MustLoadSchema(
		&ast.Source{Name: "graph/schema.graphqls", Input: `
extend type User {
    id: ID!
}

extend type Product {
    upc: String!
}

union _Entity = Product | User

extend type Query {
	entity: _Entity
}
`, BuiltIn: false},
	)

	q, err := parser.ParseQuery(&ast.Source{Name: "ff", Input: `{
		entity {
		  ... on User {
			id
		  }
		}
	}`})
	require.NoError(t, err)

	require.Nil(t, validator.Validate(s, q))
	require.Nil(t, validator.ValidateWithRules(s, q, nil))
}

func TestVariablesInAllowedPositionRetainsSourcesForMultipleLocations(t *testing.T) {
	s := gqlparser.MustLoadSchema(&ast.Source{
		Name: "schema.graphqls",
		Input: `
input Filter @oneOf {
  byID: ID
}

type Query {
  search(filter: Filter): String
}
`,
	})

	querySource := &ast.Source{
		Name:  "queries/Search.graphql",
		Input: "query Search($id: ID) { ...SearchFields }",
	}
	fragmentSource := &ast.Source{
		Name:  "fragments/SearchFields.graphql",
		Input: "fragment SearchFields on Query { search(filter: {byID: $id}) }",
	}
	query, err := parser.ParseQuery(querySource)
	require.NoError(t, err)
	fragment, err := parser.ParseQuery(fragmentSource)
	require.NoError(t, err)

	doc := &ast.QueryDocument{
		Operations: query.Operations,
		Fragments:  fragment.Fragments,
	}
	errs := validator.Validate(s, doc)

	var found *gqlerror.Error
	for _, err := range errs {
		if err.Rule == "VariablesInAllowedPosition" {
			found = err
			break
		}
	}
	require.NotNil(t, found, "expected a VariablesInAllowedPosition error, got %v", errs)
	require.Len(t, found.Locations, 2)
	require.Len(t, found.LocationSources, 2)
	require.Same(t, querySource, found.LocationSources[0])
	require.Same(t, fragmentSource, found.LocationSources[1])
	require.Equal(t, 1, found.Locations[0].Line)
	require.Equal(t, 1, found.Locations[1].Line)
	require.Equal(t, 14, found.Locations[0].Column)
	require.Equal(t, 56, found.Locations[1].Column)
	require.Equal(t, fragmentSource.Name, found.Extensions["file"])
	require.Equal(t, "queries/Search.graphql:1:14: "+found.Message, found.Error())

	encoded, marshalErr := json.Marshal(found)
	require.NoError(t, marshalErr)
	require.JSONEq(t, `{
		"message": "Variable \"$id\" is of type \"ID\" but must be non-nullable to be used for OneOf Input Object \"Filter\".",
		"locations": [
			{"line": 1, "column": 14},
			{"line": 1, "column": 56}
		],
		"extensions": {"file": "fragments/SearchFields.graphql"}
	}`, string(encoded))
}

func TestSingleLocationValidationRetainsSource(t *testing.T) {
	s := gqlparser.MustLoadSchema(&ast.Source{
		Name:  "schema.graphqls",
		Input: "type Query { search: String }",
	})
	source := &ast.Source{
		Name:  "queries/Search.graphql",
		Input: "query Search { missing }",
	}
	query, err := parser.ParseQuery(source)
	require.NoError(t, err)

	var found *gqlerror.Error
	for _, err := range validator.Validate(s, query) {
		if err.Rule == "FieldsOnCorrectType" {
			found = err
			break
		}
	}
	require.NotNil(t, found)
	require.Len(t, found.Locations, 1)
	require.Len(t, found.LocationSources, 1)
	require.Same(t, source, found.LocationSources[0])
}

func TestValidationRulesAreIndependent(t *testing.T) {
	s := gqlparser.MustLoadSchema(
		&ast.Source{Name: "graph/schema.graphqls", Input: `
extend type Query {
    myAction(myEnum: Locale!): SomeResult!
}

type SomeResult {
    id: String
}

enum Locale {
    EN
    LT
    DE
}
`, BuiltIn: false},
	)

	// Validation as a first call
	q1, err := parser.ParseQuery(&ast.Source{
		Name: "SomeOperation", Input: `
query SomeOperation {
	# Note: Not providing mandatory parameter: (myEnum: Locale!)
	myAction {
		id
	}
}
	`,
	})
	require.NoError(t, err)

	r1 := validator.Validate(s, q1)
	require.Len(t, r1, 1)
	const errorString = `SomeOperation:4:2: Field "myAction" argument "myEnum" of type "Locale!" is required, but it was not provided.`
	require.EqualError(t, r1[0], errorString)

	// Some other call that should not affect validator behavior
	q2, err := parser.ParseQuery(&ast.Source{
		Name: "SomeOperation", Input: `
# Note: there is default enum value in variables
query SomeOperation ($locale: Locale! = DE) {
	myAction(myEnum: $locale) {
		id
	}
}
	`,
	})
	require.NoError(t, err)

	require.Nil(t, validator.Validate(s, q2))

	// Repeating same query and expecting to still return same validation error
	require.Len(t, r1, 1)
	require.EqualError(t, r1[0], errorString)
}

func TestValidationRulesAreIndependentWithRules(t *testing.T) {
	s := gqlparser.MustLoadSchema(
		&ast.Source{Name: "graph/schema.graphqls", Input: `
extend type Query {
    myAction(myEnum: Locale!): SomeResult!
}

type SomeResult {
    id: String
}

enum Locale {
    EN
    LT
    DE
}
`, BuiltIn: false},
	)

	// Validation as a first call
	q1, err := parser.ParseQuery(&ast.Source{
		Name: "SomeOperation", Input: `
query SomeOperation {
	# Note: Not providing mandatory parameter: (myEnum: Locale!)
	myAction {
		id
	}
}
	`,
	})
	require.NoError(t, err)
	r1 := validator.ValidateWithRules(s, q1, nil)
	require.Len(t, r1, 1)
	const errorString = `SomeOperation:4:2: Field "myAction" argument "myEnum" of type "Locale!" is required, but it was not provided.`
	require.EqualError(t, r1[0], errorString)

	// Some other call that should not affect validator behavior
	q2, err := parser.ParseQuery(&ast.Source{
		Name: "SomeOperation", Input: `
# Note: there is default enum value in variables
query SomeOperation ($locale: Locale! = DE) {
	myAction(myEnum: $locale) {
		id
	}
}
	`,
	})
	require.NoError(t, err)
	require.Nil(t, validator.ValidateWithRules(s, q2, nil))

	// Repeating same query and expecting to still return same validation error
	require.Len(t, r1, 1)
	require.EqualError(t, r1[0], errorString)
}

func TestDeprecatingTypes(t *testing.T) {
	schema := &ast.Source{
		Name: "graph/schema.graphqls",
		Input: `
			type DeprecatedType {
				deprecatedField: String @deprecated
				newField(deprecatedArg: Int): Boolean
			}

			enum DeprecatedEnum {
				ALPHA @deprecated
			}
		`,
		BuiltIn: false,
	}

	_, err := validator.LoadSchema(append([]*ast.Source{validator.Prelude}, schema)...)
	require.NoError(t, err)
}

func TestNoUnusedVariables(t *testing.T) {
	// https://github.com/99designs/gqlgen/issues/2028
	t.Run("gqlgen issues #2028", func(t *testing.T) {
		s := gqlparser.MustLoadSchema(
			&ast.Source{Name: "graph/schema.graphqls", Input: `
	type Query {
		bar: String!
	}
	`, BuiltIn: false},
		)

		q, err := parser.ParseQuery(&ast.Source{Name: "2028", Input: `
			query Foo($flag: Boolean!) {
				...Bar
			}
			fragment Bar on Query {
				bar @include(if: $flag)
			}
		`})
		require.NoError(t, err)

		require.Nil(t, validator.Validate(s, q))
	})
}

func TestNoUnusedVariablesWithRules(t *testing.T) {
	// https://github.com/99designs/gqlgen/issues/2028
	t.Run("gqlgen issues #2028", func(t *testing.T) {
		s := gqlparser.MustLoadSchema(
			&ast.Source{Name: "graph/schema.graphqls", Input: `
	type Query {
		bar: String!
	}
	`, BuiltIn: false},
		)

		q, err := parser.ParseQuery(&ast.Source{Name: "2028", Input: `
			query Foo($flag: Boolean!) {
				...Bar
			}
			fragment Bar on Query {
				bar @include(if: $flag)
			}
		`})
		require.NoError(t, err)
		require.Nil(t, validator.ValidateWithRules(s, q, nil))
	})

	t.Run("variable used in fragment definition directive", func(t *testing.T) {
		s := gqlparser.MustLoadSchema(
			&ast.Source{Name: "graph/schema.graphqls", Input: `
	directive @testDirective(x: Int) on FRAGMENT_DEFINITION

	type Query {
		bar: String!
	}
	`, BuiltIn: false},
		)

		q, err := parser.ParseQuery(&ast.Source{Name: "fragmentDefinitionDirective", Input: `
			query Foo($x: Int) {
				...Bar
			}

			fragment Bar on Query @testDirective(x: $x) {
				bar
			}
		`})
		require.NoError(t, err)

		require.Nil(t, validator.ValidateWithRules(s, q, nil))
	})
}

func TestCustomRuleSet(t *testing.T) {
	someRule := validator.Rule{
		Name: "SomeRule",
		RuleFunc: func(observers *validator.Events, addError validator.AddErrFunc) {
			addError(validator.Message("%s", "some error message"))
		},
	}

	someOtherRule := validator.Rule{
		Name: "SomeOtherRule",
		RuleFunc: func(observers *validator.Events, addError validator.AddErrFunc) {
			addError(validator.Message("%s", "some other error message"))
		},
	}

	s := gqlparser.MustLoadSchema(
		&ast.Source{
			Name: "graph/schema.graphqls",
			Input: `
	type Query {
		bar: String!
	}
	`, BuiltIn: false,
		},
	)

	q, err := parser.ParseQuery(&ast.Source{
		Name: "SomeQuery",
		Input: `
			query Foo($flag: Boolean!) {
				...Bar
			}
		`,
	})
	require.NoError(t, err)

	errList := validator.Validate(s, q, []validator.Rule{someRule, someOtherRule}...)
	require.Len(t, errList, 2)
	require.Equal(t, "some error message", errList[0].Message)
	require.Equal(t, "some other error message", errList[1].Message)
}

func TestCustomRuleSetWithRules(t *testing.T) {
	someRule := validator.Rule{
		Name: "SomeRule",
		RuleFunc: func(observers *validator.Events, addError validator.AddErrFunc) {
			addError(validator.Message("%s", "some error message"))
		},
	}

	someOtherRule := validator.Rule{
		Name: "SomeOtherRule",
		RuleFunc: func(observers *validator.Events, addError validator.AddErrFunc) {
			addError(validator.Message("%s", "some other error message"))
		},
	}

	s := gqlparser.MustLoadSchema(
		&ast.Source{
			Name: "graph/schema.graphqls",
			Input: `
	type Query {
		bar: String!
	}
	`, BuiltIn: false,
		},
	)

	q, err := parser.ParseQuery(&ast.Source{
		Name: "SomeQuery",
		Input: `
			query Foo($flag: Boolean!) {
				...Bar
			}
		`,
	})
	require.NoError(t, err)
	errList := validator.ValidateWithRules(s, q, rules.NewRules(someRule, someOtherRule))
	require.Len(t, errList, 2)

	// because we hold rules in a map, the order is not guaranteed
	// this is fine because we used to add the rule in the init function, so it didn't need to be
	// specified as a requirement for the order.
	messages := []string{errList[0].Message, errList[1].Message}
	require.Contains(t, messages, "some error message")
	require.Contains(t, messages, "some other error message")
}

func TestRemoveRule(t *testing.T) {
	// no error
	validator.RemoveRule("rule that does not exist")

	validator.AddRule(
		"Rule that should no longer exist",
		func(observers *validator.Events, addError validator.AddErrFunc) {},
	)

	// no error
	validator.RemoveRule("Rule that should no longer exist")
}
