package gopdf

import (
	"runtime"
	"unsafe"
)

// UserFontFace implements a custom font face using user-provided data.
type UserFontFace interface {
	FontFace
	// Add other user font specific methods here, e.g.,
	// SetInitFunc, SetRenderGlyphFunc, etc.
}

// userFontFace implements the UserFontFace interface.
type userFontFace struct {
	baseFontFace

	// User-defined functions (placeholders)
	initFunc        func(face FontFace) Status
	renderGlyphFunc func(scaledFont ScaledFont, glyphID uint64, context Context) Status
}

// NewUserFontFace creates a new user font face.
func NewUserFontFace() UserFontFace {
	face := &userFontFace{
		baseFontFace: baseFontFace{
			refCount: 1,
			status:   StatusSuccess,
			fontType: FontTypeUser,
			userData: make(map[*UserDataKey]interface{}),
		},
	}

	runtime.SetFinalizer(face, (*userFontFace).Destroy)
	return face
}

// Reference increments the reference count.
func (f *userFontFace) Reference() FontFace {
	f.refCount++
	return f
}

// Destroy decrements the reference count and cleans up if it reaches zero.
func (f *userFontFace) Destroy() {
	// Decrement reference count and cleanup if needed
	// This is a placeholder implementation
	f.refCount--
	if f.refCount <= 0 {
		// Cleanup resources
		f.userData = nil
	}
}

// GetReferenceCount returns the current reference count.
func (f *userFontFace) GetReferenceCount() int {
	return int(f.refCount)
}

// GetType returns the font type.
func (f *userFontFace) GetType() FontType {
	return f.fontType
}

// Status returns the current status of the font face.
func (f *userFontFace) Status() Status {
	return f.status
}

// SetUserData sets user data for the font face.
func (f *userFontFace) SetUserData(key *UserDataKey, userData unsafe.Pointer, destroy DestroyFunc) Status {
	if f.status != StatusSuccess {
		return f.status
	}
	if f.userData == nil {
		f.userData = make(map[*UserDataKey]interface{})
	}
	f.userData[key] = userData
	_ = destroy // destroy func is currently ignored
	return StatusSuccess
}

// GetUserData retrieves user data for the font face.
func (f *userFontFace) GetUserData(key *UserDataKey) unsafe.Pointer {
	if f.userData == nil {
		return nil
	}
	if data, ok := f.userData[key]; ok {
		return data.(unsafe.Pointer)
	}
	return nil
}

// SetInitFunc sets the initialization function for the user font face.
func (f *userFontFace) SetInitFunc(initFunc func(face FontFace) Status) {
	f.initFunc = initFunc
}

// SetRenderGlyphFunc sets the function to render a single glyph.
func (f *userFontFace) SetRenderGlyphFunc(renderGlyphFunc func(scaledFont ScaledFont, glyphID uint64, context Context) Status) {
	f.renderGlyphFunc = renderGlyphFunc
}

// The ScaledFont implementation needs to be updated to call these functions.
// This is a complex task and requires significant changes to the font rendering pipeline.
// For now, this file defines the surface structure.
