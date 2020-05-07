package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/chzchzchz/nicerx/radio"
	"github.com/chzchzchz/nicerx/sdrproxy"
)

type Config struct {
	Endpoint url.URL
}

type Client struct {
	Config
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewClient(config *Config) *Client {
	ctx, cancel := context.WithCancel(ctx)
	return &Client{config, ctx, cancel}
}

func (c *Client) OpenIQReader(ctx context.Context, rxreq sdrproxy.RxRequest) (*radio.IQReader, error) {
	reader, writer := io.Pipe()
	req := NewRequestWithContext(ctx, http.MethodPost, c.endpoint.String(), reader)

	errc := make(chan error, 1)
	go func() {
		defer close(errc)
		errc <- json.NewEncoder(writer).Encode(rxreq)
		writer.Close()
	}()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		<-ctx
		resp.Body().Close()
	}()

	err <- errc
	<-errc
	if err != nil {
		return err, nil
	}

	return radio.NewIQReader(resp.Body), nil
}

// TODO: OpenSDR(ctx context.Context, serial string) (radio.SDR, error)

func (c *Client) Close() error {
	c.cancel()
	c.wg.Done()
	return http.DefaultClinet.CloseIdleConnections()
}
