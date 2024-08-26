//
//
//

package httpstatus

import (
	"context"
	"time"
)

type Options_t struct {
	NoLogs bool
}

type CtxOption func(self *Options_t)

func NoLogs(in bool) CtxOption {
	return func(self *Options_t) {
		self.NoLogs = in
	}
}

type Contexter interface {
	Ctx() (context.Context, context.CancelFunc)
	Options() Options_t
}

type Pass struct {
	ctx     context.Context
	options Options_t
}

func ContextPass(ctx context.Context, options ...CtxOption) Contexter {
	res := Pass{ctx: ctx}
	for _, v := range options {
		v(&res.options)
	}
	return res
}

func (self Pass) Ctx() (context.Context, context.CancelFunc) {
	return self.ctx, func() {}
}

func (self Pass) Options() Options_t {
	return self.options
}

type Timeout struct {
	ctx     context.Context
	timeout time.Duration
	options Options_t
}

func ContextTimeout(ctx context.Context, timeout time.Duration, options ...CtxOption) Contexter {
	res := Timeout{ctx: ctx, timeout: timeout}
	for _, v := range options {
		v(&res.options)
	}
	return res
}

func (self Timeout) Ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(self.ctx, self.timeout)
}

func (self Timeout) Options() Options_t {
	return self.options
}
