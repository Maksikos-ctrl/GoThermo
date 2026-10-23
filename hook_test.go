package main

import (
	"context"

	"github.com/go-redis/redis/v8"
)

// hookFunc runs fn once before each pipeline/transaction is sent.
type hookFunc func()

func (h hookFunc) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	return ctx, nil
}
func (h hookFunc) AfterProcess(ctx context.Context, cmd redis.Cmder) error { return nil }
func (h hookFunc) BeforeProcessPipeline(ctx context.Context, cmds []redis.Cmder) (context.Context, error) {
	h()
	return ctx, nil
}
func (h hookFunc) AfterProcessPipeline(ctx context.Context, cmds []redis.Cmder) error { return nil }
