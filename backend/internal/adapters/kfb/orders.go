package kfb

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
)

type OrdersResult struct {
	Orders []map[string]any
	Source string
}

// FetchOrders reads the current list from the selected connection, including
// QUIK orders that were entered outside this application.
func (c *Client) FetchOrders(ctx context.Context) (OrdersResult, error) {
	result := OrdersResult{Orders: []map[string]any{}}
	if !c.Configured() {
		return result, fmt.Errorf("kfb order list is not configured")
	}
	version := c.ConnectionVersion()
	offset := 0
	for {
		path := "/api/v1/orders"
		if offset > 0 {
			path += "?offset=" + strconv.Itoa(offset)
		}
		raw, err := c.get(ctx, path)
		if err != nil {
			return OrdersResult{}, err
		}
		var page struct {
			Orders     json.RawMessage `json:"orders"`
			Source     string          `json:"source"`
			HasMore    bool            `json:"has_more"`
			NextOffset int             `json:"next_offset"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return OrdersResult{}, err
		}
		if len(page.Orders) == 0 {
			return OrdersResult{}, fmt.Errorf("kfb order list is missing orders")
		}
		var items []map[string]any
		if err := json.Unmarshal(page.Orders, &items); err != nil {
			return OrdersResult{}, err
		}
		if c.ConnectionVersion() != version {
			return OrdersResult{}, fmt.Errorf("kfb connection changed while reading orders")
		}
		result.Orders = append(result.Orders, items...)
		if result.Source == "" {
			result.Source = page.Source
		}
		if !page.HasMore {
			return result, nil
		}
		if page.NextOffset <= offset || len(items) == 0 {
			return OrdersResult{}, fmt.Errorf("invalid kfb order pagination")
		}
		offset = page.NextOffset
	}
}
