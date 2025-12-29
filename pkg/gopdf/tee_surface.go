package gopdf

import (
	"runtime"
)

// TeeSurface is a surface that redirects drawing operations to multiple target surfaces.
type TeeSurface interface {
	Surface
	AddSurface(Surface) error
	RemoveSurface(Surface) error
}

// teeSurface implements the TeeSurface interface.
type teeSurface struct {
	baseSurface

	// The list of target surfaces
	targets []Surface
}

// NewTeeSurface creates a new Tee surface.
func NewTeeSurface() TeeSurface {
	surface := &teeSurface{
		baseSurface: baseSurface{
			refCount:            1,
			status:              StatusSuccess,
			surfaceType:         SurfaceTypeTee,
			content:             ContentColorAlpha, // Tee surface content is the union of its targets
			userData:            make(map[*UserDataKey]interface{}),
			fontOptions:         &FontOptions{},
			deviceScaleX:        1.0,
			deviceScaleY:        1.0,
			fallbackResolutionX: 72.0,
			fallbackResolutionY: 72.0,
		},
		targets: make([]Surface, 0),
	}

	runtime.SetFinalizer(surface, (*teeSurface).Destroy)
	return surface
}

// AddSurface adds a surface to the list of targets.
func (s *teeSurface) AddSurface(target Surface) error {
	if target == nil {
		return newError(StatusNullPointer, "target surface is nil")
	}
	s.targets = append(s.targets, target.Reference())
	return nil
}

// RemoveSurface removes a surface from the list of targets.
func (s *teeSurface) RemoveSurface(target Surface) error {
	if target == nil {
		return newError(StatusNullPointer, "target surface is nil")
	}

	for i, t := range s.targets {
		// Check if the target is the same object
		if t == target {
			// Remove the element and destroy its reference
			t.Destroy()
			s.targets = append(s.targets[:i], s.targets[i+1:]...)
			return nil
		}
	}

	return newError(StatusInvalidIndex, "target surface not found in tee surface")
}

// GetTargets returns the list of target surfaces.
func (s *teeSurface) GetTargets() []Surface {
	return s.targets
}

// The Context implementation needs to be updated to handle TeeSurface.
// When a Context is created with a TeeSurface, the Context's drawing operations
// must be forwarded to a Context created for each target surface.
// This requires a significant change in context.go's NewContext logic.
// We will address this in a subsequent step.
