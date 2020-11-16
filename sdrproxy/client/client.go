package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
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

func New(ep url.URL) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{Config: Config{ep}, ctx: ctx, cancel: cancel}
}

func (c *Client) OpenIQReader(ctx context.Context, rxreq sdrproxy.RxRequest) (*radio.IQReader, error) {
	rxreqBytes, err := json.Marshal(rxreq)
	if err != nil {
		return nil, err
	}

	// Note: no trailling "/" => 301 => rewrite to POST
	u := c.Endpoint.String() + "/api/rx/"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewBuffer(rxreqBytes))
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf(http.StatusText(resp.StatusCode))
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		<-ctx.Done()
		resp.Body.Close()
	}()

	return radio.NewIQReader(resp.Body), nil
}

func (c *Client) Signals(ctx context.Context) (msg []sdrproxy.RxSignal, err error) {
	u := c.Endpoint.String() + "/api/rx/"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &msg); err != nil {
		return nil, err
	}
	return msg, nil
}

func (c *Client) Close() error {
	c.cancel()
	c.wg.Wait()
	http.DefaultClient.CloseIdleConnections()
	return nil
}
