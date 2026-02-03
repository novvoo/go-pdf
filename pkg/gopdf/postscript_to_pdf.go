package gopdf

import (
	"fmt"
	"os"
	"path/filepath"
)

func ConvertPostScriptToPDF(psPath, pdfPath string) error {
	if psPath == "" || pdfPath == "" {
		return fmt.Errorf("missing psPath or pdfPath")
	}

	tmp, err := os.CreateTemp("", "gopdf_ps_*.svg")
	if err != nil {
		return err
	}
	tmpSVG := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpSVG)

	if err := ConvertPostScriptToSVG(psPath, tmpSVG); err != nil {
		return err
	}
	if err := ConvertSVGToPDF(tmpSVG, pdfPath); err != nil {
		return err
	}
	return nil
}

func ConvertPostScriptToPDFWithSVG(psPath, svgPath, pdfPath string) error {
	if psPath == "" || svgPath == "" || pdfPath == "" {
		return fmt.Errorf("missing psPath/svgPath/pdfPath")
	}
	if err := ConvertPostScriptToSVG(psPath, svgPath); err != nil {
		return err
	}
	return ConvertSVGToPDF(svgPath, pdfPath)
}

func ensureDirForFile(path string) error {
	if path == "" {
		return nil
	}
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0755)
}
