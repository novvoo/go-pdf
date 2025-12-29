package gopdf

import (
	"fmt"
	"sync"
)

// ConcurrentRenderer 并发渲染器，用于批量处理多个页面
type ConcurrentRenderer struct {
	reader     *PDFReader
	maxWorkers int
	workerPool chan struct{}
}

// RenderJob 渲染任务
type RenderJob struct {
	PageNum    int
	OutputPath string
	DPI        float64
}

// RenderResult 渲染结果
type RenderResult struct {
	PageNum int
	Error   error
}

// NewConcurrentRenderer 创建并发渲染器
func NewConcurrentRenderer(reader *PDFReader, maxWorkers int) *ConcurrentRenderer {
	if maxWorkers <= 0 {
		maxWorkers = 4 // 默认 4 个工作线程
	}

	return &ConcurrentRenderer{
		reader:     reader,
		maxWorkers: maxWorkers,
		workerPool: make(chan struct{}, maxWorkers),
	}
}

// RenderPages 并发渲染多个页面
func (cr *ConcurrentRenderer) RenderPages(jobs []RenderJob) []RenderResult {
	results := make([]RenderResult, len(jobs))
	var wg sync.WaitGroup

	for i, job := range jobs {
		wg.Add(1)

		// 获取工作线程槽位
		cr.workerPool <- struct{}{}

		go func(index int, j RenderJob) {
			defer wg.Done()
			defer func() { <-cr.workerPool }() // 释放槽位

			err := cr.reader.RenderPageToPNG(j.PageNum, j.OutputPath, j.DPI)
			results[index] = RenderResult{
				PageNum: j.PageNum,
				Error:   err,
			}

			if err != nil {
				LogError("Failed to render page %d: %v", j.PageNum, err)
			} else {
				Debug("Successfully rendered page %d to %s", j.PageNum, j.OutputPath)
			}
		}(i, job)
	}

	wg.Wait()
	return results
}

// RenderAllPages 并发渲染所有页面
func (cr *ConcurrentRenderer) RenderAllPages(outputDir string, dpi float64) error {
	pageCount, err := cr.reader.GetPageCount()
	if err != nil {
		return fmt.Errorf("failed to get page count: %w", err)
	}

	jobs := make([]RenderJob, pageCount)
	for i := 0; i < pageCount; i++ {
		jobs[i] = RenderJob{
			PageNum:    i + 1,
			OutputPath: fmt.Sprintf("%s/page_%d.png", outputDir, i+1),
			DPI:        dpi,
		}
	}

	results := cr.RenderPages(jobs)

	// 检查是否有错误
	var firstError error
	errorCount := 0
	for _, result := range results {
		if result.Error != nil {
			errorCount++
			if firstError == nil {
				firstError = result.Error
			}
		}
	}

	if errorCount > 0 {
		return fmt.Errorf("failed to render %d pages, first error: %w", errorCount, firstError)
	}

	return nil
}

// BatchRenderWithCallback 批量渲染并提供进度回调
func (cr *ConcurrentRenderer) BatchRenderWithCallback(
	jobs []RenderJob,
	progressCallback func(completed, total int),
) []RenderResult {
	results := make([]RenderResult, len(jobs))
	var wg sync.WaitGroup
	var mu sync.Mutex
	completed := 0

	for i, job := range jobs {
		wg.Add(1)
		cr.workerPool <- struct{}{}

		go func(index int, j RenderJob) {
			defer wg.Done()
			defer func() { <-cr.workerPool }()

			err := cr.reader.RenderPageToPNG(j.PageNum, j.OutputPath, j.DPI)
			results[index] = RenderResult{
				PageNum: j.PageNum,
				Error:   err,
			}

			// 更新进度
			mu.Lock()
			completed++
			current := completed
			mu.Unlock()

			if progressCallback != nil {
				progressCallback(current, len(jobs))
			}
		}(i, job)
	}

	wg.Wait()
	return results
}
