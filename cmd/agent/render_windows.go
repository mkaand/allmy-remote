package main

import (
	"bytes"
	_ "embed"
	"image"
	"image/png"
	"math"
	"runtime"
	"unsafe"
)

//go:embed branding/logo.png
var lightLogo []byte

//go:embed branding/logo-dark.png
var darkLogo []byte

type raster struct {
	width, height int
	pixels        []uint32
}

type bitmapHeader struct {
	Size          uint32
	Width, Height int32
	Planes, Bits  uint16
	Compression   uint32
	ImageSize     uint32
	XPPM, YPPM    int32
	Colors        uint32
	Important     uint32
}

func dibColor(c uint32) uint32 {
	return (c&255)<<16 | c&0xff00 | (c>>16)&255
}

func (b raster) draw(dc uintptr, x, y int32) {
	if len(b.pixels) == 0 {
		return
	}
	// Top-down, opaque 32-bit DIB. No selected bitmap or persistent GDI handle.
	info := bitmapHeader{Size: 40, Width: int32(b.width), Height: -int32(b.height), Planes: 1, Bits: 32}
	gdi32.NewProc("StretchDIBits").Call(dc, uintptr(x), uintptr(y), uintptr(b.width), uintptr(b.height), 0, 0, uintptr(b.width), uintptr(b.height), uintptr(unsafe.Pointer(&b.pixels[0])), uintptr(unsafe.Pointer(&info)), 0, 0x00cc0020)
	runtime.KeepAlive(b.pixels)
}

func channels(c uint32) (float64, float64, float64) {
	return float64(c & 255), float64((c >> 8) & 255), float64((c >> 16) & 255)
}

func pixel(r, g, b float64) uint32 {
	return uint32(math.Round(r))<<16 | uint32(math.Round(g))<<8 | uint32(math.Round(b))
}

// sampleLogo composites premultiplied PNG channels onto the theme background.
// Bilinear interpolation avoids nearest-neighbor enlargement artifacts.
func sampleLogo(src image.Image, x, y float64, bg uint32) (float64, float64, float64) {
	bounds := src.Bounds()
	x = math.Max(0, math.Min(float64(bounds.Dx()-1), x))
	y = math.Max(0, math.Min(float64(bounds.Dy()-1), y))
	x0, y0 := int(x), int(y)
	x1, y1 := min(x0+1, bounds.Dx()-1), min(y0+1, bounds.Dy()-1)
	fx, fy := x-float64(x0), y-float64(y0)
	br, bgc, bb := channels(bg)
	var r, g, b float64
	for _, p := range []struct {
		x, y int
		w    float64
	}{{x0, y0, (1 - fx) * (1 - fy)}, {x1, y0, fx * (1 - fy)}, {x0, y1, (1 - fx) * fy}, {x1, y1, fx * fy}} {
		cr, cg, cb, ca := src.At(bounds.Min.X+p.x, bounds.Min.Y+p.y).RGBA()
		alpha := 1 - float64(ca)/65535
		r += p.w * (float64(cr)/257 + br*alpha)
		g += p.w * (float64(cg)/257 + bgc*alpha)
		b += p.w * (float64(cb)/257 + bb*alpha)
	}
	return r, g, b
}

func logoRaster(data []byte, width int, bg uint32) raster {
	src, err := png.Decode(bytes.NewReader(data))
	if err != nil || width < 1 || width > 2048 || src.Bounds().Empty() {
		return raster{}
	}
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	height := max(1, int(math.Round(float64(width)*float64(sh)/float64(sw))))
	if height > 512 {
		return raster{}
	}
	out := raster{width: width, height: height, pixels: make([]uint32, width*height)}
	// Four-by-four integration when shrinking; bilinear when enlarging.
	samples := 1
	if width < sw {
		samples = 4
	}
	n := float64(samples)
	for y := range height {
		for x := range width {
			var r, g, b float64
			for sy := range samples {
				for sx := range samples {
					xx := (float64(x)+(float64(sx)+0.5)/n)*float64(sw)/float64(width) - 0.5
					yy := (float64(y)+(float64(sy)+0.5)/n)*float64(sh)/float64(height) - 0.5
					cr, cg, cb := sampleLogo(src, xx, yy, bg)
					r, g, b = r+cr, g+cg, b+cb
				}
			}
			out.pixels[y*width+x] = pixel(r/(n*n), g/(n*n), b/(n*n))
		}
	}
	return out
}

// Supersample both the capsule and circular thumb together so the two edges
// blend against the actual row background, rather than a fixed white matte.
func switchRaster(width, height int, background, track uint32, on bool) raster {
	if width < height || height < 4 || width > 256 || height > 128 {
		return raster{}
	}
	out := raster{width: width, height: height, pixels: make([]uint32, width*height)}
	radius := float64(height) / 2
	thumb := radius * 0.75
	center := radius
	if on {
		center = float64(width) - radius
	}
	br, bg, bb := channels(background)
	tr, tg, tb := channels(track)
	const samples = 4
	for y := range height {
		for x := range width {
			var r, g, b float64
			for sy := range samples {
				for sx := range samples {
					xx := float64(x) + (float64(sx)+0.5)/samples
					yy := float64(y) + (float64(sy)+0.5)/samples
					cx := math.Max(radius, math.Min(float64(width)-radius, xx))
					dx, dy := xx-cx, yy-radius
					tx := xx - center
					switch {
					case tx*tx+dy*dy <= thumb*thumb:
						r, g, b = r+255, g+255, b+255
					case dx*dx+dy*dy <= radius*radius:
						r, g, b = r+tr, g+tg, b+tb
					default:
						r, g, b = r+br, g+bg, b+bb
					}
				}
			}
			out.pixels[y*width+x] = pixel(r/16, g/16, b/16)
		}
	}
	return out
}
