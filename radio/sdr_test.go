package radio

import (
	"context"
	"testing"
	"time"
)

var testRadioSerial = "0"

func TestSampleRate(t *testing.T) {
	// Rate must divide crystal 28.8MHz (5^5*3^2*2^10) to avoid resampling.
	testRate := 240000
	testSeconds := 5 * time.Second
	timeoutTime := 2*testSeconds + (10 * time.Second)
	ctx, cancel := context.WithTimeout(context.TODO(), timeoutTime)
	defer cancel()

	sdr, err := NewSDRWithSerial(ctx, testRadioSerial)
	if err != nil {
		t.Fatal(err)
	}
	defer sdr.Close()

	// Override calibration.
	if err = sdr.SetFreqCorrection(5); err != nil {
		t.Fatal(err)
	}
	band := HzBand{Center: 100 * 1e6, Width: uint64(testRate)}
	if err = sdr.SetBand(band); err != nil {
		t.Fatal(err)
	}
	sigc := sdr.Reader().BatchStream64(ctx, testRate, 0)

	// Drop first sample batch to measure closer to device-rate.
	select {
	case <-sigc:
	case <-ctx.Done():
	}

	start := time.Now()
	timer, samples := time.NewTimer(testSeconds), 0
	for ctx.Err() == nil {
		select {
		case sig, ok := <-sigc:
			if !ok {
				cancel()
			}
			samples += len(sig)
		case <-timer.C:
			cancel()
		}
	}
	end := time.Now()
	seconds := end.Sub(start).Seconds()

	sps := float64(samples) / seconds
	if sps < 0.95*float64(testRate) || sps > 1.05*float64(testRate) {
		t.Fatalf("expected 5%% from rate %v, got %v", testRate, sps)
	}
	t.Logf("time: %.2g, got %.4gMSPS\n", seconds, sps/1e6)
}
