package receiver

import (
	"github.com/chzchzchz/nicerx/radio"
)

type CaptureConfig struct {
	radio.HzBand
}

func NewCapture(target radio.HzBand) RxFunc {
	return nil
}
