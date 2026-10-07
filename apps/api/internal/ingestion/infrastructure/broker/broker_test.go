package broker_test

import (
	"testing"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/infrastructure/broker"
)

func TestBrokerFanOutAndUnsubscribe(t *testing.T) {
	t.Parallel()

	b := broker.New()
	a, stopA := b.Subscribe()
	c, stopC := b.Subscribe()

	b.Publish(domain.UploadJob{ID: "1"})
	if (<-a).ID != "1" || (<-c).ID != "1" {
		t.Fatal("both subscribers must receive the event")
	}

	stopA()
	stopA() // idempotent
	if _, ok := <-a; ok {
		t.Fatal("channel must be closed after unsubscribe")
	}
	b.Publish(domain.UploadJob{ID: "2"})
	if (<-c).ID != "2" {
		t.Fatal("remaining subscriber must keep receiving")
	}
	stopC()
}

func TestBrokerNeverBlocksOnSlowSubscribers(t *testing.T) {
	t.Parallel()

	b := broker.New()
	_, stop := b.Subscribe() // never read
	defer stop()
	for range 1000 {
		b.Publish(domain.UploadJob{ID: "x"}) // must not deadlock
	}
}
