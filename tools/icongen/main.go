// Command icongen renders the application and tray icons.
//
// It is a development-only tool; the generated PNGs are committed so the
// application and its build do not depend on an image library at runtime.
package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
)

type point struct{ x, y float64 }

func main() {
	out := filepath.Join("gui", "assets")
	build := filepath.Join("gui", "build")
	if err := os.MkdirAll(out, 0755); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(build, 0755); err != nil {
		log.Fatal(err)
	}

	app := render(1024, 1024, 2, drawAppIcon)
	writePNG(filepath.Join(out, "appicon.png"), app)
	writePNG(filepath.Join(build, "appicon.png"), app)
	writePNG(filepath.Join(out, "tray-unlocked.png"), render(44, 44, 4, drawTrayUnlocked))
	writePNG(filepath.Join(out, "tray-locked.png"), render(44, 44, 4, drawTrayLocked))
}

func writePNG(path string, img image.Image) {
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		log.Fatal(err)
	}
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}
}

func render(size, design, scale int, draw func(*image.RGBA, float64)) *image.RGBA {
	canvas := image.NewRGBA(image.Rect(0, 0, size*scale, size*scale))
	draw(canvas, float64(size*scale)/float64(design))
	return downsample(canvas, size)
}

func downsample(src *image.RGBA, size int) *image.RGBA {
	scale := src.Bounds().Dx() / size
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var r, g, b, a uint32
			for dy := 0; dy < scale; dy++ {
				for dx := 0; dx < scale; dx++ {
					c := src.RGBAAt(x*scale+dx, y*scale+dy)
					r += uint32(c.R)
					g += uint32(c.G)
					b += uint32(c.B)
					a += uint32(c.A)
				}
			}
			n := uint32(scale * scale)
			dst.SetRGBA(x, y, color.RGBA{uint8(r / n), uint8(g / n), uint8(b / n), uint8(a / n)})
		}
	}
	return dst
}

func drawAppIcon(img *image.RGBA, s float64) {
	bg := color.RGBA{15, 27, 36, 255}
	mint := color.RGBA{159, 231, 216, 255}
	amber := color.RGBA{242, 184, 75, 235}

	fillRoundRect(img, 48*s, 48*s, 928*s, 928*s, 210*s, bg)
	for i := 0; i < 30; i++ {
		a0 := -math.Pi/2 + float64(i)*2*math.Pi/30
		a1 := a0 + (2*math.Pi/30)*0.72
		fillSector(img, 512*s, 512*s, 380*s, 430*s, a0, a1, amber)
	}
	shield := []point{
		{512, 180}, {790, 290}, {790, 560}, {512, 860}, {234, 560}, {234, 290},
	}
	fillPolygon(img, scalePoints(shield, s), mint)

	fillCircle(img, 512*s, 430*s, 90*s, bg)
	fillPolygon(img, scalePoints([]point{
		{470, 500}, {554, 500}, {590, 680}, {512, 730}, {434, 680},
	}, s), bg)
}

func cubicPoints(p0, p1, p2, p3 point, steps int) []point {
	pts := make([]point, 0, steps)
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		mt := 1 - t
		x := mt*mt*mt*p0.x + 3*mt*mt*t*p1.x + 3*mt*t*t*p2.x + t*t*t*p3.x
		y := mt*mt*mt*p0.y + 3*mt*mt*t*p1.y + 3*mt*t*t*p2.y + t*t*t*p3.y
		pts = append(pts, point{x, y})
	}
	return pts
}

func trayShieldOutline(img *image.RGBA, s float64, c color.RGBA) {
	outer := []point{{10, 8}, {34, 8}}
	outer = append(outer, cubicPoints(point{34, 8}, point{38, 12}, point{36, 28}, point{22, 40}, 12)...)
	outer = append(outer, cubicPoints(point{22, 40}, point{8, 28}, point{6, 12}, point{10, 8}, 12)...)
	fillPolygon(img, scalePoints(outer, s), c)

	inner := []point{{13.5, 11.5}, {30.5, 11.5}}
	inner = append(inner, cubicPoints(point{30.5, 11.5}, point{33.5, 14.5}, point{32.5, 26}, point{22, 36}, 12)...)
	inner = append(inner, cubicPoints(point{22, 36}, point{11, 26}, point{10.5, 14.5}, point{13.5, 11.5}, 12)...)
	clearPolygon(img, scalePoints(inner, s))
}

func drawTrayUnlocked(img *image.RGBA, s float64) {
	black := color.RGBA{0, 0, 0, 255}
	trayShieldOutline(img, s, black)
	fillCircle(img, 22*s, 17*s, 4.2*s, black)
	fillPolygon(img, scalePoints([]point{
		{19, 21}, {25, 21}, {26.6, 33}, {22, 35.5}, {17.4, 33},
	}, s), black)
}

func drawTrayLocked(img *image.RGBA, s float64) {
	black := color.RGBA{0, 0, 0, 255}
	trayShieldOutline(img, s, black)
	fillSector(img, 22*s, 20*s, 5.2*s, 8.2*s, math.Pi, 2*math.Pi, black)
	fillRoundRect(img, 14*s, 20*s, 16*s, 14*s, 3*s, black)
}

func fillPolygon(img *image.RGBA, pts []point, c color.RGBA) {
	if len(pts) < 3 {
		return
	}
	minY, maxY := pts[0].y, pts[0].y
	for _, p := range pts {
		minY = math.Min(minY, p.y)
		maxY = math.Max(maxY, p.y)
	}
	for y := int(math.Floor(minY)); y <= int(math.Ceil(maxY)); y++ {
		var xs []float64
		for i := range pts {
			j := (i + 1) % len(pts)
			a, b := pts[i], pts[j]
			if (a.y <= float64(y) && b.y > float64(y)) || (b.y <= float64(y) && a.y > float64(y)) {
				xs = append(xs, a.x+(float64(y)-a.y)*(b.x-a.x)/(b.y-a.y))
			}
		}
		sort.Float64s(xs)
		for k := 0; k+1 < len(xs); k += 2 {
			for x := int(math.Ceil(xs[k])); x <= int(math.Floor(xs[k+1])); x++ {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func fillCircle(img *image.RGBA, cx, cy, r float64, c color.RGBA) {
	for y := int(cy - r); y <= int(cy+r); y++ {
		for x := int(cx - r); x <= int(cx+r); x++ {
			dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
			if dx*dx+dy*dy <= r*r {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func clearCircle(img *image.RGBA, cx, cy, r float64) {
	fillCircle(img, cx, cy, r, color.RGBA{})
}

func fillRoundRect(img *image.RGBA, x, y, w, h, r float64, c color.RGBA) {
	fillCircle(img, x+r, y+r, r, c)
	fillCircle(img, x+w-r, y+r, r, c)
	fillCircle(img, x+r, y+h-r, r, c)
	fillCircle(img, x+w-r, y+h-r, r, c)
	fillPolygon(img, []point{{x + r, y}, {x + w - r, y}, {x + w - r, y + h}, {x + r, y + h}}, c)
	fillPolygon(img, []point{{x, y + r}, {x + w, y + r}, {x + w, y + h - r}, {x, y + h - r}}, c)
}

func clearRoundRect(img *image.RGBA, x, y, w, h, r float64) {
	fillRoundRect(img, x, y, w, h, r, color.RGBA{})
}

func fillSector(img *image.RGBA, cx, cy, rIn, rOut, start, end float64, c color.RGBA) {
	for y := int(cy - rOut); y <= int(cy+rOut); y++ {
		for x := int(cx - rOut); x <= int(cx+rOut); x++ {
			dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
			d := math.Hypot(dx, dy)
			if d < rIn || d > rOut {
				continue
			}
			a := math.Atan2(dy, dx)
			for a < start {
				a += 2 * math.Pi
			}
			if a <= end {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func clearSector(img *image.RGBA, cx, cy, rIn, rOut, start, end float64) {
	fillSector(img, cx, cy, rIn, rOut, start, end, color.RGBA{})
}

func clearPolygon(img *image.RGBA, pts []point) {
	if len(pts) < 3 {
		return
	}
	minY, maxY := pts[0].y, pts[0].y
	for _, p := range pts {
		minY = math.Min(minY, p.y)
		maxY = math.Max(maxY, p.y)
	}
	for y := int(math.Floor(minY)); y <= int(math.Ceil(maxY)); y++ {
		var xs []float64
		for i := range pts {
			j := (i + 1) % len(pts)
			a, b := pts[i], pts[j]
			if (a.y <= float64(y) && b.y > float64(y)) || (b.y <= float64(y) && a.y > float64(y)) {
				xs = append(xs, a.x+(float64(y)-a.y)*(b.x-a.x)/(b.y-a.y))
			}
		}
		sort.Float64s(xs)
		for k := 0; k+1 < len(xs); k += 2 {
			for x := int(math.Ceil(xs[k])); x <= int(math.Floor(xs[k+1])); x++ {
				img.SetRGBA(x, y, color.RGBA{})
			}
		}
	}
}

func scalePoints(pts []point, s float64) []point {
	out := make([]point, len(pts))
	for i, p := range pts {
		out[i] = point{p.x * s, p.y * s}
	}
	return out
}
