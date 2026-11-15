// Command iconconvert turns gui/assets/icon-example.png into the menu-bar
// template icons and the application icon used by the Wails GUI.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

const (
	examplePath  = "gui/assets/icon-example.png"
	unlockedPath = "gui/assets/tray-unlocked.png"
	lockedPath   = "gui/assets/tray-locked.png"
	appAssets    = "gui/assets/appicon.png"
	appBuild     = "gui/build/appicon.png"
	previewDir   = "/tmp/opencode"
)

type mask struct {
	w, h int
	a    []uint8
}

func main() {
	src, err := loadMask(examplePath)
	if err != nil {
		fatal(err)
	}

	writeTray(src, unlockedPath, filepath.Join(previewDir, "tray-unlocked-preview.png"))

	locked := cloneMask(src)
	// The example's central star is at (210, 280), radius ~55.
	clearCircle(locked, 210, 280, 55)
	drawLock(locked, 210, 280)
	writeTray(locked, lockedPath, filepath.Join(previewDir, "tray-locked-preview.png"))

	app := makeAppIcon(src)
	writePNG(appAssets, app)
	writePNG(appBuild, app)

	fmt.Printf("wrote %s, %s, %s, %s\n", unlockedPath, lockedPath, appAssets, appBuild)
}

func loadMask(path string) (*mask, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	m := &mask{w: b.Dx(), h: b.Dy(), a: make([]uint8, b.Dx()*b.Dy())}
	for y := 0; y < m.h; y++ {
		for x := 0; x < m.w; x++ {
			r, g, b, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			if a == 0 {
				continue
			}
			lum := (uint32(r>>8) + uint32(g>>8) + uint32(b>>8)) / 3
			v := (255 - lum) * uint32(a>>8) / 255
			if v >= 8 {
				m.a[y*m.w+x] = uint8(v)
			}
		}
	}
	return m, nil
}

func cloneMask(src *mask) *mask {
	dst := &mask{w: src.w, h: src.h, a: make([]uint8, len(src.a))}
	copy(dst.a, src.a)
	return dst
}

func (m *mask) set(x, y int) {
	if x >= 0 && x < m.w && y >= 0 && y < m.h {
		m.a[y*m.w+x] = 255
	}
}

func clearCircle(m *mask, cx, cy, r int) {
	r2 := r * r
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			if x < 0 || x >= m.w || y < 0 || y >= m.h {
				continue
			}
			dx, dy := x-cx, y-cy
			if dx*dx+dy*dy <= r2 {
				m.a[y*m.w+x] = 0
			}
		}
	}
}

// drawLock draws a small filled padlock where the example's star was.
func drawLock(m *mask, cx, cy int) {
	const (
		bodyW = 72
		bodyH = 50
		bodyR = 8
	)
	left := float64(cx - bodyW/2)
	top := float64(cy - 8)
	right := float64(cx + bodyW/2)
	bottom := float64(cy + bodyH - 8)
	for y := int(top); y <= int(bottom); y++ {
		for x := int(left); x <= int(right); x++ {
			if insideRounded(float64(x), float64(y), left, top, right, bottom, bodyR) {
				m.set(x, y)
			}
		}
	}

	// Semicircular shackle above the body.
	scx, scy := float64(cx), float64(cy-8)
	for y := int(scy) - 28; y <= int(scy); y++ {
		for x := int(scx) - 28; x <= int(scx)+28; x++ {
			d := math.Hypot(float64(x)-scx, float64(y)-scy)
			if d >= 19 && d <= 28 {
				m.set(x, y)
			}
		}
	}
}

func writeTray(src *mask, path, previewPath string) {
	writePNG(path, resizeBox(src, 44, 44))
	writePNG(previewPath, resizeBox(src, 256, 256))
}

func resizeBox(src *mask, tw, th int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, tw, th))
	for ty := 0; ty < th; ty++ {
		sy0 := float64(ty) * float64(src.h) / float64(th)
		sy1 := float64(ty+1) * float64(src.h) / float64(th)
		for tx := 0; tx < tw; tx++ {
			sx0 := float64(tx) * float64(src.w) / float64(tw)
			sx1 := float64(tx+1) * float64(src.w) / float64(tw)
			var sum, area float64
			for sy := int(math.Floor(sy0)); sy < int(math.Ceil(sy1)); sy++ {
				oy := math.Min(sy1, float64(sy+1)) - math.Max(sy0, float64(sy))
				if oy <= 0 || sy < 0 || sy >= src.h {
					continue
				}
				for sx := int(math.Floor(sx0)); sx < int(math.Ceil(sx1)); sx++ {
					ox := math.Min(sx1, float64(sx+1)) - math.Max(sx0, float64(sx))
					if ox <= 0 || sx < 0 || sx >= src.w {
						continue
					}
					sum += float64(src.a[sy*src.w+sx]) * ox * oy
					area += ox * oy
				}
			}
			a := uint8(0)
			if area > 0 {
				a = uint8(math.Round(sum / area))
			}
			dst.SetRGBA(tx, ty, color.RGBA{0, 0, 0, a})
		}
	}
	return dst
}

func makeAppIcon(src *mask) *image.RGBA {
	const size = 1024
	out := image.NewRGBA(image.Rect(0, 0, size, size))
	const (
		left   = 64.0
		top    = 64.0
		right  = 960.0
		bottom = 960.0
		radius = 200.0
		border = 4.0
	)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			fx, fy := float64(x)+0.5, float64(y)+0.5
			if !insideRounded(fx, fy, left, top, right, bottom, radius) {
				continue
			}
			c := color.RGBA{255, 255, 255, 255}
			if !insideRounded(fx, fy, left+border, top+border, right-border, bottom-border, radius-border) {
				c = color.RGBA{0xE3, 0xE8, 0xE8, 255}
			}
			out.SetRGBA(x, y, c)
		}
	}

	const logoSize = 760.0
	lx := (size - logoSize) / 2
	ly := (size - logoSize) / 2
	for y := 0; y < int(logoSize); y++ {
		for x := 0; x < int(logoSize); x++ {
			a := sampleBilinear(src,
				(float64(x)+0.5)*float64(src.w)/logoSize,
				(float64(y)+0.5)*float64(src.h)/logoSize,
			)
			if a == 0 {
				continue
			}
			px, py := int(lx)+x, int(ly)+y
			if !insideRounded(float64(px)+0.5, float64(py)+0.5, left, top, right, bottom, radius) {
				continue
			}
			v := uint8(math.Round(255 * (1 - float64(a)/255)))
			out.SetRGBA(px, py, color.RGBA{v, v, v, 255})
		}
	}
	return out
}

func sampleBilinear(src *mask, x, y float64) uint8 {
	x -= 0.5
	y -= 0.5
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	fx, fy := x-float64(x0), y-float64(y0)
	at := func(xx, yy int) float64 {
		if xx < 0 || xx >= src.w || yy < 0 || yy >= src.h {
			return 0
		}
		return float64(src.a[yy*src.w+xx])
	}
	v := at(x0, y0)*(1-fx)*(1-fy) +
		at(x0+1, y0)*fx*(1-fy) +
		at(x0, y0+1)*(1-fx)*fy +
		at(x0+1, y0+1)*fx*fy
	return uint8(math.Round(v))
}

func insideRounded(x, y, left, top, right, bottom, r float64) bool {
	if x < left || x > right || y < top || y > bottom {
		return false
	}
	cx := math.Max(left+r, math.Min(x, right-r))
	cy := math.Max(top+r, math.Min(y, bottom-r))
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= r*r
}

func writePNG(path string, img image.Image) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "iconconvert:", err)
	os.Exit(1)
}
