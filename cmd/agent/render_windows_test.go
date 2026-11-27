package main

import (
	"bytes"
	"os"
	"testing"
)

func TestDashboardLogos(t *testing.T) {
	for _, entry := range []struct {
		name string
		data []byte
	}{{"logo.png", lightLogo}, {"logo-dark.png", darkLogo}} {
		canonical, err := os.ReadFile("../../internal/core/static/" + entry.name)
		if err != nil || !bytes.Equal(canonical, entry.data) {
			t.Fatal("embedded logo differs from dashboard asset")
		}
		for _, width := range []int{380, 475, 570, 760} {
			out := logoRaster(entry.data, width, rgb(16, 21, 30))
			if out.width != width || out.height != (width*78+240)/480 || len(out.pixels) != out.width*out.height {
				t.Fatal("logo aspect ratio or raster size changed")
			}
			first := out.pixels[0]
			different := false
			for _, p := range out.pixels {
				if p != first {
					different = true
					break
				}
			}
			if !different {
				t.Fatal("blank logo")
			}
		}
	}
}

func TestAntialiasedSwitch(t *testing.T) {
	bg, track := rgb(27, 36, 50), rgb(96, 165, 250)
	for _, size := range [][2]int{{44, 24}, {55, 30}, {66, 36}, {88, 48}} {
		w, h := size[0], size[1]
		off, on := switchRaster(w, h, bg, track, false), switchRaster(w, h, bg, track, true)
		if len(off.pixels) != w*h || len(on.pixels) != w*h {
			t.Fatal("invalid switch size")
		}
		blended := false
		for y := range h {
			for x := range w {
				p := off.pixels[y*w+x]
				if p != on.pixels[y*w+w-1-x] {
					t.Fatal("switch thumb and track are not symmetric")
				}
				if p != dibColor(bg) && p != dibColor(track) && p != 0xffffff {
					blended = true
				}
			}
		}
		if !blended || off.pixels[0] != dibColor(bg) || off.pixels[h/2*w+h/2] != 0xffffff {
			t.Fatal("switch lacks blended edges, background or white thumb")
		}
	}
}
