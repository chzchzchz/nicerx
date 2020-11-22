package main

import (
	"encoding/binary"

	"github.com/spf13/cobra"

	"github.com/chzchzchz/nicerx/dsp"
	"github.com/chzchzchz/nicerx/nicerx"
	"github.com/chzchzchz/nicerx/radio"
)

var (
	flagBand    radio.HzBand
	deviationHz uint
	downmixHz   int
	cutoffHz    uint
	bandwidthHz uint
	powerFFTs   int
	imageWidth  int
	pcmHz       uint
)

var rootCmd = &cobra.Command{
	Use:   "iqpipe",
	Short: "A tool to pipe around IQ modulation.",
}

func addFlagBand(cmd *cobra.Command) {
	cmd.Flags().Uint64VarP(&flagBand.Center, "center-hz", "c", 0, "Center frequency in Hz")
	cmd.Flags().Uint64VarP(&flagBand.Width, "sample-rate", "s", 2048000, "Sample rate in Hz")
}

func init() {
	downmixCmd := &cobra.Command{
		Use:   "downmix [flags] input.iq8 output.iq8",
		Short: "Frequency downmix",
		Run:   func(cmd *cobra.Command, args []string) { applyXfmCmd(downmixXfm, args[0], args[1]) },
	}
	downmixCmd.Flags().IntVarP(&downmixHz, "frequency-downmix", "S", 0, "Frequency to down mix in Hz")
	addFlagBand(downmixCmd)
	rootCmd.AddCommand(downmixCmd)

	lowpassCmd := &cobra.Command{
		Use:   "lpf [flags] input.iq8 output.iq8",
		Short: "Lowpass filter",
		Run:   func(cmd *cobra.Command, args []string) { applyXfmCmd(lowpassXfm, args[0], args[1]) },
	}
	lowpassCmd.Flags().UintVarP(&cutoffHz, "cutoff", "C", 0, "Cutoff frequency in Hz")
	addFlagBand(lowpassCmd)
	rootCmd.AddCommand(lowpassCmd)

	spectrogramCmd := &cobra.Command{
		Use:   "spectrogram [flags] input.iq8 output.jpg",
		Short: "Write spectrogram jpg",
		Run:   func(cmd *cobra.Command, args []string) { spectrogram(args[0], args[1]) },
	}
	spectrogramCmd.Flags().IntVarP(&imageWidth, "image-width", "w", 1024, "Width of FFT")
	addFlagBand(spectrogramCmd)
	rootCmd.AddCommand(spectrogramCmd)

	demodCmd := &cobra.Command{
		Use:   "fmdemod iqfile pcmfile",
		Short: "FM demodulate an iq8 file to PCM",
		Run:   func(cmd *cobra.Command, args []string) { demod(args[0], args[1]) },
	}
	demodCmd.Flags().UintVarP(&deviationHz, "deviation", "d", 0, "Maximum signal deviation in Hz")
	demodCmd.Flags().UintVarP(&pcmHz, "pcm-rate", "p", 0, "PCM sampling rate in Hz")
	addFlagBand(demodCmd)
	rootCmd.AddCommand(demodCmd)
}

func mustOpenIQW(outf string) (*radio.IQWriter, func()) {
	w, c, err := nicerx.OpenIQW(outf, flagBand)
	if err != nil {
		panic(err)
	}
	return w, c
}

func mustOpenInput(inf string) (*radio.MixerIQReader, func()) {
	r, c, err := nicerx.OpenIQR(inf, flagBand)
	if err != nil {
		panic(err)
	}
	return r, c
}

func demod(inf, outf string) {
	if flagBand.Width == 0 || deviationHz == 0 || pcmHz == 0 {
		panic("need sample-rate and deviation")
	}
	iqr, rcloser := mustOpenInput(inf)
	defer rcloser()

	outBand := radio.HzBand{Center: iqr.Center, Width: uint64(pcmHz)}
	writer, wcloser, err := nicerx.OpenOutputS16(outf, outBand)
	if err != nil {
		panic(err)
	}
	defer wcloser()

	h := float64(deviationHz) / float64(iqr.Width)
	demodc := dsp.DemodFM(float32(h), iqr.Batch64(512, 0))
	r := float64(pcmHz) / float64(iqr.Width)
	resampc := dsp.Resample(float32(r), demodc)
	min, max := float32(0), float32(0)
	for rsamps := range resampc {
		outsamps := make([]int16, len(rsamps))
		for i, v := range rsamps {
			if min > v && v == v {
				min = v
			}
			if max < v && v == v {
				max = v
			}
			vv := 2.0 * (((v - min) / (max - min)) - 0.5)
			outsamps[i] = int16(vv * 0x7fff)
		}
		if err := binary.Write(writer, binary.LittleEndian, outsamps); err != nil {
			panic(err)
		}
	}
}

func spectrogram(inf, outf string) {
	if err := nicerx.WriteSpectrogramFile(inf, outf, imageWidth); err != nil {
		panic(err)
	}
}

type xfmFunc func(iqr *radio.MixerIQReader, iqw *radio.IQWriter)

func applyXfmCmd(xf xfmFunc, inf, outf string) {
	iqr, rcloser := mustOpenInput(inf)
	defer rcloser()

	writer, wcloser := mustOpenIQW(outf)
	defer wcloser()
	xf(iqr, writer)
}

func downmixXfm(iqr *radio.MixerIQReader, iqw *radio.IQWriter) {
	inc := iqr.Batch64(int(iqr.Width), 5)
	for outSamps := range dsp.MixDown(float64(downmixHz), int(iqr.Width), inc) {
		iqw.Write64(outSamps)
	}
}

func lowpassXfm(iqr *radio.MixerIQReader, iqw *radio.IQWriter) {
	inc := iqr.Batch64(int(iqr.Width), 5)
	lpfc := dsp.Lowpass(float64(cutoffHz), int(iqr.Width), 1, inc)
	for outSamps := range lpfc {
		iqw.Write64(outSamps)
	}
}

func main() {
	rootCmd.Execute()
}
