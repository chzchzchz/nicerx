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
