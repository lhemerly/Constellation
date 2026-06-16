package node

import (
	"errors"
	"math/rand"
	"time"
)

// ErrChaosInjected is returned when the chaos middleware randomly decides to return an error.
var ErrChaosInjected = errors.New("chaos injected error")

// ChaosConfig holds the probabilities for injecting different types of faults.
// Probabilities should be between 0.0 and 1.0.
type ChaosConfig struct {
	LatencyProb float64
	ErrorProb   float64
	PanicProb   float64
	MaxLatency  time.Duration
}

// ChaosMiddleware injects faults (latency, errors, panics) based on configured probabilities to test system resilience.
func ChaosMiddleware(config ChaosConfig) Middleware {
	return func(next func([]byte) ([]byte, error)) func([]byte) ([]byte, error) {
		return func(input []byte) ([]byte, error) {
			if rand.Float64() < config.PanicProb {
				panic("chaos injected panic")
			}

			if rand.Float64() < config.ErrorProb {
				return nil, ErrChaosInjected
			}

			if rand.Float64() < config.LatencyProb && config.MaxLatency > 0 {
				latency := time.Duration(rand.Int63n(int64(config.MaxLatency)))
				time.Sleep(latency)
			}

			return next(input)
		}
	}
}
