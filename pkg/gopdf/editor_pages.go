package gopdf

import (
	"fmt"
	"strconv"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

func ReorderPagesFile(inFile, outFile string, pageOrder []int) error {
	if len(pageOrder) == 0 {
		return fmt.Errorf("missing page order")
	}
	selection := make([]string, 0, len(pageOrder))
	for _, p := range pageOrder {
		if p <= 0 {
			return fmt.Errorf("invalid page: %d", p)
		}
		selection = append(selection, strconv.Itoa(p))
	}
	conf := model.NewDefaultConfiguration()
	conf.Unit = types.POINTS
	return api.CollectFile(inFile, outFile, selection, conf)
}

func RemovePagesFile(inFile, outFile string, pages []int) error {
	if len(pages) == 0 {
		return fmt.Errorf("missing pages")
	}
	selection := make([]string, 0, len(pages))
	for _, p := range pages {
		if p <= 0 {
			return fmt.Errorf("invalid page: %d", p)
		}
		selection = append(selection, strconv.Itoa(p))
	}
	conf := model.NewDefaultConfiguration()
	conf.Unit = types.POINTS
	return api.RemovePagesFile(inFile, outFile, selection, conf)
}

func InsertBlankPagesFile(inFile, outFile string, pages []int, before bool, width, height float64) error {
	if len(pages) == 0 {
		return fmt.Errorf("missing pages")
	}
	selection := make([]string, 0, len(pages))
	for _, p := range pages {
		if p <= 0 {
			return fmt.Errorf("invalid page: %d", p)
		}
		selection = append(selection, strconv.Itoa(p))
	}
	conf := model.NewDefaultConfiguration()
	conf.Unit = types.POINTS

	var pageConf *pdfcpu.PageConfiguration
	if width > 0 && height > 0 {
		pageConf = &pdfcpu.PageConfiguration{PageDim: &types.Dim{Width: width, Height: height}}
	}

	return api.InsertPagesFile(inFile, outFile, selection, before, pageConf, conf)
}

