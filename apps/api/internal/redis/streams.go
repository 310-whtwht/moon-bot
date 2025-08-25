package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// Stream names
	TradeEventsStream    = "trade_events"
	CircuitEventsStream  = "circuit_events"
	StrategyLogsStream   = "strategy_logs"
	OrderEventsStream    = "order_events"
	
	// Consumer group names
	TradeEventsGroup     = "trade_events_group"
	CircuitEventsGroup   = "circuit_events_group"
	StrategyLogsGroup    = "strategy_logs_group"
	OrderEventsGroup     = "order_events_group"
	
	// DLQ stream names
	TradeEventsDLQ       = "trade_events_dlq"
	CircuitEventsDLQ     = "circuit_events_dlq"
	StrategyLogsDLQ      = "strategy_logs_dlq"
	OrderEventsDLQ       = "order_events_dlq"
)

type StreamManager struct {
	client *redis.Client
}

func NewStreamManager(client *redis.Client) *StreamManager {
	return &StreamManager{
		client: client,
	}
}

// InitializeStreams creates all required streams and consumer groups
func (sm *StreamManager) InitializeStreams(ctx context.Context) error {
	streams := []string{
		TradeEventsStream,
		CircuitEventsStream,
		StrategyLogsStream,
		OrderEventsStream,
	}

	for _, stream := range streams {
		if err := sm.createStreamIfNotExists(ctx, stream); err != nil {
			return fmt.Errorf("failed to create stream %s: %w", stream, err)
		}
	}

	// Create consumer groups
	groups := map[string]string{
		TradeEventsStream:   TradeEventsGroup,
		CircuitEventsStream: CircuitEventsGroup,
		StrategyLogsStream:  StrategyLogsGroup,
		OrderEventsStream:   OrderEventsGroup,
	}

	for stream, group := range groups {
		if err := sm.createConsumerGroupIfNotExists(ctx, stream, group); err != nil {
			return fmt.Errorf("failed to create consumer group %s for stream %s: %w", group, stream, err)
		}
	}

	return nil
}

// createStreamIfNotExists creates a stream if it doesn't exist
func (sm *StreamManager) createStreamIfNotExists(ctx context.Context, streamName string) error {
	// Add a dummy message to create the stream
	_, err := sm.client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamName,
		Values: map[string]interface{}{
			"init": "stream_created",
			"timestamp": time.Now().Unix(),
		},
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to create stream %s: %w", streamName, err)
	}

	// Delete the dummy message
	sm.client.XDel(ctx, streamName, "0-1")

	return nil
}

// createConsumerGroupIfNotExists creates a consumer group if it doesn't exist
func (sm *StreamManager) createConsumerGroupIfNotExists(ctx context.Context, streamName, groupName string) error {
	// Check if group exists
	groups, err := sm.client.XInfoGroups(ctx, streamName).Result()
	if err != nil {
		return fmt.Errorf("failed to get groups for stream %s: %w", streamName, err)
	}

	for _, group := range groups {
		if group.Name == groupName {
			return nil // Group already exists
		}
	}

	// Create the group
	_, err = sm.client.XGroupCreate(ctx, streamName, groupName, "0").Result()
	if err != nil {
		return fmt.Errorf("failed to create consumer group %s for stream %s: %w", groupName, streamName, err)
	}

	return nil
}

// PublishTradeEvent publishes a trade event to the stream
func (sm *StreamManager) PublishTradeEvent(ctx context.Context, event map[string]interface{}) error {
	_, err := sm.client.XAdd(ctx, &redis.XAddArgs{
		Stream: TradeEventsStream,
		Values: event,
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to publish trade event: %w", err)
	}

	return nil
}

// PublishCircuitEvent publishes a circuit event to the stream
func (sm *StreamManager) PublishCircuitEvent(ctx context.Context, event map[string]interface{}) error {
	_, err := sm.client.XAdd(ctx, &redis.XAddArgs{
		Stream: CircuitEventsStream,
		Values: event,
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to publish circuit event: %w", err)
	}

	return nil
}

// PublishStrategyLog publishes a strategy log to the stream
func (sm *StreamManager) PublishStrategyLog(ctx context.Context, log map[string]interface{}) error {
	_, err := sm.client.XAdd(ctx, &redis.XAddArgs{
		Stream: StrategyLogsStream,
		Values: log,
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to publish strategy log: %w", err)
	}

	return nil
}

// PublishOrderEvent publishes an order event to the stream
func (sm *StreamManager) PublishOrderEvent(ctx context.Context, event map[string]interface{}) error {
	_, err := sm.client.XAdd(ctx, &redis.XAddArgs{
		Stream: OrderEventsStream,
		Values: event,
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to publish order event: %w", err)
	}

	return nil
}

// ConsumeEvents consumes events from a stream with retry logic
func (sm *StreamManager) ConsumeEvents(ctx context.Context, streamName, groupName, consumerName string, handler func([]redis.XMessage) error) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Read messages from the stream
			messages, err := sm.client.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    groupName,
				Consumer: consumerName,
				Streams:  []string{streamName, ">"},
				Count:    10,
				Block:    5 * time.Second,
			}).Result()

			if err != nil {
				if err == redis.Nil {
					continue // No messages available
				}
				return fmt.Errorf("failed to read from stream %s: %w", streamName, err)
			}

			for _, stream := range messages {
				for _, message := range stream.Messages {
					// Process the message
					if err := handler([]redis.XMessage{message}); err != nil {
						// Move to DLQ
						dlqStream := streamName + "_dlq"
						sm.client.XAdd(ctx, &redis.XAddArgs{
							Stream: dlqStream,
							Values: map[string]interface{}{
								"original_message": message.Values,
								"error":           err.Error(),
								"timestamp":       time.Now().Unix(),
							},
						})

						// Acknowledge the message to remove it from pending
						sm.client.XAck(ctx, streamName, groupName, message.ID)
						continue
					}

					// Acknowledge the message
					sm.client.XAck(ctx, streamName, groupName, message.ID)
				}
			}
		}
	}
}