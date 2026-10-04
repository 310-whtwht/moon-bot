package trader

import (
	"context"
	"fmt"
	"time"

	"github.com/moomoo-trading/core/notify"
)

var eventLabels = map[string]string{
	"opened":        ":large_green_circle: 新規約定",
	"closed":        ":white_circle: 決済",
	"rejected":      ":warning: 注文見送り",
	"skipped":       ":no_entry_sign: 新規停止中",
	"error":         ":rotating_light: エラー",
	"kill_switch":   ":octagonal_sign: Kill Switch",
	"daily_summary": ":bar_chart: 日次サマリ",
}

// SlackNotifier sends events to a Slack webhook from a background goroutine,
// so trading never waits on Slack. Events are dropped when the queue is full.
type SlackNotifier struct {
	queue chan Event
	send  func(ctx context.Context, text string) error
	logf  func(format string, args ...any)
}

// NewSlackNotifier starts the sender; it stops when ctx is cancelled.
func NewSlackNotifier(ctx context.Context, webhookURL string, logf func(string, ...any)) *SlackNotifier {
	n := &SlackNotifier{
		queue: make(chan Event, 100),
		send:  func(ctx context.Context, text string) error { return notify.Slack(ctx, webhookURL, text) },
		logf:  logf,
	}
	go n.run(ctx)
	return n
}

func (n *SlackNotifier) Notify(e Event) {
	select {
	case n.queue <- e:
	default:
		n.logf("slack: queue full, dropped %s event", e.Kind)
	}
}

func (n *SlackNotifier) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case e := <-n.queue:
			sendCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			if err := n.send(sendCtx, FormatEvent(e)); err != nil {
				n.logf("slack: %v", err)
			}
			cancel()
		}
	}
}

// FormatEvent renders an event as one Slack message.
func FormatEvent(e Event) string {
	label, ok := eventLabels[e.Kind]
	if !ok {
		label = e.Kind
	}
	if e.Deployment.Name == "" {
		return fmt.Sprintf("%s\n%s", label, e.Message)
	}
	return fmt.Sprintf("%s — %s\n%s", label, e.Deployment.Name, e.Message)
}
