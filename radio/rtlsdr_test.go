package radio

import (
	"context"
	"testing"
	"time"
)

func TestRTLBadRate(t *testing.T) {
	testRate := 24000
	ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancel()

	rtlsdr, err := newRTLSDR(ctx, testRadioSerial)
	if err != nil {
		t.Fatal(err)
	}
	defer rtlsdr.Close()
	if err = rtlsdr.SetSampleRate(uint32(testRate)); err == nil {
		t.Fatal("expected error on setting bad rate")
	}
}

func TestRTLList(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancel()
	sdrs, err := rtlSDRList(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(sdrs) == 0 {
		t.Fatal("did not detect any sdrs")
	}
}

// Test hammering creating/destroying a device.
func TestRTLHammerOpenClose(t *testing.T) {
	for i := 0; i < 10; i++ {
		ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
		rtlsdr, err := newRTLSDR(ctx, testRadioSerial)
		if err != nil {
			t.Fatal(err)
		}
		rtlsdr.Close()
		cancel()
	}
}
