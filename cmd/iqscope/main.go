package main

import (
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/veandco/go-sdl2/sdl"

	"github.com/chzchzchz/nicerx/nicerx"
	"github.com/chzchzchz/nicerx/radio"
)

var (
	sampleHz  uint32
	winWidth  int
	winHeight int
)

var rootCmd = &cobra.Command{
	Use:   "iqscope",
	Short: "A tool to graphically display IQ data in the typical fashion.",
}

func init() {
	fftCmd := &cobra.Command{
		Use:   "fft [flags] input.iq8",
		Short: "Stream FFT",
		Run:   func(cmd *cobra.Command, args []string) { fftCmd(args[0]) },
	}
	fftCmd.Flags().Uint32VarP(&sampleHz, "sample-rate", "s", 2048000, "Sample rate in Hz")
	fftCmd.Flags().IntVarP(&winWidth, "window-width", "w", 600, "Total FFT buckets / window width")
	fftCmd.Flags().IntVarP(&winHeight, "window-height", "r", 480, "Total FFT rows to display")
	rootCmd.AddCommand(fftCmd)
}

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

func fftCmd(fname string) {
	// TODO: move out stdout stuff.
	var f *os.File
	if fname == "-" {
		f = os.Stdin
	} else {
		newf, err := os.Open(fname)
		if err != nil {
			panic(err)
		}
		defer newf.Close()
		f = newf
	}
	fftc := nicerx.SpectrogramChan(radio.NewIQReader(f), winWidth)

	window, err := sdl.CreateWindow("iqscope FFT", sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED,
		int32(winWidth), int32(winHeight), sdl.WINDOW_SHOWN)
	if err != nil {
		panic(err)
	}
	defer window.Destroy()

	surface, err := window.GetSurface()
	if err != nil {
		panic(err)
	}

	fs := newFFTSurface(surface, winHeight)
	for {
		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			switch event.(type) {
			case *sdl.QuitEvent:
				log.Println("Quit")
				return
			}
		}
		select {
		case row, ok := <-fftc:
			if ok {
				fs.add(row)
				fs.blit()
				window.UpdateSurface()
			} else {
				panic("no data")
			}
		default:
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func main() {
	if err := sdl.Init(sdl.INIT_TIMER | sdl.INIT_VIDEO | sdl.INIT_EVENTS); err != nil {
		panic(err)
	}
	defer sdl.Quit()
	rootCmd.Execute()
}
