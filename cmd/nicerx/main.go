package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/chzchzchz/nicerx/decoder"
	"github.com/chzchzchz/nicerx/nicerx"
	"github.com/chzchzchz/nicerx/nicerx/http"
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
	pcmHz       uint
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "serve",
		Short: "Start the server",
		Run:   func(cmd *cobra.Command, args []string) { serve() },
	})

	captureCmd := &cobra.Command{
		Use:   "capture [flags]",
		Short: "Capture a band",
		Run:   func(cmd *cobra.Command, args []string) { capture() },
	}
	captureCmd.Flags().Uint64VarP(&centerHz, "frequency", "f", 0, "Frequency to capture in Hz")
	captureCmd.Flags().UintVarP(&bandwidthHz, "bandwidth", "b", 0, "Bandwidth to capture in Hz")
	rootCmd.AddCommand(captureCmd)

	importCmd := &cobra.Command{
		Use:   "import csvfile",
		Short: "Import gqrx csv file into bands.db",
		Run:   func(cmd *cobra.Command, args []string) { importCSV(args[0]) },
	}
	rootCmd.AddCommand(importCmd)

	decodeCommand := &cobra.Command{
		Use:   "decode iqfile",
		Short: "Decode iq8 file with multimon-ng",
		Run:   func(cmd *cobra.Command, args []string) { decode(args[0]) },
	}
	decodeCommand.Flags().Uint32VarP(&sampleHz, "sample-rate", "s", 0, "Sample rate in Hz")
	rootCmd.AddCommand(decodeCommand)
}

func decode(inf string) {
	if sampleHz == 0 {
		panic("need sample-rate")
	}
	f, err := os.Open(inf)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	s, err := decoder.FlexDecode(float32(sampleHz), radio.NewIQReader(f).Batch64(512, 0))
	if err != nil {
		panic(err)
	}
	fmt.Println(s)
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

func main() {
	rootCmd.Execute()
}
