package provider

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"
)

type SimulatedProvider struct{}

func NewSimulatedProvider() *SimulatedProvider {
	return &SimulatedProvider{}
}

func (p *SimulatedProvider) Send(ctx context.Context, to, subject, body string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(200 * time.Millisecond):
	}

	if rand.Float64() < 0.30 {
		return fmt.Errorf("simulated provider failure")
	}

	log.Printf("[Notification] Simulated email sent to %s with subject %q", to, subject)
	return nil
}
