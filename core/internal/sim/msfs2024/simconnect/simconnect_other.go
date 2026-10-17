//go:build !windows

package simconnect

import "context"

func (c *Client) run(ctx context.Context, logf func(string, ...any)) {
	c.set(StateNoDLL, errUnsupported.Error())
}
