package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"encoding/binary"

	"github.com/spf13/cobra"

	"github.com/chzchzchz/nicerx/dsp"
	"github.com/chzchzchz/nicerx/http"
	"github.com/chzchzchz/nicerx/nicerx"
	"github.com/chzchzchz/nicerx/radio"
	"github.com/chzchzchz/nicerx/store"
)

var rootCmd = &cobra.Command{
	Use:   "nicerx",
	Short: "A SDR interface server.",
}
var (
	centerHz    uint64
	sampleHz    uint32
	deviationHz uint
	downmixHz   int
	cutoffHz    uint
	bandwidthHz uint
	powerFFTs   int
	imageWidth  int
	pcmHz uint
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "serve",
		Short: "Start the server",
		Run:   func(cmd *cobra.Command, args []string) { serve() },
	})

	downmixCmd := &cobra.Command{
		Use:   "downmix [flags] input.iq8 output.iq8",
		Short: "Frequency downmix",
		Run:   func(cmd *cobra.Command, args []string) { applyXfmCmd(downmixXfm, args[0], args[1]) },
	}
	downmixCmd.Flags().IntVarP(&downmixHz, "frequency-downmix", "S", 0, "Frequency to down mix in Hz")
	downmixCmd.Flags().Uint32VarP(&sampleHz, "sample-rate", "s", 2048000, "Sample rate in Hz")
	rootCmd.AddCommand(downmixCmd)

	lowpassCmd := &cobra.Command{
		Use:   "lpf [flags] input.iq8 output.iq8",
		Short: "Lowpass filter",
		Run:   func(cmd *cobra.Command, args []string) { applyXfmCmd(lowpassXfm, args[0], args[1]) },
	}
	lowpassCmd.Flags().UintVarP(&cutoffHz, "cutoff", "c", 0, "Cutoff frequency in Hz")
	lowpassCmd.Flags().Uint32VarP(&sampleHz, "sample-rate", "s", 2048000, "Sample rate in Hz")
	rootCmd.AddCommand(lowpassCmd)

	captureCmd := &cobra.Command{
		Use:   "capture [flags]",
		Short: "Capture a band",
		Run:   func(cmd *cobra.Command, args []string) { capture() },
	}
	captureCmd.Flags().Uint64VarP(&centerHz, "frequency", "f", 0, "Frequency to capture in Hz")
	captureCmd.Flags().UintVarP(&bandwidthHz, "bandwidth", "b", 0, "Bandwidth to capture in Hz")
	rootCmd.AddCommand(captureCmd)

	spectrogramCmd := &cobra.Command{
		Use:   "spectrogram [flags] input.iq8 output.jpg",
		Short: "Write spectrogram jpg",
		Run:   func(cmd *cobra.Command, args []string) { spectrogram(args[0], args[1]) },
	}
	spectrogramCmd.Flags().Uint32VarP(&sampleHz, "sample-rate", "s", 2048000, "Sample rate in Hz")
	spectrogramCmd.Flags().IntVarP(&imageWidth, "image-width", "w", 1024, "Sample rate in Hz")
	rootCmd.AddCommand(spectrogramCmd)

	importCmd := &cobra.Command{
		Use: "import csvfile",
		Short: "Import gqrx csv file into bands.db",
		Run: func(cmd *cobra.Command, args []string) { importCSV(args[0]) },
	}
	rootCmd.AddCommand(importCmd)

	demodCommand := &cobra.Command{
		Use: "fmdemod iqfile pcmfile",
		Short: "FM demodulate an iq8 file to PCM",
		Run: func(cmd *cobra.Command, args []string) { demod(args[0], args[1]) },
	}
	demodCommand.Flags().Uint32VarP(&sampleHz, "sample-rate", "s", 0, "Sample rate in Hz")
	demodCommand.Flags().UintVarP(&deviationHz, "deviation", "d", 0, "Maximum signal deviation in Hz")
	demodCommand.Flags().UintVarP(&pcmHz, "pcm-rate", "p", 0, "PCM sampling rate in Hz")
	rootCmd.AddCommand(demodCommand)

}

func demod(inf, outf string) {
	if sampleHz == 0 || deviationHz == 0 || pcmHz == 0 {
		panic("need sample-rate and deviation")
	}
	f, err := os.Open(inf)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	fout, err := os.OpenFile(outf, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}
	defer fout.Close()

	h := float64(deviationHz) / float64(sampleHz)
	iqr := radio.NewIQReader(f)
	demodc := dsp.DemodFM(float32(h), iqr.Batch64(512, 0))
	r := float64(pcmHz) /float64(sampleHz)
	resampc := dsp.Resample(float32(r), demodc)
	for rsamps := range resampc {
		outsamps := make([]int16, len(rsamps))
		for i, v := range rsamps {
			outsamps[i] = int16(v*65536)
		}
		if err := binary.Write(fout, binary.LittleEndian, outsamps); err != nil {
			panic(err)
		}
	}
}

func importCSV(inf string) {
	f, err := os.Open(inf)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	b := store.NewBandStore()
	b.Load("bands.db")
	if err := b.ImportCSV(f); err != nil {
		panic(err)
	}
	if err := b.Save("bands.db"); err != nil {
		panic(err)
	}
}

func spectrogram(inf, outf string) {
	if err := nicerx.WriteSpectrogramFile(inf, outf, imageWidth); err != nil {
		panic(err)
	}
}

func capture() {
	if centerHz == 0 {
		panic("need center frequency")
	}
	if bandwidthHz == 0 {
		panic("need bandwidth")
	}
	sdr, err := radio.NewSDR(context.TODO())
	if err != nil {
		panic(err)
	}
	defer sdr.Close()
	fb := radio.FreqBand{Center: float64(centerHz) / 1e6, Width: float64(bandwidthHz) / 1e6}
	ss, err := store.NewSignalStore("bands")
	if err != nil {
		panic(err)
	}
	c := nicerx.NewCapture(sdr, fb, ss)
	if err := c.Step(context.TODO()); err != nil && err != io.EOF {
		panic(err)
	}
}

func serve() {
	ctx, cancel := context.WithCancel(context.Background())
	sdr, err := radio.NewSDR(ctx)
	if err != nil {
		panic(err)
	}
	defer func() {
		cancel()
		sdr.Close()
	}()
	s, err := nicerx.NewServer(sdr)
	if err != nil {
		panic(err)
	}
	go func() { s.Serve(ctx) }()
	fmt.Println("serving http on :8080...")
	if err := http.ServeHttp(s, ":8080"); err != nil {
		fmt.Println(err)
	}
}

type xfmFunc func(iqr *radio.IQReader, iqw *radio.IQWriter)

func applyXfmCmd(xf xfmFunc, inf, outf string) {
	f, err := os.Open(inf)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	fout, err := os.OpenFile(outf, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}
	defer fout.Close()
	xf(radio.NewIQReader(f), radio.NewIQWriter(fout))
}

func downmixXfm(iqr *radio.IQReader, iqw *radio.IQWriter) {
	inc := iqr.Batch64(int(sampleHz), 5)
	for outSamps := range dsp.MixDown(float64(downmixHz), int(sampleHz), inc) {
		iqw.Write64(outSamps)
	}
}

func lowpassXfm(iqr *radio.IQReader, iqw *radio.IQWriter) {
	inc := iqr.Batch64(int(sampleHz), 5)
	lpfc := dsp.Lowpass(float64(cutoffHz), int(sampleHz), 1, inc)
	for outSamps := range lpfc {
		iqw.Write64(outSamps)
	}
}

func main() {
	rootCmd.Execute()
}
