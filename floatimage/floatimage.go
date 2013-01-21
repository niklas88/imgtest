// Package floatimage implements a basic image like data structure that
// holds all data as float32 values. It is loosely based on the Go image
// library but doesn't actually implement the image.Image interface
// this is done to make functions easier inlineable.
//
// FloatImgs use a dynamic channel count so they can be used
// for example to hold different spatial derivatives or a vector field
// as used for example during optic flow analysis.
// At the moment there is only an interface to the Go image world for
// single channel FloatImgs that store gray values which are mapped to/from color.Gray
package floatimage

import (
	"image"
	"image/color"
	"math"
)

// ToColorFunc is applied on a float32 array representing
// one image point so as to transform it into an image color
type ToColorFunc func(x, y int, data []float32) color.Color

// ColorModelFunc returns the color.Model used
type ColorModelFunc func() color.Model

// FloatImg holds float32 image like data with possibly multiple channels
type FloatImg struct {
	Pix            []float32
	Stride         int
	Rect           image.Rectangle
	Chancnt        int
	ColorFunc      ToColorFunc
	ColorModelFunc ColorModelFunc
}

// NewFloatImg creates a new FloatImage with covering the given rectangle and having
// channelCount channels
func NewFloatImg(r image.Rectangle, channelCount int) *FloatImg {
	w, h := r.Dx(), r.Dy()
	pix := make([]float32, channelCount*w*h)
	colorModelFunc := func() (m color.Model) {
		switch channelCount {
		case 4:
			m = color.RGBAModel
		case 3:
			m = color.RGBAModel
		case 2:
			m = color.YCbCrModel
		case 1:
			m = color.GrayModel
		default:
			m = color.GrayModel
		}
		return
	}
	return &FloatImg{pix, channelCount * w, r,
		channelCount, StandardColorFunc,
		colorModelFunc}
}

// Implements the ColorModel function of the image interface
func (p *FloatImg) ColorModel() color.Model {
	return p.ColorModelFunc()
}

// PixOffset computes the offset for the first element at position x, y
func (p *FloatImg) PixOffset(x, y int) int {
	return (y-p.Rect.Min.Y)*p.Stride + (x-p.Rect.Min.X)*p.Chancnt
}

// AtF gets a []float32 corresponing to the channels at position x,y the values
// at this postion can be manipulated using the returned slice
func (p *FloatImg) AtF(x, y int) []float32 {
	i := p.PixOffset(x, y)
	return p.Pix[i : i+p.Chancnt]
}

// At implements image.Image by applying ColorFunc on the
// given image point
func (p *FloatImg) At(x, y int) color.Color {
	if !(image.Point{x, y}.In(p.Rect)) {
		return color.Black
	}
	i := p.PixOffset(x, y)
	return p.ColorFunc(x, y, p.Pix[i:i+p.Chancnt])
}

// Set sets the float32 value val at position x,y for channel c
func (p *FloatImg) Set(x, y, c int, val float32) {
	i := p.PixOffset(x, y)
	p.Pix[i+c] = val
}

// Copy copies the content of the original image into this image
// adjusting size as necessary
func (p *FloatImg) Copy(orig *FloatImg) {
	p.Rect = orig.Rect
	p.Stride = orig.Stride
	p.Chancnt = orig.Chancnt
	copy(p.Pix, orig.Pix)
}

// Dummies sets the outer most row and column to mirroring boundary conditions
func (f *FloatImg) Dummies() {
	bounds := f.Bounds()
	for x := bounds.Min.X + 1; x < bounds.Max.X-1; x++ {
		chansUp := f.AtF(x, bounds.Min.Y+1)
		chansLow := f.AtF(x, bounds.Max.Y-2)
		chansOutUp := f.AtF(x, bounds.Min.Y)
		chansOutLow := f.AtF(x, bounds.Max.Y-1)
		for c := 0; c < f.Chancnt; c++ {
			chansOutLow[c] = chansLow[c]
			chansOutUp[c] = chansUp[c]
		}
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		chansLeft := f.AtF(bounds.Min.X+1, y)
		chansRight := f.AtF(bounds.Max.X-2, y)
		chansOutLeft := f.AtF(bounds.Min.X, y)
		chansOutRight := f.AtF(bounds.Max.X-1, y)
		for c := 0; c < f.Chancnt; c++ {
			chansOutLeft[c] = chansLeft[c]
			chansOutRight[c] = chansRight[c]
		}
	}
}

// Bounds gets the Rect that the FloatImg covers
func (p *FloatImg) Bounds() image.Rectangle { return p.Rect }

// GrayFloatWithDummiesFromImage Creates a FloatImage from the given Image, mapping
// all colors to Gray float32 values in the range 0.0 <= val <= 255.0
func GrayFloatWithDummiesFromImage(img image.Image) (f *FloatImg) {
	bounds := img.Bounds()
	p1 := image.Point{bounds.Min.X - 1, bounds.Min.Y - 1}
	p2 := image.Point{bounds.Max.X + 1, bounds.Max.Y + 1}

	realBounds := image.Rectangle{p1, p2}

	f = NewFloatImg(realBounds, 1)

	var gray uint8

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.At(x, y)
			switch t := c.(type) {
			case color.Gray:
				gray = t.Y
			default:
				r, g, b, _ := c.RGBA()
				gray = uint8(((299*r + 587*g + 114*b + 500) / 1000) >> 8)
			}
			f.Set(x, y, 0, float32(gray))
		}
	}

	// Create some dummy borders with mirroring
	f.Dummies()
	return
}

// Converts to uint8 by truncating to 0 <= val <= 255.0
func Tu8c(d float32) uint8 {
	var c uint8
	switch {
	case d < 0.0:
		c = 0
	case d > 255.0:
		c = 255
	default:
		c = uint8(d)
	}
	return c
}

// Standard function to convert float[] at image point to color.Color
func StandardColorFunc(x, y int, data []float32) (c color.Color) {
	switch len(data) {
	case 4:
		c = color.RGBA{Tu8c(data[0]), Tu8c(data[1]), Tu8c(data[2]), Tu8c(data[3])}
	case 3:
		c = color.RGBA{Tu8c(data[0]), Tu8c(data[1]), Tu8c(data[2]), 0}
	case 2:
		c = color.YCbCr{128, Tu8c(data[0]), Tu8c(data[1])}
	case 1:
		c = color.Gray{Tu8c(data[0])}
	default:
		c = color.Gray{0}
	}
	return
}

// SubImage creates a *FloatImg for a region within the original image
// this image is backed by the data in the original image which thus can
// be manipulated by manipulating the sub image
func (p *FloatImg) SubImage(r image.Rectangle) *FloatImg {
	r = r.Intersect(p.Rect)
	// If r1 and r2 are Rectangles, r1.Intersect(r2) is not guaranteed to be inside
	// either r1 or r2 if the intersection is empty. Without explicitly checking for
	// this, the Pix[i:] expression below can panic.
	if r.Empty() {
		return NewFloatImg(r, 0)
	}
	i := p.PixOffset(r.Min.X, r.Min.Y)
	return &FloatImg{
		Pix:            p.Pix[i:],
		Stride:         p.Stride,
		Rect:           r,
		Chancnt:        p.Chancnt,
		ColorFunc:      p.ColorFunc,
		ColorModelFunc: p.ColorModelFunc}
}

// Returns the sub image that excludes the 1 pixel dummy borders
func (p *FloatImg) Dedummify() *FloatImg {
	bounds := p.Bounds()
	p1 := image.Point{bounds.Min.X + 1, bounds.Min.Y + 1}
	p2 := image.Point{bounds.Max.X - 1, bounds.Max.Y - 1}

	realBounds := image.Rectangle{p1, p2}

	return p.SubImage(realBounds)
}

// Scales all channels to the 0 <= val <= 255 range, assumes >= 0 values
func (p *FloatImg) ScaleToUnsignedByte() {
	bounds := p.Bounds()
	max := make([]float32, p.Chancnt)
	for i, _ := range max {
		max[i] = -1.0 * math.MaxFloat32
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			for c := 0; c < p.Chancnt; c++ {
				v := p.AtF(x, y)[c]
				if v > max[c] {
					max[c] = v
				}
			}
		}
	}

	var help float32
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			for c := 0; c < p.Chancnt; c++ {
				help = 255 * p.AtF(x, y)[c] / max[c]
				p.Set(x, y, c, help)
			}
		}
	}
}
