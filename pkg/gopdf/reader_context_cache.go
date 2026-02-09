package gopdf

import (
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

func (r *PDFReader) getContext() (*model.Context, error) {
	if r == nil {
		return nil, nil
	}
	if r.contextCache != nil {
		return r.contextCache, nil
	}
	ctx, err := api.ReadContextFile(r.pdfPath)
	if err != nil {
		return nil, err
	}
	r.contextCache = ctx
	return ctx, nil
}

