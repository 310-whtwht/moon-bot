package trader

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatEvent(t *testing.T) {
	msg := FormatEvent(Event{Kind: "opened", Deployment: Deployment{Name: "EMA USD/JPY"}, Message: "BUY 100 USD_JPY @ 150.01"})
	assert.Contains(t, msg, "新規約定")
	assert.Contains(t, msg, "EMA USD/JPY")
	assert.Contains(t, msg, "BUY 100 USD_JPY @ 150.01")

	msg = FormatEvent(Event{Kind: "kill_switch", Message: "Kill Switch global: 発動"})
	assert.Contains(t, msg, "Kill Switch")
	assert.NotContains(t, msg, " — \n", "no deployment separator without a deployment")

	assert.Contains(t, FormatEvent(Event{Kind: "custom", Message: "x"}), "custom")
}

func TestSlackNotifier_SendsInBackgroundAndNeverBlocks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	var sent []string
	release := make(chan struct{})
	n := &SlackNotifier{
		queue: make(chan Event, 2),
		logf:  func(string, ...any) {},
		send: func(_ context.Context, text string) error {
			<-release // Slack is slow
			mu.Lock()
			sent = append(sent, text)
			mu.Unlock()
			return errors.New("ignored: failures are only logged")
		},
	}
	go n.run(ctx)

	done := make(chan struct{})
	go func() {
		for i := 0; i < 10; i++ { // far more than the queue holds
			n.Notify(Event{Kind: "opened", Message: "x"})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Notify blocked while Slack was slow")
	}

	close(release)
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(sent) >= 2
	}, time.Second, 10*time.Millisecond)
	mu.Lock()
	assert.LessOrEqual(t, len(sent), 3, "the rest were dropped, not queued forever")
	mu.Unlock()
}
