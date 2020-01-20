package receiver

import (
	"context"

	"github.com/chzchzchz/nicerx/radio"
)

type RxStream struct {
	ch  <-chan interface{}
	err error
}

func (rs *RxStream) Err() error               { return rs.err }
func (rs *RxStream) Chan() <-chan interface{} { return rs.ch }

type RxFunc func(context.Context, *radio.MixerIQReader) *RxStream

type RxConfigBase struct {
	UserName string `json:"user_name"`
	TypeName string `json:"type_name"`
	radio.HzBand
}

type Rxer struct {
	RxConfigBase
	Open RxFunc
}
