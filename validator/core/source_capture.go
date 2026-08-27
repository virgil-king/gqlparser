package core

import (
	"fmt"
	"sync"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

type sourceCapture struct {
	err       *gqlerror.Error
	locations []gqlerror.Location
	sources   []*ast.Source
}

var sourceCaptures sync.Map // map[*gqlerror.Error]*sourceCapture

// CaptureSourceLocations applies error options while recording the source
// documents supplied to At. It returns sources aligned with err.Locations.
// Source-aware validation uses this helper; the regular validation API does
// not install a capture and keeps its existing behavior.
func CaptureSourceLocations(err *gqlerror.Error, apply func()) []*ast.Source {
	if err == nil {
		panic("gqlparser: cannot capture source locations for a nil error")
	}
	capture := &sourceCapture{err: err}
	sourceCaptures.Store(err, capture)
	defer sourceCaptures.Delete(err)

	apply()
	if len(capture.locations) == 0 {
		return nil
	}
	sources := make([]*ast.Source, len(err.Locations))
	recorded := 0
	for i, location := range err.Locations {
		if recorded == len(capture.locations) {
			break
		}
		if capture.locations[recorded] != location {
			continue
		}
		sources[i] = capture.sources[recorded]
		recorded++
	}
	if recorded != len(capture.locations) {
		panic(fmt.Sprintf(
			"gqlparser: captured source location %d does not match the final error locations",
			recorded,
		))
	}
	return sources
}

func recordSourceLocation(err *gqlerror.Error, location gqlerror.Location, source *ast.Source) {
	value, ok := sourceCaptures.Load(err)
	if !ok {
		return
	}
	capture := value.(*sourceCapture)
	capture.locations = append(capture.locations, location)
	capture.sources = append(capture.sources, source)
}
