// Package common implements common functionality for both the API and worker servers.
package common

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gocloud.dev/pubsub"
	"gopkg.in/op/go-logging.v1"
)

var log = logging.MustGetLogger("common")

// MustOpenSubscription opens a subscription, which must have been created ahead of time.
// It dies on any errors.
func MustOpenSubscription(url string) *pubsub.Subscription {
	ctx, cancel := context.WithCancel(context.Background())
	s, err := pubsub.OpenSubscription(ctx, url)
	if err != nil {
		log.Fatalf("Failed to open subscription %s: %s", url, err)
	}
	handleSignals(cancel, s)
	return s
}

// MustOpenTopic opens a topic, which must have been created ahead of time.
func MustOpenTopic(url string) *pubsub.Topic {
	ctx, cancel := context.WithCancel(context.Background())
	t, err := pubsub.OpenTopic(ctx, url)
	if err != nil {
		log.Fatalf("Failed to open topic %s: %s", url, err)
	}
	handleSignals(cancel, t)
	return t
}

func handleSignals(cancel context.CancelFunc, s Shutdownable) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGABRT, syscall.SIGTERM)
	go func() {
		log.Warning("Received signal %s, shutting down queue", <-ch)
		ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
		if err := s.Shutdown(ctx); err != nil {
			log.Error("Failed to shut down queue: %s", err)
		}
		cancel()
		log.Fatalf("Shutting down server")
	}()
}

type Shutdownable interface {
	Shutdown(context.Context) error
}
