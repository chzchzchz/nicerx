package main

import (
	"context"
	"log"
	"net/url"
	"sync"

	"github.com/spf13/cobra"

	"github.com/chzchzchz/nicerx/radio"
	"github.com/chzchzchz/nicerx/sdrproxy"
	"github.com/chzchzchz/nicerx/sdrproxy/client"
)

var (
	endpoint string
	minKHz   int
)

var rootCmd = &cobra.Command{
	Use:   "sdrmon",
	Short: "Monitor SDRs",
	Run:   func(cmd *cobra.Command, args []string) { run() },
}

func init() {
	rootCmd.Flags().StringVarP(&endpoint, "url", "", "http://localhost:12000", "URL for sdrproxy")
	rootCmd.Flags().IntVarP(&minKHz, "min-khz", "", 5, "Minimum KHz to inspect for signal")
}

func doMonitor(ctx context.Context, c *client.Client, r radio.SDRHWInfo, sigs []sdrproxy.RxSignal) {
	log.Printf("monitoring %+v", r.Id)
	band := radio.HzBand{Center: r.CenterHz, Width: uint64(r.SampleRate)}
	rxreq := sdrproxy.RxRequest{
		Name:   "sdrmon-" + r.Id,
		Radio:  r.Id,
		HzBand: band,
	}
	iqr, err := c.OpenIQReader(ctx, rxreq)
	if err != nil {
		panic(err)
	}

	// Filter out known bands.
	knownBands := make([]radio.FreqBand, 0, len(sigs))
	for _, sig := range sigs {
		// Ignore anything monitoring entire band.
		if sig.Request.HzBand != band {
			knownBands = append(knownBands, sig.Request.ToMHz())
		}
	}
	knownBands = radio.BandMerge(knownBands)

	// Split into 500hz chunks; 2ms windows.
	bins := int(r.SampleRate / 500)
	sp := radio.NewSpectralPower(band.ToMHz(), bins, 20)
	iqrc := iqr.BatchStream64(ctx, bins, 0)
	if row := <-iqrc; row == nil {
		panic("could not read first row")
	}

	var bands []radio.FreqBand
	for {
		// Measure for one second.
		for i := 0; i < 25; i++ {
			if err := sp.Measure(iqrc); err != nil {
				panic(err)
			}
			for _, v := range sp.Bands() {
				if int(v.BandwidthKHz()) < minKHz {
					continue
				}
				ol := false
				for _, kb := range knownBands {
					if ol = kb.Overlaps(v); ol {
						break
					}
				}
				if !ol {
					bands = append(bands, v)
				}
			}
		}
		bands = radio.BandMerge(bands)
		for _, v := range bands {
			log.Printf("[%s] %+v", r.Id, v)
		}
	}
}

func run() {
	u, err := url.Parse(endpoint)
	if err != nil {
		panic(err)
	}
	c := client.New(*u)
	log.Println(u.String())
	defer c.Close()

	cctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigs, err := c.Signals(cctx)
	if err != nil {
		panic(err)
	}

	// Collect active SDRs.
	sdrs := make(map[string]radio.SDRHWInfo)
	sigs2sdr := make(map[string][]sdrproxy.RxSignal)
	for _, sig := range sigs {
		r := sig.Response.Radio
		sdrs[r.Id] = r
		sigs2sdr[r.Id] = append(sigs2sdr[r.Id], sig)
	}

	var wg sync.WaitGroup
	wg.Add(len(sdrs))
	for _, r := range sdrs {
		rr := r
		go func() {
			defer wg.Done()
			doMonitor(cctx, c, rr, sigs2sdr[rr.Id])
		}()
	}
	wg.Wait()
}

func main() {
	rootCmd.Execute()
}
