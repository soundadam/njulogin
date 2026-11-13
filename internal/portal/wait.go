package portal

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Poll re-reads portal state until an accepted operation is observed or
// Timeout passes. The first read happens immediately.
type Poll struct {
	Timeout  time.Duration
	Interval time.Duration
}

// DefaultPoll bounds postcondition checks for every mutating command.
var DefaultPoll = Poll{Timeout: 10 * time.Second, Interval: 250 * time.Millisecond}

// AuthState waits until the reported authentication state equals want and
// returns the status that satisfied it.
func (p Poll) AuthState(ctx context.Context, reader StatusReader, want bool) (Status, error) {
	failure := "operation accepted but unauthenticated state was not observed before timeout"
	if want {
		failure = "operation accepted but authenticated state was not observed before timeout"
	}
	var status Status
	err := p.until(ctx, failure, func(ctx context.Context) (bool, error) {
		var err error
		status, err = reader.Status(ctx)
		return err == nil && status.Authenticated == want, err
	})
	return status, err
}

// Rows waits until done accepts the rows returned by list.
func (p Poll) Rows(ctx context.Context, list func(context.Context) ([]Row, error), done func([]Row) bool, failure string) error {
	return p.until(ctx, failure, func(ctx context.Context) (bool, error) {
		rows, err := list(ctx)
		return err == nil && done(rows), err
	})
}

func (p Poll) until(ctx context.Context, failure string, check func(context.Context) (bool, error)) error {
	deadline := time.Now().Add(p.Timeout)
	for {
		ok, err := check(ctx)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		if !time.Now().Before(deadline) {
			return errors.New(failure)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(p.Interval):
		}
	}
}

// IDAbsent reports whether no row has the given id.
func IDAbsent(id string) func([]Row) bool {
	return func(rows []Row) bool {
		for _, row := range rows {
			if row.ID() == id {
				return false
			}
		}
		return true
	}
}

// DevicePresent reports whether some quick-login row has the given name.
func DevicePresent(name string) func([]Row) bool {
	return func(rows []Row) bool {
		for _, row := range rows {
			if strings.TrimSpace(row.Field("device")) == name {
				return true
			}
		}
		return false
	}
}
