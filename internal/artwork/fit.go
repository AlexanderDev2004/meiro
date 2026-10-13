package artwork

import (
	"image"
	"image/draw"
	"math"

	xdraw "golang.org/x/image/draw"
)

// fit crops img to the square its Cover fit would show, shrinks it to side
// pixels across when it is larger, and rounds its corners by corner, a
// fraction of the side. The result is the picture exactly as it is drawn, so
// the cache keeps no pixels that are never seen and the drawing needs no
// clip mask. A picture smaller than side is not enlarged.
func fit(img image.Image, side int, corner float64) *image.RGBA {
	b := img.Bounds()
	crop := min(b.Dx(), b.Dy())
	x, y := b.Min.X+(b.Dx()-crop)/2, b.Min.Y+(b.Dy()-crop)/2
	src := image.Rect(x, y, x+crop, y+crop)
	out := crop
	if side > 0 && side < crop {
		out = side
	}
	dst := image.NewRGBA(image.Rect(0, 0, out, out))
	if out == crop {
		draw.Draw(dst, dst.Bounds(), img, src.Min, draw.Src)
	} else {
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, src, xdraw.Src, nil)
	}
	round(dst, corner*float64(out))
	return dst
}

// round makes the corners of a square picture transparent outside a quarter
// circle of the given radius in pixels, with a one-pixel soft edge. The
// pixels are premultiplied, so each is scaled as a whole.
func round(img *image.RGBA, radius float64) {
	n := img.Bounds().Dx()
	radius = min(radius, float64(n)/2)
	if radius < 0.5 {
		return
	}
	span := int(math.Ceil(radius))
	for y := range span {
		for x := range span {
			// Distance of the pixel's centre from the centre of the corner's
			// circle, which sits radius in from both edges.
			dx, dy := radius-float64(x)-0.5, radius-float64(y)-0.5
			cover := min(max(radius-math.Hypot(dx, dy)+0.5, 0), 1)
			if cover >= 1 {
				continue
			}
			for _, at := range [4][2]int{{x, y}, {n - 1 - x, y}, {x, n - 1 - y}, {n - 1 - x, n - 1 - y}} {
				i := img.PixOffset(at[0], at[1])
				for k := range 4 {
					img.Pix[i+k] = uint8(float64(img.Pix[i+k])*cover + 0.5)
				}
			}
		}
	}
}
