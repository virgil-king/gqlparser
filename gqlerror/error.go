package gqlerror

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
)

// Error is the standard graphql error type described in https://spec.graphql.org/draft/#sec-Errors
type Error struct {
	Err             error          `json:"-"`
	Message         string         `json:"message"`
	Path            ast.Path       `json:"path,omitempty"`
	Locations       []Location     `json:"locations,omitempty"`
	LocationSources []*ast.Source  `json:"-"`
	Extensions      map[string]any `json:"extensions,omitempty"`
	Rule            string         `json:"-"`
}

func (err *Error) SetFile(file string) {
	if file == "" {
		return
	}
	if err.Extensions == nil {
		err.Extensions = map[string]any{}
	}

	err.Extensions["file"] = file
}

type Location struct {
	Line   int `json:"line,omitempty"`
	Column int `json:"column,omitempty"`
}

// SourceLocation pairs a GraphQL location with its source document. Source is
// nil when the location has no source document.
type SourceLocation struct {
	Location Location
	Source   *ast.Source
}

// AddLocation appends a location to the GraphQL response and its source-aware
// representation.
func (err *Error) AddLocation(location Location, source *ast.Source) {
	if len(err.LocationSources) == 0 && len(err.Locations) > 0 {
		err.LocationSources = make([]*ast.Source, len(err.Locations))
	}
	if len(err.LocationSources) != len(err.Locations) {
		panic(fmt.Sprintf(
			"gqlerror: location count %d does not match source count %d",
			len(err.Locations),
			len(err.LocationSources),
		))
	}
	err.Locations = append(err.Locations, location)
	err.LocationSources = append(err.LocationSources, source)
}

// SourceLocations returns a shallow copy of the source-aware locations in
// validation order. It returns nil when the error has no source metadata. It
// panics when LocationSources is not aligned with Locations.
func (err *Error) SourceLocations() []SourceLocation {
	if err == nil {
		return nil
	}
	if len(err.LocationSources) == 0 {
		return nil
	}
	if len(err.LocationSources) != len(err.Locations) {
		panic(fmt.Sprintf(
			"gqlerror: location count %d does not match source count %d",
			len(err.Locations),
			len(err.LocationSources),
		))
	}
	locations := make([]SourceLocation, len(err.Locations))
	for i, location := range err.Locations {
		locations[i] = SourceLocation{
			Location: location,
			Source:   err.LocationSources[i],
		}
	}
	return locations
}

// UnmarshalJSON discards source documents because GraphQL error JSON does not
// encode them.
func (err *Error) UnmarshalJSON(data []byte) error {
	type errorWithoutMethods Error
	err.LocationSources = nil
	return json.Unmarshal(data, (*errorWithoutMethods)(err))
}

type List []*Error

func (err *Error) Error() string {
	var res strings.Builder
	if err == nil {
		return ""
	}
	sourcesAligned := len(err.LocationSources) == len(err.Locations)
	filename, _ := err.Extensions["file"].(string)
	if len(err.Locations) == 1 {
		if filename == "" && sourcesAligned && err.LocationSources[0] != nil {
			filename = err.LocationSources[0].Name
		}
	} else if len(err.Locations) > 1 && sourcesAligned {
		filename = ""
		if err.LocationSources[0] != nil {
			filename = err.LocationSources[0].Name
		}
	}
	if filename == "" {
		filename = "input"
	}
	res.WriteString(filename)

	if len(err.Locations) > 0 {
		res.WriteByte(':')
		res.WriteString(strconv.Itoa(err.Locations[0].Line))
		res.WriteByte(':')
		res.WriteString(strconv.Itoa(err.Locations[0].Column))
	}

	res.WriteString(": ")
	if ps := err.pathString(); ps != "" {
		res.WriteString(ps)
		res.WriteByte(' ')
	}

	res.WriteString(err.Message)

	return res.String()
}

func (err *Error) pathString() string {
	return err.Path.String()
}

func (err *Error) Unwrap() error {
	return err.Err
}

func (err *Error) AsError() error {
	if err == nil {
		return nil
	}
	return err
}

func (errs List) Error() string {
	var buf strings.Builder
	for _, err := range errs {
		buf.WriteString(err.Error())
		buf.WriteByte('\n')
	}
	return buf.String()
}

func (errs List) Is(target error) bool {
	for _, err := range errs {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

func (errs List) As(target any) bool {
	for _, err := range errs {
		if errors.As(err, target) {
			return true
		}
	}
	return false
}

func (errs List) Unwrap() []error {
	l := make([]error, len(errs))
	for i, err := range errs {
		l[i] = err
	}
	return l
}

func WrapPath(path ast.Path, err error) *Error {
	if err == nil {
		return nil
	}
	return &Error{
		Err:     err,
		Message: err.Error(),
		Path:    path,
	}
}

func Wrap(err error) *Error {
	if err == nil {
		return nil
	}
	return &Error{
		Err:     err,
		Message: err.Error(),
	}
}

func WrapIfUnwrapped(err error) *Error {
	if err == nil {
		return nil
	}
	gqlErr := &Error{}
	if errors.As(err, &gqlErr) {
		return gqlErr
	}
	return &Error{
		Err:     err,
		Message: err.Error(),
	}
}

func Errorf(message string, args ...any) *Error {
	return &Error{
		Message: fmt.Sprintf(message, args...),
	}
}

func ErrorPathf(path ast.Path, message string, args ...any) *Error {
	return &Error{
		Message: fmt.Sprintf(message, args...),
		Path:    path,
	}
}

func ErrorPosf(pos *ast.Position, message string, args ...any) *Error {
	if pos == nil {
		return ErrorLocf(
			"",
			-1,
			-1,
			message,
			args...,
		)
	}
	err := &Error{Message: fmt.Sprintf(message, args...)}
	err.SetFile(pos.Src.Name)
	err.AddLocation(Location{Line: pos.Line, Column: pos.Column}, pos.Src)
	return err
}

func ErrorLocf(file string, line, col int, message string, args ...any) *Error {
	err := &Error{Message: fmt.Sprintf(message, args...)}
	err.SetFile(file)
	err.AddLocation(Location{Line: line, Column: col}, nil)
	return err
}
