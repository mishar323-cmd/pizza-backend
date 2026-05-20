package iiko

import (
	"context"
	"errors"
)

var ErrNotImplemented = errors.New("iiko integration not yet implemented")

type Client struct{}

func NewClient() *Client { return &Client{} }

func (c *Client) SendOrder(ctx context.Context, raw map[string]any) error {
	return ErrNotImplemented
}
