package gopdf

import "testing"

func TestMatrixMultiplyAffine(t *testing.T) {
	translate := NewTranslationMatrix(10, 20)
	scale := NewScaleMatrix(2, 3)

	m := translate.Multiply(scale)
	if m.XX != 2 || m.XY != 0 || m.YX != 0 || m.YY != 3 || m.X0 != 10 || m.Y0 != 20 {
		t.Fatalf("unexpected matrix: got %+v", *m)
	}

	x, y := m.Transform(1, 1)
	if x != 12 || y != 23 {
		t.Fatalf("unexpected transform: got (%.2f,%.2f), want (12,23)", x, y)
	}
}

func TestContextTransformOrder(t *testing.T) {
	surface := NewImageSurface(FormatARGB32, 10, 10)
	defer surface.Destroy()

	ctx := NewContext(surface)
	defer ctx.Destroy()

	ctx.Translate(10, 0)
	ctx.Scale(2, 1)

	x, y := ctx.UserToDevice(0, 0)
	if x != 10 || y != 0 {
		t.Fatalf("unexpected transform result: got (%.2f,%.2f), want (10,0)", x, y)
	}

	x, y = ctx.UserToDevice(1, 0)
	if x != 12 || y != 0 {
		t.Fatalf("unexpected transform result: got (%.2f,%.2f), want (12,0)", x, y)
	}
}
