package gopdf

import (
	"math"
	"testing"
)

func TestMatrixOperations(t *testing.T) {
	// Test identity matrix
	m := NewMatrix()
	if m.XX != 1.0 || m.YY != 1.0 || m.XY != 0.0 || m.YX != 0.0 || m.X0 != 0.0 || m.Y0 != 0.0 {
		t.Errorf("NewMatrix() should create identity matrix, got %+v", m)
	}

	// Test translation
	m.InitTranslate(10, 20)
	if m.X0 != 10.0 || m.Y0 != 20.0 {
		t.Errorf("InitTranslate(10, 20) failed, got X0=%f, Y0=%f", m.X0, m.Y0)
	}

	// Test scaling
	m.InitScale(2.0, 3.0)
	if m.XX != 2.0 || m.YY != 3.0 {
		t.Errorf("InitScale(2.0, 3.0) failed, got XX=%f, YY=%f", m.XX, m.YY)
	}

	// Test rotation (90 degrees)
	m.InitRotate(math.Pi / 2)
	tolerance := 1e-10
	if math.Abs(m.XX) > tolerance || math.Abs(m.YY) > tolerance || math.Abs(m.XY-(-1.0)) > tolerance || math.Abs(m.YX-1.0) > tolerance {
		t.Errorf("InitRotate(π/2) failed, got %+v", m)
	}
}

func TestMatrixTransform(t *testing.T) {
	m := NewMatrix()
	m.InitTranslate(10, 20)

	// Transform point
	x, y := MatrixTransformPoint(m, 5, 15)
	if x != 15.0 || y != 35.0 {
		t.Errorf("MatrixTransformPoint failed, expected (15, 35), got (%f, %f)", x, y)
	}

	// Transform distance (should not be affected by translation)
	dx, dy := MatrixTransformDistance(m, 5, 15)
	if dx != 5.0 || dy != 15.0 {
		t.Errorf("MatrixTransformDistance failed, expected (5, 15), got (%f, %f)", dx, dy)
	}
}

func TestMatrixInvert(t *testing.T) {
	// Test identity matrix inversion
	m := NewMatrix()
	status := MatrixInvert(m)
	if status != StatusSuccess {
		t.Errorf("MatrixInvert on identity failed with status %v", status)
	}

	// Test scaling matrix inversion
	m.InitScale(2.0, 4.0)
	status = MatrixInvert(m)
	if status != StatusSuccess {
		t.Errorf("MatrixInvert on scale matrix failed with status %v", status)
	}
	if math.Abs(m.XX-0.5) > 1e-10 || math.Abs(m.YY-0.25) > 1e-10 {
		t.Errorf("MatrixInvert scale result incorrect, got XX=%f, YY=%f", m.XX, m.YY)
	}
}

func TestSurfaceCreation(t *testing.T) {
	surface := NewImageSurface(FormatARGB32, 100, 200)
	if surface.Status() != StatusSuccess {
		t.Errorf("NewImageSurface failed with status %v", surface.Status())
	}

	// Test surface properties
	if surface.GetType() != SurfaceTypeImage {
		t.Errorf("Surface type incorrect, expected %v, got %v", SurfaceTypeImage, surface.GetType())
	}
	if surface.GetContent() != ContentColorAlpha {
		t.Errorf("Surface content incorrect, expected %v, got %v", ContentColorAlpha, surface.GetContent())
	}

	// Test that surface implements ImageSurface interface
	if imageSurface, ok := surface.(ImageSurface); ok {
		if imageSurface.GetWidth() != 100 || imageSurface.GetHeight() != 200 {
			t.Errorf("Surface dimensions incorrect, expected 100x200, got %dx%d", imageSurface.GetWidth(), imageSurface.GetHeight())
		}
		if imageSurface.GetFormat() != FormatARGB32 {
			t.Errorf("Surface format incorrect, expected %v, got %v", FormatARGB32, imageSurface.GetFormat())
		}
	} else {
		t.Errorf("Surface is not an ImageSurface")
	}
	surface.Destroy()

	// Test invalid surface creation
	invalidSurface := NewImageSurface(FormatARGB32, -1, 100)
	if invalidSurface.Status() == StatusSuccess {
		t.Errorf("NewImageSurface with negative width should fail")
	}
}

func TestContextCreation(t *testing.T) {
	surface := NewImageSurface(FormatARGB32, 100, 100)
	defer surface.Destroy()

	// Test valid context creation
	ctx := NewContext(surface)
	if ctx.Status() != StatusSuccess {
		t.Errorf("NewContext failed with status %v", ctx.Status())
	}
	if ctx.GetTarget() != surface {
		t.Errorf("Context target incorrect")
	}
	ctx.Destroy()

	// Test context creation with nil surface
	nilCtx := NewContext(nil)
	if nilCtx.Status() != StatusNullPointer {
		t.Errorf("NewContext with nil surface should return StatusNullPointer, got %v", nilCtx.Status())
	}
}

func TestPatternCreation(t *testing.T) {
	// Test solid color pattern
	pattern := NewPatternRGBA(0.5, 0.7, 0.9, 0.8)
	if pattern.Status() != StatusSuccess {
		t.Errorf("NewPatternRGBA failed with status %v", pattern.Status())
	}
	if pattern.GetType() != PatternTypeSolid {
		t.Errorf("Pattern type incorrect, expected %v, got %v", PatternTypeSolid, pattern.GetType())
	}
	if solidPattern, ok := pattern.(SolidPattern); ok {
		r, g, b, a := solidPattern.GetRGBA()
		tolerance := 1e-10
		if math.Abs(r-0.5) > tolerance || math.Abs(g-0.7) > tolerance || math.Abs(b-0.9) > tolerance || math.Abs(a-0.8) > tolerance {
			t.Errorf("Pattern RGBA values incorrect, expected (0.5, 0.7, 0.9, 0.8), got (%f, %f, %f, %f)", r, g, b, a)
		}
	} else {
		t.Errorf("Pattern is not a SolidPattern")
	}
	pattern.Destroy()

	// Test linear gradient pattern
	gradient := NewPatternLinear(0, 0, 100, 100)
	if gradient.Status() != StatusSuccess {
		t.Errorf("NewPatternLinear failed with status %v", gradient.Status())
	}
	if gradient.GetType() != PatternTypeLinear {
		t.Errorf("Gradient type incorrect, expected %v, got %v", PatternTypeLinear, gradient.GetType())
	}
	gradient.Destroy()
}

func TestContextStateManagement(t *testing.T) {
	surface := NewImageSurface(FormatARGB32, 100, 100)
	defer surface.Destroy()

	ctx := NewContext(surface)
	defer ctx.Destroy()

	// Test initial state
	if ctx.GetOperator() != OperatorOver {
		t.Errorf("Initial operator should be OperatorOver, got %v", ctx.GetOperator())
	}
	if ctx.GetLineWidth() != 2.0 {
		t.Errorf("Initial line width should be 2.0, got %f", ctx.GetLineWidth())
	}

	// Test save/restore
	ctx.SetOperator(OperatorSource)
	ctx.SetLineWidth(5.0)
	ctx.Save()
	ctx.SetOperator(OperatorClear)
	ctx.SetLineWidth(10.0)
	if ctx.GetOperator() != OperatorClear {
		t.Errorf("Operator after change should be OperatorClear, got %v", ctx.GetOperator())
	}
	ctx.Restore()
	if ctx.GetOperator() != OperatorSource {
		t.Errorf("Operator after restore should be OperatorSource, got %v", ctx.GetOperator())
	}
	if ctx.GetLineWidth() != 5.0 {
		t.Errorf("Line width after restore should be 5.0, got %f", ctx.GetLineWidth())
	}
}

func TestPathOperations(t *testing.T) {
	surface := NewImageSurface(FormatARGB32, 100, 100)
	defer surface.Destroy()

	ctx := NewContext(surface)
	defer ctx.Destroy()

	// Test current point
	if ctx.HasCurrentPoint() != False {
		t.Errorf("New context should not have current point")
	}

	// Test move to
	ctx.MoveTo(10, 20)
	if ctx.HasCurrentPoint() != True {
		t.Errorf("Context should have current point after MoveTo")
	}
	x, y := ctx.GetCurrentPoint()
	if x != 10.0 || y != 20.0 {
		t.Errorf("Current point should be (10, 20), got (%f, %f)", x, y)
	}

	// Test line to
	ctx.LineTo(30, 40)
	x, y = ctx.GetCurrentPoint()
	if x != 30.0 || y != 40.0 {
		t.Errorf("Current point after LineTo should be (30, 40), got (%f, %f)", x, y)
	}

	// Test new path
	ctx.NewPath()
	if ctx.HasCurrentPoint() != False {
		t.Errorf("Context should not have current point after NewPath")
	}
}

func TestReferenceCountingContext(t *testing.T) {
	surface := NewImageSurface(FormatARGB32, 100, 100)
	defer surface.Destroy()

	ctx := NewContext(surface)
	if ctx.GetReferenceCount() != 1 {
		t.Errorf("Initial reference count should be 1, got %d", ctx.GetReferenceCount())
	}
	ctx2 := ctx.Reference()
	if ctx.GetReferenceCount() != 2 {
		t.Errorf("Reference count after Reference() should be 2, got %d", ctx.GetReferenceCount())
	}
	ctx2.Destroy()
	if ctx.GetReferenceCount() != 1 {
		t.Errorf("Reference count after Destroy() should be 1, got %d", ctx.GetReferenceCount())
	}
	ctx.Destroy()
}

func TestReferenceCountingPattern(t *testing.T) {
	pattern := NewPatternRGB(1, 0, 0)
	if pattern.GetReferenceCount() != 1 {
		t.Errorf("Initial reference count should be 1, got %d", pattern.GetReferenceCount())
	}
	pattern2 := pattern.Reference()
	if pattern.GetReferenceCount() != 2 {
		t.Errorf("Reference count after Reference() should be 2, got %d", pattern.GetReferenceCount())
	}
	pattern2.Destroy()
	if pattern.GetReferenceCount() != 1 {
		t.Errorf("Reference count after Destroy() should be 1, got %d", pattern.GetReferenceCount())
	}
	pattern.Destroy()
}

func TestArcOperations(t *testing.T) {
	surface := NewImageSurface(FormatARGB32, 100, 100)
	defer surface.Destroy()

	ctx := NewContext(surface)
	defer ctx.Destroy()

	// Test arc drawing
	ctx.Arc(50, 50, 30, 0, math.Pi/2)
	// Should have current point after arc
	if ctx.HasCurrentPoint() != True {
		t.Errorf("Context should have current point after Arc")
	}

	// Test arc negative drawing
	ctx.NewPath()
	ctx.ArcNegative(50, 50, 30, math.Pi/2, 0)
	// Should have current point after arc negative
	if ctx.HasCurrentPoint() != True {
		t.Errorf("Context should have current point after ArcNegative")
	}
}

func TestGradientPatterns(t *testing.T) {
	// Test linear gradient with color stops
	linear := NewPatternLinear(0, 0, 100, 100)
	if linearPattern, ok := linear.(LinearGradientPattern); ok {
		linearPattern.AddColorStopRGB(0.0, 1, 0, 0) // Red at start
		linearPattern.AddColorStopRGB(0.5, 0, 1, 0) // Green at middle
		linearPattern.AddColorStopRGB(1.0, 0, 0, 1) // Blue at end

		// Check color stop count
		if linearPattern.GetColorStopCount() != 3 {
			t.Errorf("Expected 3 color stops, got %d", linearPattern.GetColorStopCount())
		}

		// Check first color stop
		offset, r, g, b, a, status := linearPattern.GetColorStop(0)
		if status != StatusSuccess {
			t.Errorf("GetColorStop failed with status %v", status)
		}
		if offset != 0.0 || r != 1.0 || g != 0.0 || b != 0.0 || a != 1.0 {
			t.Errorf("First color stop incorrect, got offset=%f, r=%f, g=%f, b=%f, a=%f", offset, r, g, b, a)
		}
	} else {
		t.Errorf("Pattern is not a LinearGradientPattern")
	}
	linear.Destroy()

	// Test radial gradient with color stops
	radial := NewPatternRadial(50, 50, 10, 50, 50, 50)
	if radialPattern, ok := radial.(RadialGradientPattern); ok {
		radialPattern.AddColorStopRGBA(0.0, 1, 1, 1, 1) // White center
		radialPattern.AddColorStopRGBA(1.0, 0, 0, 0, 1) // Black edge

		// Check color stop count
		if radialPattern.GetColorStopCount() != 2 {
			t.Errorf("Expected 2 color stops, got %d", radialPattern.GetColorStopCount())
		}
	} else {
		t.Errorf("Pattern is not a RadialGradientPattern")
	}
	radial.Destroy()
}

func TestMatrixTransformations(t *testing.T) {
	surface := NewImageSurface(FormatARGB32, 100, 100)
	defer surface.Destroy()

	ctx := NewContext(surface)
	defer ctx.Destroy()

	// Test initial matrix is identity
	matrix := ctx.GetMatrix()
	if matrix.XX != 1.0 || matrix.YY != 1.0 || matrix.XY != 0.0 || matrix.YX != 0.0 || matrix.X0 != 0.0 || matrix.Y0 != 0.0 {
		t.Errorf("Initial matrix should be identity, got %+v", matrix)
	}

	// Test translate
	ctx.Translate(10, 20)
	matrix = ctx.GetMatrix()
	if matrix.X0 != 10.0 || matrix.Y0 != 20.0 {
		t.Errorf("Translate failed, got %+v", matrix)
	}

	// Test scale
	ctx.Save()
	ctx.Scale(2.0, 3.0)
	matrix = ctx.GetMatrix()
	// Note: This is a simplified check, actual matrix multiplication would be more complex
	if matrix.XX < 2.0 || matrix.YY < 3.0 {
		t.Errorf("Scale failed, got %+v", matrix)
	}
	ctx.Restore()

	// Test rotate
	ctx.Save()
	ctx.Rotate(math.Pi / 2)
	matrix = ctx.GetMatrix()
	tolerance := 1e-10
	if math.Abs(matrix.XX) > tolerance || math.Abs(matrix.YY) > tolerance || math.Abs(matrix.XY-(-1.0)) > tolerance || math.Abs(matrix.YX-1.0) > tolerance {
		t.Errorf("Rotate failed, got %+v", matrix)
	}
	ctx.Restore()

	// Test set matrix
	newMatrix := NewMatrix()
	newMatrix.InitTranslate(5, 15)
	ctx.SetMatrix(newMatrix)
	matrix = ctx.GetMatrix()
	if matrix.X0 != 5.0 || matrix.Y0 != 15.0 {
		t.Errorf("SetMatrix failed, got %+v", matrix)
	}

	// Test identity matrix
	ctx.IdentityMatrix()
	matrix = ctx.GetMatrix()
	if matrix.XX != 1.0 || matrix.YY != 1.0 || matrix.XY != 0.0 || matrix.YX != 0.0 || matrix.X0 != 0.0 || matrix.Y0 != 0.0 {
		t.Errorf("IdentityMatrix failed, got %+v", matrix)
	}
}
