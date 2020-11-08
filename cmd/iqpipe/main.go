package main

import (
	"encoding/binary"
	"os"

	"github.com/spf13/cobra"

	"github.com/chzchzchz/nicerx/dsp"
	"github.com/chzchzchz/nicerx/nicerx"
	"github.com/chzchzchz/nicerx/radio"
)

var rootCmd = &cobra.Command{
	Use:   "iqpipe",
	Short: "A tool to pipe around IQ modulation.",
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

	spectrogramCmd := &cobra.Command{
		Use:   "spectrogram [flags] input.iq8 output.jpg",
		Short: "Write spectrogram jpg",
		Run:   func(cmd *cobra.Command, args []string) { spectrogram(args[0], args[1]) },
	}
	spectrogramCmd.Flags().Uint32VarP(&sampleHz, "sample-rate", "s", 2048000, "Sample rate in Hz")
	spectrogramCmd.Flags().IntVarP(&imageWidth, "image-width", "w", 1024, "Width of FFT")
	rootCmd.AddCommand(spectrogramCmd)

	demodCommand := &cobra.Command{
		Use:   "fmdemod iqfile pcmfile",
		Short: "FM demodulate an iq8 file to PCM",
		Run:   func(cmd *cobra.Command, args []string) { demod(args[0], args[1]) },
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
	r := float64(pcmHz) / float64(sampleHz)
	resampc := dsp.Resample(float32(r), demodc)
	min, max := float32(1), float32(-1)
	for rsamps := range resampc {
		outsamps := make([]int16, len(rsamps))
		for i, v := range rsamps {
			if min > v {
				min = v
			}
			if max < v {
				max = v
			}
			outsamps[i] = int16((v / (max - min) * 0x7fff))
		}
		if err := binary.Write(fout, binary.LittleEndian, outsamps); err != nil {
			panic(err)
		}
	}
}

func spectrogram(inf, outf string) {
	if err := nicerx.WriteSpectrogramFile(inf, outf, imageWidth); err != nil {
		panic(err)
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
