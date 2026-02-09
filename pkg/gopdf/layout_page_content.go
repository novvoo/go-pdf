package gopdf

import (
	"fmt"
)

func (r *PDFReader) GetPageContent(pageNum int) ([]byte, error) {
	ctx, err := r.getContext()
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		return nil, fmt.Errorf("nil context")
	}
	pageDict, _, _, err := ctx.PageDict(pageNum, true)
	if err != nil {
		return nil, err
	}
	content, err := ctx.PageContent(pageDict, pageNum)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(content))
	copy(out, content)

	return out, nil
}
