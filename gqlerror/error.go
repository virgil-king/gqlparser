package gqlerror

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
)

// Error is the standard graphql error type described in https://spec.graphql.org/draft/#sec-Errors
type Error struct {
	Err             error      `json:"-"`
	Message         string     `json:"message"`
	Path            ast.Path   `json:"path,omitempty"`
	Locations       []Location `json:"locations,omitempty"`
	sourceLocations []SourceLocation
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

// SourceLocation identifies a location and its source document. Source is nil
// when the location has no source document.
type SourceLocation struct {
	Location Location
	Source   *ast.Source
}

// AddLocation appends a location to the GraphQL response and its source-aware
// representation.
func (err *Error) AddLocation(location Location, source *ast.Source) {
	err.validateSourceLocations()
	if len(err.sourceLocations) == 0 && len(err.Locations) > 0 {
		err.sourceLocations = make([]SourceLocation, len(err.Locations))
		for i, existing := range err.Locations {
			err.sourceLocations[i].Location = existing
		}
	}
	err.Locations = append(err.Locations, location)
	err.sourceLocations = append(err.sourceLocations, SourceLocation{
		Location: location,
		Source:   source,
	})
}

// SourceLocations returns a shallow copy of the source-aware locations in
// validation order. It panics when Locations changed independently after a
// source-aware location was added.
func (err *Error) SourceLocations() []SourceLocation {
	if err == nil {
		return nil
	}
	err.validateSourceLocations()
	locations := make([]SourceLocation, len(err.sourceLocations))
	copy(locations, err.sourceLocations)
	return locations
}

func (err *Error) validateSourceLocations() {
	if len(err.sourceLocations) == 0 {
		return
	}
	if len(err.sourceLocations) != len(err.Locations) {
		panic(fmt.Sprintf(
			"gqlerror: location count %d does not match source location count %d",
			len(err.Locations),
			len(err.sourceLocations),
		))
	}
	for i, sourceLocation := range err.sourceLocations {
		if sourceLocation.Location != err.Locations[i] {
			panic(fmt.Sprintf("gqlerror: location %d changed independently of its source", i))
		}
	}
}

type List []*Error

func (err *Error) Error() string {
	var res strings.Builder
	if err == nil {
		return ""
	}
	err.validateSourceLocations()
	filename := ""
	if len(err.sourceLocations) == 0 {
		filename, _ = err.Extensions["file"].(string)
	} else if err.sourceLocations[0].Source != nil {
		filename = err.sourceLocations[0].Source.Name
	} else if len(err.sourceLocations) == 1 {
		filename, _ = err.Extensions["file"].(string)
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
