// Package broker fans job changes out to server-sent-event subscribers.
package broker

import (
	"sync"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/application"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain"
)

// subscriberBuffer absorbs bursts (progress ticks of several uploads). A
// subscriber that falls further behind misses events; the next event of the
// same job carries its full state, so nothing is lost for good.
const subscriberBuffer = 64

// Broker is an in-memory publish/subscribe hub.
type Broker struct {
	mu   sync.Mutex
	subs map[chan domain.UploadJob]struct{}
}

var _ application.Publisher = (*Broker)(nil)

// New builds a broker.
func New() *Broker {
	return &Broker{subs: map[chan domain.UploadJob]struct{}{}}
}

// Subscribe returns a channel of job changes and a function to stop receiving.
func (b *Broker) Subscribe() (<-chan domain.UploadJob, func()) {
	ch := make(chan domain.UploadJob, subscriberBuffer)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	var once sync.Once
	return ch, func() {
		once.Do(func() {
			b.mu.Lock()
			delete(b.subs, ch)
			b.mu.Unlock()
			close(ch)
		})
	}
}

// Publish implements application.Publisher. It never blocks.
func (b *Broker) Publish(job domain.UploadJob) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- job:
		default:
		}
	}
}
