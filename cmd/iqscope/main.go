package main

import (
	"github.com/spf13/cobra"
	"github.com/veandco/go-sdl2/sdl"

	"github.com/chzchzchz/nicerx/nicerx"
	"github.com/chzchzchz/nicerx/radio"
)

var (
	flagBand  radio.HzBand
	winWidth  int
	winHeight int
	resizable bool
	popup     bool
)

var rootCmd = &cobra.Command{
	Use:   "iqscope",
	Short: "A tool to graphically display IQ data in the typical fashion.",
}

func init() {
	fftCmd := &cobra.Command{
		Use:   "fft [flags] [input.iq8|sdr://host/device]",
		Short: "Stream FFT waterfall",
		Run:   func(cmd *cobra.Command, args []string) { fftCmd(args) },
	}

	fftCmd.Flags().Uint64VarP(&flagBand.Center, "center-hz", "c", 0, "Center Frequency in Hz")
	fftCmd.Flags().Uint64VarP(&flagBand.Width, "sample-rate", "s", 2048000, "Sample rate in Hz")

	// UI
	fftCmd.Flags().IntVarP(&winWidth, "window-width", "w", 600, "Total FFT buckets / window width")
	fftCmd.Flags().IntVarP(&winHeight, "window-height", "r", 480, "Total FFT rows to display")
	fftCmd.Flags().BoolVarP(&resizable, "resize", "R", true, "Window is resizable")
	fftCmd.Flags().BoolVarP(&popup, "popup", "p", false, "Window is a pop-up (i3 hack)")

	rootCmd.AddCommand(fftCmd)
}

func fftCmd(args []string) {
	if len(args) == 0 {
		panic("expected arguments for fft file / device")
	}
	iqr, closer, err := nicerx.OpenIQR(args[0], flagBand)
	if err != nil {
		panic(err)
	}
	defer closer()

	fw, err := NewFFTWindow(iqr, winWidth, winHeight)
	if err != nil {
		panic(err)
	}
	defer fw.Close()
	fw.iqrPath = args[0]
	fw.Run()

	// Close any streams before waiting on processes to terminate.
	closer()
}

func main() {
	if err := sdl.Init(sdl.INIT_TIMER | sdl.INIT_VIDEO | sdl.INIT_EVENTS); err != nil {
		panic(err)
	}
	defer sdl.Quit()
	rootCmd.Execute()
}
