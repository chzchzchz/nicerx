package main

import (
	"github.com/veandco/go-sdl2/sdl"

	"github.com/chzchzchz/nicerx/nicerx"
)

type fftSurface struct {
	rows   []*sdl.Surface
	win    *sdl.Surface
	rowIdx int // wraps around
}

func newFFTSurface(winSurface *sdl.Surface, rows int) *fftSurface {
	fs := &fftSurface{rows: make([]*sdl.Surface, rows), win: winSurface}
	wfmt := fs.win.Format
	// Create surfaces for each row.
	for i := range fs.rows {
		var err error
		fs.rows[i], err = sdl.CreateRGBSurface(
			0, fs.win.W, 1, int32(wfmt.BitsPerPixel), wfmt.Rmask, wfmt.Gmask, wfmt.Bmask, wfmt.Amask)
		if err != nil {
			panic(err)
		}
		fs.rows[i].FillRect(nil, 0)
	}
	return fs
}

func (fs *fftSurface) blit() {
	srcRect := &sdl.Rect{X: 0, Y: 0, W: fs.win.W, H: 1}
	dstRect := &sdl.Rect{X: 0 /* Y set in loops */, W: fs.win.W, H: 1}
	for i := fs.rowIdx; i < len(fs.rows); i++ {
		if err := fs.rows[i].Blit(srcRect, fs.win, dstRect); err != nil {
			panic(err)
		}
		dstRect.Y++
	}
	for i := 0; i < fs.rowIdx; i++ {
		if err := fs.rows[i].Blit(srcRect, fs.win, dstRect); err != nil {
			panic(err)
		}
		dstRect.Y++
	}
}

func (fs *fftSurface) add(row []float64) {
	for i, v := range row {
		fs.rows[fs.rowIdx].Set(i, 0, nicerx.FFTBin2Color(v*v))
	}
	fs.rowIdx++
	if fs.rowIdx >= len(fs.rows) {
		fs.rowIdx = 0
	}
}

type fftTexture struct {
	r       *sdl.Renderer
	rows    []*sdl.Texture
	rowIdx  int // wraps around
	w       int
	row8888 []byte
	rowRect *sdl.Rect
}

func newFFTTexture(r *sdl.Renderer, w, h int) *fftTexture {
	ft := &fftTexture{
		r:       r,
		rows:    make([]*sdl.Texture, h),
		w:       w,
		row8888: make([]byte, w*4),
		rowRect: &sdl.Rect{X: 0, Y: 0, W: int32(w), H: 1},
	}
	for i := 0; i < w; i++ {
		ft.row8888[4*i+3] = 0xff
	}
	// Create textures for each row.
	for i := range ft.rows {
		var err error
		ft.rows[i], err = r.CreateTexture(
			sdl.PIXELFORMAT_RGB888, sdl.TEXTUREACCESS_STREAMING, int32(w), 1)
		if err != nil {
			panic(err)
		}
		if err = ft.rows[i].Update(ft.rowRect, ft.row8888, 4); err != nil {
			panic(err)
		}
	}
	return ft
}

func (ft *fftTexture) blit() {
	dstRect := &sdl.Rect{X: 0 /* Y set in loops */, W: int32(ft.w), H: 1}
	for i := ft.rowIdx; i < len(ft.rows); i++ {
		if err := ft.r.Copy(ft.rows[i], ft.rowRect, dstRect); err != nil {
			panic(err)
		}
		dstRect.Y++
	}
	for i := 0; i < ft.rowIdx; i++ {
		if err := ft.r.Copy(ft.rows[i], ft.rowRect, dstRect); err != nil {
			panic(err)
		}
		dstRect.Y++
	}
}

func (ft *fftTexture) add(row []float64) {
	for i, v := range row {
		c := nicerx.FFTBin2Color(v * v)
		ft.row8888[4*i] = byte(c.R)
		ft.row8888[4*i+1] = byte(c.G)
		ft.row8888[4*i+2] = byte(c.B)
	}
	ft.rows[ft.rowIdx].Update(ft.rowRect, ft.row8888, 4)

	ft.rowIdx++
	if ft.rowIdx >= len(ft.rows) {
		ft.rowIdx = 0
	}
}
