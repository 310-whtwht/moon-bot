package jobs

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/moomoo-trading/core/backtest"
	"github.com/redis/go-redis/v9"
)

// Consumer reads backtest jobs from the Redis stream with a consumer group,
// so a job is handled once even with several bot instances.
type Consumer struct {
	Client *redis.Client
	Name   string // consumer name, unique per bot instance
	Handle func(ctx context.Context, backtestID string) error
	Block  time.Duration
	Stream string
	Group  string
}

func (c *Consumer) defaults() {
	if c.Block == 0 {
		c.Block = 5 * time.Second
	}
	if c.Stream == "" {
		c.Stream = backtest.JobStream
	}
	if c.Group == "" {
		c.Group = backtest.JobGroup
	}
}

// ensureGroup creates the stream and group. Starting at "0" means jobs queued
// while no bot was running are still processed.
func (c *Consumer) ensureGroup(ctx context.Context) error {
	err := c.Client.XGroupCreateMkStream(ctx, c.Stream, c.Group, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return fmt.Errorf("create consumer group: %w", err)
	}
	return nil
}

// Run consumes until ctx is cancelled. Messages left pending by a previous run
// of this consumer (e.g. a crash mid-job) are processed first.
func (c *Consumer) Run(ctx context.Context) error {
	c.defaults()
	if err := c.ensureGroup(ctx); err != nil {
		return err
	}

	start := "0" // own pending entries first, then new ones
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		streams, err := c.Client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group: c.Group, Consumer: c.Name, Streams: []string{c.Stream, start},
			Count: 10, Block: c.Block,
		}).Result()
		if errors.Is(err, redis.Nil) {
			continue
		}
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			log.Printf("jobs: read %s: %v (retrying)", c.Stream, err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second):
			}
			continue
		}

		for _, s := range streams {
			for _, msg := range s.Messages {
				c.process(ctx, msg)
			}
		}
		// Pending entries get one retry at startup; anything still failing stays
		// pending until the next restart instead of looping here.
		start = ">"
	}
}

func (c *Consumer) process(ctx context.Context, msg redis.XMessage) {
	id, _ := msg.Values[backtest.JobField].(string)
	if id == "" {
		log.Printf("jobs: %s %s: %v", c.Stream, msg.ID, errNoID)
	} else if err := c.Handle(ctx, id); err != nil {
		// Storage errors: leave the entry pending so it is retried on restart.
		log.Printf("jobs: backtest %s: %v", id, err)
		return
	}
	if err := c.Client.XAck(ctx, c.Stream, c.Group, msg.ID).Err(); err != nil {
		log.Printf("jobs: ack %s: %v", msg.ID, err)
	}
}
