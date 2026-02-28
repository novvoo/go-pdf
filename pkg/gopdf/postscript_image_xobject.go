package gopdf

import (
	"fmt"
	"image"
)

func psRenderImageUnitSquare(c *context, img image.Image) error {
	s := c.psSurfaceTarget()
	if s == nil || s.writer == nil {
		return fmt.Errorf("ps surface not available")
	}
	s.ensureInPage()
	ps := s

	if img == nil {
		return nil
	}
	b := img.Bounds()
	wpx := b.Dx()
	hpx := b.Dy()
	if wpx <= 0 || hpx <= 0 {
		return nil
	}

	m := c.gstate.matrix
	m2XX := m.XX
	m2YX := m.YX
	m2XY := -m.XY
	m2YY := -m.YY
	m2X0 := m.X0 + m.XY
	m2Y0 := m.Y0 + m.YY

	fmt.Fprint(s.writer, "gsave\ninitmatrix\n")
	if ps != nil {
		fmt.Fprintf(s.writer, "0 %.8f translate\n1 -1 scale\n", ps.height)
	}
	fmt.Fprintf(s.writer, "/picstr %d string def\n", wpx*3)
	fmt.Fprintf(s.writer, "%d %d 8 [%.8f %.8f %.8f %.8f %.8f %.8f] { currentfile picstr readhexstring pop } false 3 colorimage\n", wpx, hpx, m2XX, m2YX, m2XY, m2YY, m2X0, m2Y0)
	if err := psWriteImageHexRGB(s.writer, img); err != nil {
		return err
	}
	fmt.Fprint(s.writer, "\n\ngrestore\n")
	return s.writer.Flush()
}
