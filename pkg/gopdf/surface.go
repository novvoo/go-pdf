package gopdf

import (
	"bufio"
	"fmt"
	"image"
	"image/png"
	"os"
	"runtime" // Added for SetFinalizer
	"sync"
	"sync/atomic"
	"unsafe"
)

// surfaceDataPool is a sync.Pool for recycling image surface data buffers.
var surfaceDataPool = sync.Pool{
	New: func() interface{} {
		// Return nil, as we need to size the slice dynamically
		return nil
	},
}

// imageSurface implements image-based surfaces
type imageSurface struct {
	baseSurface

	// Image data (ARGB32)
	data   []byte
	width  int
	height int
	stride int
	format Format

	// RGBA buffer for image interoperability
	rgbaData  []byte
	rgbaImage *image.RGBA
	goImage   image.Image
}

// baseSurface provides common surface functionality
type baseSurface struct {
	refCount    int32
	status      Status
	surfaceType SurfaceType
	content     Content

	// Device properties
	device Device

	// User data
	userData map[*UserDataKey]interface{}

	// Font options
	fontOptions *FontOptions

	// Transform properties
	deviceTransform        Matrix
	deviceTransformInverse Matrix
	deviceOffsetX          float64
	deviceOffsetY          float64
	deviceScaleX           float64
	deviceScaleY           float64

	// Fallback resolution
	fallbackResolutionX float64
	fallbackResolutionY float64

	// Surface state
	finished bool

	// Snapshots
	snapshots []Surface
}

// NewImageSurface creates a new image surface
func NewImageSurface(format Format, width, height int) Surface {
	if width <= 0 || height <= 0 {
		return newSurfaceInError(StatusInvalidSize)
	}

	stride := formatStrideForWidth(format, width)
	if stride < 0 {
		return newSurfaceInError(StatusInvalidStride)
	}

	// Try to get a buffer from the pool
	size := stride * height
	var data []byte
	if v := surfaceDataPool.Get(); v != nil {
		if buf, ok := v.([]byte); ok && cap(buf) >= size {
			data = buf[:size]
		}
	}
	if data == nil {
		data = make([]byte, size)
	}

	surface := &imageSurface{
		baseSurface: baseSurface{
			refCount:            1,
			status:              StatusSuccess,
			surfaceType:         SurfaceTypeImage,
			content:             formatToContent(format),
			userData:            make(map[*UserDataKey]interface{}),
			fontOptions:         &FontOptions{},
			deviceScaleX:        1.0,
			deviceScaleY:        1.0,
			fallbackResolutionX: 72.0,
			fallbackResolutionY: 72.0,
		},
		data:   data,
		width:  width,
		height: height,
		stride: stride,
		format: format,
	}

	// Initialize transforms
	surface.deviceTransform.InitIdentity()
	surface.deviceTransformInverse.InitIdentity()

	// Create Go image for interoperability
	surface.createGoImage()

	runtime.SetFinalizer(surface, (*imageSurface).Destroy)
	return surface
}

// NewImageSurfaceForData creates a surface using existing data
func NewImageSurfaceForData(data []byte, format Format, width, height, stride int) Surface {
	if width <= 0 || height <= 0 {
		return newSurfaceInError(StatusInvalidSize)
	}

	if stride < formatStrideForWidth(format, width) {
		return newSurfaceInError(StatusInvalidStride)
	}

	if len(data) < stride*height {
		return newSurfaceInError(StatusInvalidSize)
	}

	surface := &imageSurface{
		baseSurface: baseSurface{
			refCount:            1,
			status:              StatusSuccess,
			surfaceType:         SurfaceTypeImage,
			content:             formatToContent(format),
			userData:            make(map[*UserDataKey]interface{}),
			fontOptions:         &FontOptions{},
			deviceScaleX:        1.0,
			deviceScaleY:        1.0,
			fallbackResolutionX: 72.0,
			fallbackResolutionY: 72.0,
		},
		data:   data,
		width:  width,
		height: height,
		stride: stride,
		format: format,
	}

	surface.deviceTransform.InitIdentity()
	surface.deviceTransformInverse.InitIdentity()
	surface.createGoImage()

	runtime.SetFinalizer(surface, (*imageSurface).Destroy)
	return surface
}

func newSurfaceInError(status Status) Surface {
	surface := &imageSurface{
		baseSurface: baseSurface{
			refCount: 1,
			status:   status,
			userData: make(map[*UserDataKey]interface{}),
		},
	}
	// Don't set finalizer for error surfaces to avoid nil pointer issues
	// runtime.SetFinalizer(surface, (*imageSurface).Destroy)
	return surface
}

// Helper functions

func formatStrideForWidth(format Format, width int) int {
	switch format {
	case FormatARGB32, FormatRGB24:
		return width * 4
	case FormatA8:
		return width
	case FormatA1:
		return (width + 31) / 32 * 4 // Round up to 32-bit boundary
	case FormatRGB16565:
		return width * 2
	case FormatRGB30:
		return width * 4
	case FormatRGB96F:
		return width * 12 // 3 * 4 bytes per pixel
	case FormatRGBA128F:
		return width * 16 // 4 * 4 bytes per pixel
	default:
		return -1
	}
}

func formatToContent(format Format) Content {
	switch format {
	case FormatARGB32, FormatRGBA128F:
		return ContentColorAlpha
	case FormatRGB24, FormatRGB16565, FormatRGB30, FormatRGB96F:
		return ContentColor
	case FormatA8, FormatA1:
		return ContentAlpha
	default:
		return ContentColorAlpha
	}
}

func (s *imageSurface) createGoImage() {
	if s.format != FormatARGB32 {
		return
	}

	size := s.stride * s.height
	s.rgbaData = make([]byte, size)
	s.rgbaImage = &image.RGBA{
		Pix:    s.rgbaData,
		Stride: s.stride,
		Rect:   image.Rect(0, 0, s.width, s.height),
	}
	s.goImage = s.rgbaImage
}

// baseSurface implementation

func (s *baseSurface) Reference() Surface {
	atomic.AddInt32(&s.refCount, 1)
	return s
}

func (s *baseSurface) Destroy() {
	if atomic.AddInt32(&s.refCount, -1) == 0 {
		s.cleanup()
	}
}

func (s *baseSurface) cleanup() {
	if s.device != nil {
		s.device.Destroy()
	}
}

func (s *baseSurface) GetReferenceCount() int {
	return int(atomic.LoadInt32(&s.refCount))
}

func (s *baseSurface) Status() Status {
	return s.status
}

func (s *baseSurface) GetType() SurfaceType {
	return s.surfaceType
}

func (s *baseSurface) GetContent() Content {
	return s.content
}

func (s *baseSurface) GetDevice() Device {
	return s.device
}

func (s *baseSurface) SetUserData(key *UserDataKey, userData unsafe.Pointer, destroy DestroyFunc) Status {
	if s.status != StatusSuccess {
		return s.status
	}

	s.userData[key] = userData
	// TODO: Store destroy function and call it when appropriate
	return StatusSuccess
}

func (s *baseSurface) GetUserData(key *UserDataKey) unsafe.Pointer {
	if data, exists := s.userData[key]; exists {
		return data.(unsafe.Pointer)
	}
	return nil
}

func (s *baseSurface) Flush() error {
	// Default implementation does nothing
	return nil
}

func (s *baseSurface) MarkDirty() {
	// Default implementation does nothing
	// Image surfaces override this method
}

func (s *baseSurface) MarkDirtyRectangle(x, y, width, height int) {
	// Default implementation does nothing
	// Image surfaces override this method
}

func (s *baseSurface) GetFontOptions() *FontOptions {
	return s.fontOptions
}

func (s *baseSurface) Finish() error {
	if s.finished {
		return nil
	}
	s.finished = true

	// Clean up snapshots
	for _, snapshot := range s.snapshots {
		snapshot.Destroy()
	}
	s.snapshots = nil

	// Call concrete surface finish
	return s.finishConcrete()
}

func (s *baseSurface) finishConcrete() error {
	// Default implementation does nothing
	return nil
}

func (s *baseSurface) CreateSimilar(content Content, width, height int) Surface {
	// Default implementation creates an image surface
	if s.surfaceType == SurfaceTypeRecording {
		return NewRecordingSurface(content, float64(width), float64(height))
	}
	var format Format
	switch content {
	case ContentColor:
		format = FormatRGB24
	case ContentAlpha:
		format = FormatA8
	case ContentColorAlpha:
		format = FormatARGB32
	default:
		return newSurfaceInError(StatusInvalidContent)
	}

	return NewImageSurface(format, width, height)
}

func (s *baseSurface) CreateSimilarImage(format Format, width, height int) Surface {
	return NewImageSurface(format, width, height)
}

func (s *baseSurface) CreateForRectangle(x, y, width, height float64) Surface {
	// TODO: Implement subsurface creation
	return s.CreateSimilar(s.content, int(width), int(height))
}

func (s *baseSurface) SetDeviceScale(xScale, yScale float64) {
	// 防止除零错误
	if xScale == 0 {
		xScale = 1.0
	}
	if yScale == 0 {
		yScale = 1.0
	}

	s.deviceScaleX = xScale
	s.deviceScaleY = yScale

	// Update transform matrices
	s.deviceTransform.InitScale(xScale, yScale)
	s.deviceTransformInverse.InitScale(1.0/xScale, 1.0/yScale)
}

func (s *baseSurface) GetDeviceScale() (xScale, yScale float64) {
	return s.deviceScaleX, s.deviceScaleY
}

func (s *baseSurface) SetDeviceOffset(xOffset, yOffset float64) {
	s.deviceOffsetX = xOffset
	s.deviceOffsetY = yOffset

	// Update transform matrices
	s.deviceTransform.InitTranslate(xOffset, yOffset)
	s.deviceTransformInverse.InitTranslate(-xOffset, -yOffset)
}

func (s *baseSurface) GetDeviceOffset() (xOffset, yOffset float64) {
	return s.deviceOffsetX, s.deviceOffsetY
}

func (s *baseSurface) SetFallbackResolution(xPixelsPerInch, yPixelsPerInch float64) {
	s.fallbackResolutionX = xPixelsPerInch
	s.fallbackResolutionY = yPixelsPerInch
}

func (s *baseSurface) GetFallbackResolution() (xPixelsPerInch, yPixelsPerInch float64) {
	return s.fallbackResolutionX, s.fallbackResolutionY
}

func (s *baseSurface) CopyPage() {
	// Default implementation does nothing (only meaningful for paginated surfaces)
}

func (s *baseSurface) ShowPage() {
	// Default implementation does nothing (only meaningful for paginated surfaces)
}

// imageSurface specific implementation

func (s *imageSurface) Reference() Surface {
	atomic.AddInt32(&s.refCount, 1)
	return s
}

// MarkDirty converts from premultiplied to non-premultiplied alpha
func (s *imageSurface) MarkDirty() {
	s.unpremultiplyAlpha()
}

// MarkDirtyRectangle converts a rectangle from premultiplied to non-premultiplied alpha
func (s *imageSurface) MarkDirtyRectangle(x, y, width, height int) {
	s.unpremultiplyAlphaRect(x, y, width, height)
}

// Image surface specific methods

func (s *imageSurface) GetData() []byte {
	return s.data
}

func (s *imageSurface) GetWidth() int {
	return s.width
}

func (s *imageSurface) GetHeight() int {
	return s.height
}

func (s *imageSurface) GetStride() int {
	return s.stride
}

func (s *imageSurface) GetFormat() Format {
	return s.format
}

func (s *imageSurface) GetGoImage() image.Image {
	return s.goImage
}

// unpremultiplyAlpha converts the entire surface from premultiplied to non-premultiplied alpha
func (s *imageSurface) unpremultiplyAlpha() {
	if s.format != FormatARGB32 {
		return
	}
	s.unpremultiplyAlphaRect(0, 0, s.width, s.height)
}

// unpremultiplyAlphaRect converts a rectangle from premultiplied to non-premultiplied alpha
func (s *imageSurface) unpremultiplyAlphaRect(x, y, width, height int) {
	if s.format != FormatARGB32 || s.rgbaImage == nil {
		return
	}

	// Clamp to surface bounds
	if x < 0 {
		width += x
		x = 0
	}
	if y < 0 {
		height += y
		y = 0
	}
	if x+width > s.width {
		width = s.width - x
	}
	if y+height > s.height {
		height = s.height - y
	}
	if width <= 0 || height <= 0 {
		return
	}

	stride := s.stride
	for row := y; row < y+height; row++ {
		argbOff := row*stride + x*4
		rgbaOff := row*stride + x*4
		argbPtr := s.data[argbOff:]
		rgbaPtr := s.rgbaData[rgbaOff:]

		for col := 0; col < width; col++ {
			i := col * 4
			a := argbPtr[i+0]
			r := argbPtr[i+1]
			g := argbPtr[i+2]
			b := argbPtr[i+3]

			// Convert from premultiplied to non-premultiplied alpha
			if a == 0 {
				rgbaPtr[i+0] = 0
				rgbaPtr[i+1] = 0
				rgbaPtr[i+2] = 0
				rgbaPtr[i+3] = 0
			} else if a == 255 {
				rgbaPtr[i+0] = r
				rgbaPtr[i+1] = g
				rgbaPtr[i+2] = b
				rgbaPtr[i+3] = a
			} else {
				// Unpremultiply: color = color * 255 / alpha
				rgbaPtr[i+0] = uint8((uint32(r) * 255) / uint32(a))
				rgbaPtr[i+1] = uint8((uint32(g) * 255) / uint32(a))
				rgbaPtr[i+2] = uint8((uint32(b) * 255) / uint32(a))
				rgbaPtr[i+3] = a
			}
		}
	}
}

// WriteToPNG writes the surface to a PNG file
func (s *imageSurface) WriteToPNG(filename string) Status {
	if s.status != StatusSuccess {
		return s.status
	}

	if s.goImage == nil {
		return StatusSurfaceTypeMismatch
	}

	file, err := os.Create(filename)
	if err != nil {
		return StatusWriteError
	}
	defer file.Close()

	err = png.Encode(file, s.goImage)
	if err != nil {
		return StatusWriteError
	}

	return StatusSuccess
}

// Format utilities

func FormatStrideForWidth(format Format, width int) int {
	return formatStrideForWidth(format, width)
}

// LoadPNGSurface creates an image surface from a PNG file
func LoadPNGSurface(filename string) (Surface, error) {
	file, err := os.Open(filename)
	if err != nil {
		return newSurfaceInError(StatusFileNotFound), err
	}
	defer file.Close()

	img, err := png.Decode(file)
	if err != nil {
		return newSurfaceInError(StatusReadError), err
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	surface := NewImageSurface(FormatARGB32, width, height).(*imageSurface)

	// Copy image data to RGBA buffer
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			surface.rgbaImage.Set(x, y, img.At(bounds.Min.X+x, bounds.Min.Y+y))
		}
	}

	return surface, nil
}

// Surface-specific interfaces for type assertions

type ImageSurface interface {
	Surface
	GetData() []byte
	GetWidth() int
	GetHeight() int
	GetStride() int
	GetFormat() Format
	GetGoImage() image.Image
	WriteToPNG(filename string) Status
}

// pdfSurface implements PDF output surface
type pdfSurface struct {
	baseSurface
	filename      string
	width, height float64
}

// svgSurface implements SVG output surface
type svgSurface struct {
	baseSurface
	filename      string
	width, height float64
}

// psSurface implements PostScript output surface (pure Go)
type psSurface struct {
	baseSurface
	filename      string
	width, height float64
	pageCount     int
	inPage        bool
	file          *os.File
	writer        *bufio.Writer
}

// scriptSurface implements Script surface (JSON serialization)
type scriptSurface struct {
	baseSurface
	filename      string
	width, height float64
	file          *os.File
	commands      []map[string]interface{}
}

// NewPDFSurface creates a new PDF surface
func NewPDFSurface(filename string, widthInPoints, heightInPoints float64) Surface {
	surface := &pdfSurface{
		baseSurface: baseSurface{
			refCount:            1,
			status:              StatusSuccess,
			surfaceType:         SurfaceTypePDF,
			content:             ContentColorAlpha,
			userData:            make(map[*UserDataKey]interface{}),
			fontOptions:         &FontOptions{},
			fallbackResolutionX: 72.0,
			fallbackResolutionY: 72.0,
		},
		filename: filename,
		width:    widthInPoints,
		height:   heightInPoints,
	}
	surface.deviceTransform.InitIdentity()
	surface.deviceTransformInverse.InitIdentity()
	return surface
}

// NewSVGSurface creates a new SVG surface
func NewSVGSurface(filename string, widthInPoints, heightInPoints float64) Surface {
	surface := &svgSurface{
		baseSurface: baseSurface{
			refCount:            1,
			status:              StatusSuccess,
			surfaceType:         SurfaceTypeSVG,
			content:             ContentColorAlpha,
			userData:            make(map[*UserDataKey]interface{}),
			fontOptions:         &FontOptions{},
			fallbackResolutionX: 72.0,
			fallbackResolutionY: 72.0,
		},
		filename: filename,
		width:    widthInPoints,
		height:   heightInPoints,
	}
	surface.deviceTransform.InitIdentity()
	surface.deviceTransformInverse.InitIdentity()
	return surface
}

// NewPSSurface creates a new PostScript surface (pure Go implementation)
func NewPSSurface(filename string, widthInPoints, heightInPoints float64) Surface {
	if widthInPoints <= 0 || heightInPoints <= 0 {
		return newSurfaceInError(StatusInvalidSize)
	}

	file, err := os.Create(filename)
	if err != nil {
		return newSurfaceInError(StatusWriteError)
	}

	writer := bufio.NewWriter(file)

	header := fmt.Sprintf(`%%!PS-Adobe-3.0
%%Creator: go-pdf
%%Title: %s
%%Pages: (atend)
%%BoundingBox: 0 0 %.0f %.0f
%%EndComments

gsave
1 setlinecap
1 setlinejoin
10 setmiterlimit

/newfont { /Helvetica findfont exch scalefont setfont } def
10 newfont

`, filename, widthInPoints, heightInPoints)

	_, err = writer.WriteString(header)
	if err != nil {
		file.Close()
		os.Remove(filename)
		return newSurfaceInError(StatusWriteError)
	}
	err = writer.Flush()
	if err != nil {
		file.Close()
		os.Remove(filename)
		return newSurfaceInError(StatusWriteError)
	}

	surface := &psSurface{
		baseSurface: baseSurface{
			refCount:            1,
			status:              StatusSuccess,
			surfaceType:         SurfaceTypePS,
			content:             ContentColorAlpha,
			userData:            make(map[*UserDataKey]interface{}),
			fontOptions:         NewFontOptions(),
			deviceScaleX:        1.0,
			deviceScaleY:        1.0,
			fallbackResolutionX: 72.0,
			fallbackResolutionY: 72.0,
		},
		filename:  filename,
		width:     widthInPoints,
		height:    heightInPoints,
		file:      file,
		writer:    writer,
		pageCount: 0,
		inPage:    false,
	}

	surface.deviceTransform.InitIdentity()
	surface.deviceTransformInverse.InitIdentity()

	runtime.SetFinalizer(surface, (*psSurface).Destroy)

	return surface
}

// PDFSurface implementation

func (s *pdfSurface) Reference() Surface {
	atomic.AddInt32(&s.refCount, 1)
	return s
}

func (s *pdfSurface) GetWidth() float64 {
	return s.width
}

func (s *pdfSurface) GetHeight() float64 {
	return s.height
}

// SVGSurface implementation

func (s *svgSurface) Reference() Surface {
	atomic.AddInt32(&s.refCount, 1)
	return s
}

func (s *svgSurface) GetWidth() float64 {
	return s.width
}

func (s *svgSurface) GetHeight() float64 {
	return s.height
}

func (s *psSurface) Reference() Surface {
	atomic.AddInt32(&s.refCount, 1)
	return s
}

func (s *psSurface) Destroy() {
	if atomic.AddInt32(&s.refCount, -1) == 0 {
		s.finishConcrete()
		s.cleanup()
	}
}

func (s *psSurface) GetWidth() float64 {
	return s.width
}

func (s *psSurface) GetHeight() float64 {
	return s.height
}

func (s *psSurface) CopyPage() {
	if s.writer != nil {
		s.writer.WriteString("copypage\n")
		s.writer.Flush()
	}
	s.pageCount++
}

func (s *psSurface) ShowPage() {
	if s.writer != nil {
		s.writer.WriteString("showpage grestore grestore\n")
		s.writer.Flush()
	}
	s.inPage = false
}

func (s *psSurface) SetSize(widthInPoints, heightInPoints float64) {
	s.width = widthInPoints
	s.height = heightInPoints
	if s.writer != nil {
		s.writer.WriteString(fmt.Sprintf("%%%%PageBoundingBox: 0 0 %.0f %.0f\n", widthInPoints, heightInPoints))
		s.writer.Flush()
	}
}

func (s *psSurface) DscComment(comment string) {
	if s.writer != nil {
		s.writer.WriteString(fmt.Sprintf("%%%% %s\n", comment))
		s.writer.Flush()
	}
}

func (s *psSurface) finishConcrete() error {
	if s.writer != nil {
		s.writer.WriteString(fmt.Sprintf("\ngrestore\n%%%%Trailer\n%%%%Pages: %d\n%%%%EOF\n", s.pageCount))
		s.writer.Flush()
		s.writer = nil
	}
	if s.file != nil {
		s.file.Close()
		s.file = nil
	}
	return nil
}

// NewScriptSurface creates a new Script surface for JSON serialization
func NewScriptSurface(filename string, width, height float64) Surface {
	if width <= 0 || height <= 0 {
		return newSurfaceInError(StatusInvalidSize)
	}

	file, err := os.Create(filename)
	if err != nil {
		return newSurfaceInError(StatusWriteError)
	}

	surface := &scriptSurface{
		baseSurface: baseSurface{
			refCount:            1,
			status:              StatusSuccess,
			surfaceType:         SurfaceTypeScript,
			content:             ContentColorAlpha,
			userData:            make(map[*UserDataKey]interface{}),
			fontOptions:         NewFontOptions(),
			deviceScaleX:        1.0,
			deviceScaleY:        1.0,
			fallbackResolutionX: 72.0,
			fallbackResolutionY: 72.0,
		},
		filename: filename,
		width:    width,
		height:   height,
		file:     file,
		commands: make([]map[string]interface{}, 0),
	}

	surface.deviceTransform.InitIdentity()
	surface.deviceTransformInverse.InitIdentity()

	runtime.SetFinalizer(surface, (*scriptSurface).Destroy)

	return surface
}

func (s *scriptSurface) Reference() Surface {
	atomic.AddInt32(&s.refCount, 1)
	return s
}

func (s *scriptSurface) Destroy() {
	if atomic.AddInt32(&s.refCount, -1) == 0 {
		s.finishConcrete()
		s.cleanup()
	}
}

func (s *scriptSurface) GetWidth() float64 {
	return s.width
}

func (s *scriptSurface) GetHeight() float64 {
	return s.height
}

func (s *scriptSurface) AddCommand(cmd map[string]interface{}) {
	s.commands = append(s.commands, cmd)
}
