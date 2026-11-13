// Icon draws resources/icon.png, the app icon: a white speech bubble with
// three dots on a squircle of the webapp's Denim blue. It is the
// project's own, so that the app is not taken for the official client.
//
//	go run ./scripts/icon
package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
)

const (
	size = 1024
	ss   = 4 // samples per pixel side, for smooth edges
)

// inSquircle reports whether a point is inside the macOS icon shape: a
// superellipse of 824 points in the 1024 grid, as Apple's template.
func inSquircle(x, y float64) bool {
	const half, n = 412.0, 5.0
	dx, dy := math.Abs(x-512)/half, math.Abs(y-512)/half
	return math.Pow(dx, n)+math.Pow(dy, n) <= 1
}

func inRoundRect(x, y, x0, y0, x1, y1, r float64) bool {
	if x < x0 || x > x1 || y < y0 || y > y1 {
		return false
	}
	cx := math.Max(x0+r, math.Min(x, x1-r))
	cy := math.Max(y0+r, math.Min(y, y1-r))
	return (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r*r
}

// inTail is the bubble's tail, a triangle at its lower left.
func inTail(x, y float64) bool {
	ax, ay, bx, by, cx, cy := 330.0, 640.0, 450.0, 640.0, 300.0, 760.0
	d := (by-cy)*(ax-cx) + (cx-bx)*(ay-cy)
	l1 := ((by-cy)*(x-cx) + (cx-bx)*(y-cy)) / d
	l2 := ((cy-ay)*(x-cx) + (ax-cx)*(y-cy)) / d
	return l1 >= 0 && l2 >= 0 && 1-l1-l2 >= 0
}

func inDot(x, y float64) bool {
	for _, cx := range []float64{392, 512, 632} {
		if (x-cx)*(x-cx)+(y-470)*(y-470) <= 44*44 {
			return true
		}
	}
	return false
}

func main() {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	top := [3]float64{0x3d, 0x6b, 0xd6}
	bottom := [3]float64{0x1e, 0x32, 0x5c}
	white := [3]float64{255, 255, 255}
	navy := [3]float64{0x1e, 0x32, 0x5c}
	for py := range size {
		for px := range size {
			var r, g, b, a float64
			for sy := range ss {
				for sx := range ss {
					x := float64(px) + (float64(sx)+0.5)/ss
					y := float64(py) + (float64(sy)+0.5)/ss
					if !inSquircle(x, y) {
						continue
					}
					t := (y - 100) / 824
					c := [3]float64{top[0] + (bottom[0]-top[0])*t, top[1] + (bottom[1]-top[1])*t, top[2] + (bottom[2]-top[2])*t}
					if inRoundRect(x, y, 250, 300, 774, 650, 120) || inTail(x, y) {
						c = white
						if inDot(x, y) {
							c = navy
						}
					}
					r, g, b, a = r+c[0], g+c[1], b+c[2], a+1
				}
			}
			if a == 0 {
				continue
			}
			img.SetNRGBA(px, py, color.NRGBA{uint8(r / a), uint8(g / a), uint8(b / a), uint8(255 * a / (ss * ss))})
		}
	}
	if err := os.MkdirAll("resources", 0o755); err != nil {
		log.Fatal(err)
	}
	f, err := os.Create("resources/icon.png")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		log.Fatal(err)
	}
}
