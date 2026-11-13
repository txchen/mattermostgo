// Package media fetches images, keeps them in a disk cache, and decodes
// and downscales them off the main thread, keeping the decoded ones in an
// LRU bounded by bytes.
package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo/ui"
)

// Fetch gets the encoded bytes of an image.
type Fetch func(ctx context.Context) ([]byte, error)

type entry struct {
	bmp     *ui.Bitmap
	bytes   int
	loading bool
	failed  time.Time
	used    uint64
}

// Cache belongs to the main thread, except for the work it starts.
type Cache struct {
	// Run runs a function on the main thread.
	Run      func(func())
	Dir      string
	MaxBytes int

	items map[string]*entry
	total int
	clock uint64
}

func New(run func(func()), dir string, maxBytes int) *Cache {
	return &Cache{Run: run, Dir: dir, MaxBytes: maxBytes, items: map[string]*entry{}}
}

// Get returns the image under key scaled to fit maxPx, or nil while it
// loads, starting the load. A failed load waits a minute before retrying.
func (c *Cache) Get(key string, maxPx int, fetch Fetch) *ui.Bitmap {
	c.clock++
	e := c.items[key]
	if e == nil {
		e = &entry{}
		c.items[key] = e
	}
	e.used = c.clock
	if e.bmp != nil || e.loading || time.Since(e.failed) < time.Minute {
		return e.bmp
	}
	e.loading = true
	go func() {
		bmp, n, err := c.load(key, maxPx, fetch)
		c.Run(func() {
			e.loading = false
			if err != nil {
				e.failed = time.Now()
				return
			}
			e.bmp, e.bytes = bmp, n
			c.total += n
			c.evict()
		})
	}()
	return nil
}

func (c *Cache) evict() {
	for c.total > c.MaxBytes {
		var oldest string
		var at uint64
		for k, e := range c.items {
			if e.bmp != nil && (oldest == "" || e.used < at) {
				oldest, at = k, e.used
			}
		}
		if oldest == "" {
			return
		}
		c.total -= c.items[oldest].bytes
		delete(c.items, oldest)
	}
}

func (c *Cache) load(key string, maxPx int, fetch Fetch) (*ui.Bitmap, int, error) {
	sum := sha256.Sum256([]byte(key))
	path := filepath.Join(c.Dir, hex.EncodeToString(sum[:16]))
	data, err := os.ReadFile(path)
	if err != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		data, err = fetch(ctx)
		if err != nil {
			return nil, 0, err
		}
		if c.Dir != "" {
			_ = os.MkdirAll(c.Dir, 0o700)
			_ = os.WriteFile(path, data, 0o600)
		}
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		// Formats the standard library does not decode, as WebP.
		bmp, err := ui.DecodeBitmap(data)
		return bmp, len(data) * 4, err
	}
	rgba := Fit(img, maxPx)
	return ui.NewBitmap(rgba), len(rgba.Pix), nil
}

// Fit returns the image halved until neither side is over twice maxPx,
// averaging each 2×2 block, so that it shows sharp at maxPx.
func Fit(img image.Image, maxPx int) *image.RGBA {
	b := img.Bounds()
	src := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(src, src.Bounds(), img, b.Min, draw.Src)
	for maxPx > 0 && (src.Rect.Dx() > 2*maxPx || src.Rect.Dy() > 2*maxPx) && src.Rect.Dx() > 1 && src.Rect.Dy() > 1 {
		w, h := src.Rect.Dx()/2, src.Rect.Dy()/2
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := range h {
			r0 := src.Pix[2*y*src.Stride:]
			r1 := src.Pix[(2*y+1)*src.Stride:]
			d := dst.Pix[y*dst.Stride:]
			for x := range w {
				for k := range 4 {
					i := 8*x + k
					d[4*x+k] = uint8((int(r0[i]) + int(r0[i+4]) + int(r1[i]) + int(r1[i+4]) + 2) / 4)
				}
			}
		}
		src = dst
	}
	return src
}
