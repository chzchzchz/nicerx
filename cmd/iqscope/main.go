package main

import (
	"fmt"
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
	centerHz  uint32
	winWidth  int
	winHeight int
	resizable bool
)

var rootCmd = &cobra.Command{
	Use:   "iqscope",
	Short: "A tool to graphically display IQ data in the typical fashion.",
}

func init() {
	fftCmd := &cobra.Command{
		Use:   "fft [flags] input.iq8",
		Short: "Stream FFT waterfall",
		Run:   func(cmd *cobra.Command, args []string) { fftCmd(args[0]) },
	}
	fftCmd.Flags().Uint32VarP(&centerHz, "center-hz", "c", 0, "Center Frequency in Hz")
	fftCmd.Flags().Uint32VarP(&sampleHz, "sample-rate", "s", 2048000, "Sample rate in Hz")
	fftCmd.Flags().IntVarP(&winWidth, "window-width", "w", 600, "Total FFT buckets / window width")
	fftCmd.Flags().IntVarP(&winHeight, "window-height", "r", 480, "Total FFT rows to display")
	fftCmd.Flags().BoolVarP(&resizable, "resize", "R", true, "Window is resizable")
	rootCmd.AddCommand(fftCmd)
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
	log.Println("waiting for first fft")
	if row0 := <-fftc; row0 == nil {
		log.Println("failed reading first fft")
		return
	}

	winFlags := uint32(sdl.WINDOW_SHOWN)
	if resizable {
		winFlags |= sdl.WINDOW_RESIZABLE | sdl.WINDOW_OPENGL
	}
	winName := fmt.Sprintf("iqscope FFT %0.2gMsps", float64(sampleHz)/1.0e6)
	if centerHz != 0 {
		winName = fmt.Sprintf("iqscope FFT @ [%0.4g,%0.4g]MHz",
			float64(centerHz-sampleHz/2)/1.0e6,
			float64(centerHz+sampleHz/2)/1.0e6)
	}

	win, err := sdl.CreateWindow(
		winName,
		sdl.WINDOWPOS_UNDEFINED,
		sdl.WINDOWPOS_UNDEFINED,
		int32(winWidth),
		int32(winHeight),
		winFlags)
	if err != nil {
		panic(err)
	}
	defer win.Destroy()

	// Disable letterboxing.
	sdl.SetHint(sdl.HINT_RENDER_LOGICAL_SIZE_MODE, "1")

	r, err := sdl.CreateRenderer(win, -1, sdl.RENDERER_ACCELERATED|sdl.RENDERER_TARGETTEXTURE)
	if err != nil {
		panic(err)
	}
	info, err := r.GetInfo()
	if err != nil {
		panic(err)
	}
	if (info.Flags & sdl.RENDERER_ACCELERATED) == 0 {
		log.Println("no hw acceleration")
	}
	if err := r.SetLogicalSize(int32(winWidth), int32(winHeight)); err != nil {
		panic(err)
	}
	if err := r.SetIntegerScale(false); err != nil {
		panic(err)
	}

	// Cope with too much data by dropping frames.
	// This will work even if the sample rate is totally wrong.
	fps := float64(30)
	framec := make(chan []float64, int(4*fps))
	go func() {
		defer close(framec)
		for row := range fftc {
			select {
			case framec <- row:
			default:
			}
		}
	}()

	fpsDur := time.Duration(float64(time.Second) / fps)
	ticker := time.NewTicker(fpsDur)
	defer ticker.Stop()

	ft := newFFTTexture(r, winWidth, winHeight)
	for {
		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			switch ev := event.(type) {
			case *sdl.QuitEvent:
				log.Println("Quit")
				return
			case *sdl.MouseMotionEvent:
				ww := float64(winWidth)
				centerOffset := (float64(ev.X) - (ww / 2.0)) / ww
				offHz := int64(centerOffset * float64(sampleHz))
				if centerHz != 0 {
					cursorMHz := (float64(centerHz) + float64(offHz)) / 1.0e6
					log.Printf("center: %0.7gMHz of [%g,%g]",
						cursorMHz,
						float64(centerHz-sampleHz/2)/1.0e6,
						float64(centerHz+sampleHz/2)/1.0e6)
				} else {
					log.Printf("offset: %vKHz", offHz/1000)
				}
			}
		}

		<-ticker.C

		// TODO: add more than one row per tick
		if row := <-framec; row != nil {
			ft.add(row)
		} else {
			log.Println("stream terminated")
			return
		}

		ft.blit()
		if err := r.Flush(); err != nil {
			panic(err)
		}
		r.Present()

		select {
		case <-ticker.C:
		default:
		}
		ticker.Reset(fpsDur)
	}
}

func main() {
	if err := sdl.Init(sdl.INIT_TIMER | sdl.INIT_VIDEO | sdl.INIT_EVENTS); err != nil {
		panic(err)
	}
	defer sdl.Quit()
	rootCmd.Execute()
}
