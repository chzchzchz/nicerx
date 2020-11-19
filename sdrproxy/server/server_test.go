package server

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/chzchzchz/nicerx/radio"
	"github.com/chzchzchz/nicerx/sdrproxy"
)

var testRadioSerial = "20000001"
var testBand = radio.HzBand{Center: 100000000, Width: 24000}

func TestConcurrentOpen(t *testing.T) {
	s := NewServer()
	defer s.Close()

	errc := make(chan error, 5)

	tests := []struct {
		name func(int) string
	}{
		{
			name: func(i int) string { return fmt.Sprintf("test-uniq-%d", i) },
		},
		{
			name: func(i int) string { return "test-same" },
		},
	}
	for tidx, tt := range tests {
		cctx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
		defer cancel()
		for i := 0; i < 5; i++ {
			go func(n string) {
				req := sdrproxy.RxRequest{
					HzBand: testBand,
					Name:   n,
				}
				sig, err := s.OpenSignal(cctx, req)
				if sig != nil {
					defer sig.Close()
				}
				errc <- err
			}(tt.name(i))
		}
		for i := 0; i < 5; i++ {
			if err := <-errc; err != nil {
				t.Errorf("%s(%d) %v", tt.name(tidx), tidx, err)
			}
		}
	}
}
