package gopdf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	pdfform "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/form"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

func FillFormFieldsMapFile(inFilePDF, outFilePDF string, values map[string]string) error {
	if inFilePDF == "" || outFilePDF == "" {
		return fmt.Errorf("missing inFilePDF or outFilePDF")
	}
	if len(values) == 0 {
		return fmt.Errorf("missing values")
	}

	conf := model.NewDefaultConfiguration()

	f, err := os.Open(inFilePDF)
	if err != nil {
		return err
	}
	formGroup, err := api.ExportForm(f, inFilePDF, conf)
	f.Close()
	if err != nil {
		return err
	}
	if formGroup == nil || len(formGroup.Forms) == 0 {
		return fmt.Errorf("missing form data")
	}

	changed := applyFormValues(&formGroup.Forms[0], values)
	if changed == 0 {
		return fmt.Errorf("no form fields affected")
	}

	b, err := json.Marshal(formGroup)
	if err != nil {
		return err
	}

	rs, err := os.Open(inFilePDF)
	if err != nil {
		return err
	}
	defer rs.Close()

	w, err := os.Create(outFilePDF)
	if err != nil {
		return err
	}
	if fillErr := api.FillForm(rs, bytes.NewReader(b), w, conf); fillErr != nil {
		w.Close()
		os.Remove(outFilePDF)
		return fillErr
	}
	return w.Close()
}

func applyFormValues(f *pdfform.Form, values map[string]string) int {
	if f == nil {
		return 0
	}

	normalize := func(s string) string { return strings.TrimSpace(s) }
	get := func(name string) (string, bool) {
		v, ok := values[name]
		return normalize(v), ok
	}

	affected := 0

	for i := range f.TextFields {
		tf := f.TextFields[i]
		if tf == nil {
			continue
		}
		if v, ok := get(tf.Name); ok {
			tf.Value = v
			affected++
		}
	}

	for i := range f.DateFields {
		df := f.DateFields[i]
		if df == nil {
			continue
		}
		if v, ok := get(df.Name); ok {
			df.Value = v
			affected++
		}
	}

	for i := range f.CheckBoxes {
		cb := f.CheckBoxes[i]
		if cb == nil {
			continue
		}
		if v, ok := get(cb.Name); ok {
			cb.Value = parseBoolLoose(v)
			affected++
		}
	}

	for i := range f.RadioButtonGroups {
		rbg := f.RadioButtonGroups[i]
		if rbg == nil {
			continue
		}
		if v, ok := get(rbg.Name); ok {
			rbg.Value = v
			affected++
		}
	}

	for i := range f.ComboBoxes {
		cb := f.ComboBoxes[i]
		if cb == nil {
			continue
		}
		if v, ok := get(cb.Name); ok {
			cb.Value = v
			affected++
		}
	}

	for i := range f.ListBoxes {
		lb := f.ListBoxes[i]
		if lb == nil {
			continue
		}
		if v, ok := get(lb.Name); ok {
			parts := splitListValues(v)
			if len(parts) == 0 {
				parts = []string{""}
			}
			lb.Values = parts
			affected++
		}
	}

	return affected
}

func parseBoolLoose(s string) bool {
	t := strings.ToLower(strings.TrimSpace(s))
	if t == "1" || t == "true" || t == "t" || t == "yes" || t == "y" || t == "on" {
		return true
	}
	if t == "0" || t == "false" || t == "f" || t == "no" || t == "n" || t == "off" || t == "" {
		return false
	}
	if i, err := strconv.Atoi(t); err == nil {
		return i != 0
	}
	return false
}

func splitListValues(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	sep := ","
	if strings.Contains(s, ";") {
		sep = ";"
	}
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}
