package processbar

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"
)

type Bar struct {
	ctx       context.Context
	cancel    context.CancelFunc
	total     int
	count     int
	percent   int
	tag       string
	format    string
	startTime time.Time
	ticker    *time.Ticker
	mut       sync.Mutex
}

func New(total int) *Bar {
	bar := &Bar{
		total:  total,
		tag:    "#",
		format: "\r[%-50s] %3d%% %" + fmt.Sprintf("%d", digitCount(total)) + "d/%d %-11s",
	}

	return bar
}

func (b *Bar) SetTag(tag string) *Bar {
	b.tag = tag
	return b
}

func (b *Bar) Incr() {
	b.mut.Lock()
	if b.count == 0 {
		b.startTime = time.Now()
	}
	b.count++
	b.mut.Unlock()
}

func (b *Bar) Flush() {
	b.calculate()
	b.display()
}

func (b *Bar) AutoFlush(interval time.Duration) {
	b.stop()
	ctx, cancel := context.WithCancel(context.Background())
	b.ctx = ctx
	b.cancel = cancel
	b.ticker = time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-b.ticker.C:
				b.Flush()
			case <-b.ctx.Done():
				b.ticker.Stop()
				return
			}
		}
	}()
}

func (b *Bar) stop() {
	if b.cancel != nil {
		b.cancel()
	}
}

func (b *Bar) calculate() {
	b.mut.Lock()
	if b.count >= b.total {
		b.percent = 100
	} else {
		b.percent = b.count * 100 / b.total
	}
	b.mut.Unlock()
}

func (b *Bar) display() {
	b.mut.Lock()
	secs := time.Second * time.Duration(int(time.Since(b.startTime)/time.Second))
	fmt.Fprintf(os.Stderr, b.format, strings.Repeat(b.tag, b.percent/2), b.percent, b.count, b.total, secs)
	b.mut.Unlock()
}

func (b *Bar) Finish() {
	b.stop()
	fmt.Fprintln(os.Stderr)
}

func digitCount(n int) int {
	return int(math.Floor(math.Log10(float64(n)))) + 1
}
