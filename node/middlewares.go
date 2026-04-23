package node

import (
	"errors"
	"log"
	"time"
)

// LoggingMiddleware logs the payload size and processing time.
func LoggingMiddleware(next func([]byte) ([]byte, error)) func([]byte) ([]byte, error) {
	return func(input []byte) ([]byte, error) {
		start := time.Now()
		log.Printf("Process started. Input size: %d bytes\n", len(input))

		output, err := next(input)

		duration := time.Since(start)
		if err != nil {
			log.Printf("Process failed after %s: %v\n", duration, err)
		} else {
			log.Printf("Process completed in %s. Output size: %d bytes\n", duration, len(output))
		}

		return output, err
	}
}

// RetryMiddleware retries a failed operation for a specified number of times with a delay.
func RetryMiddleware(retries int, delay time.Duration) Middleware {
	return func(next func([]byte) ([]byte, error)) func([]byte) ([]byte, error) {
		return func(input []byte) ([]byte, error) {
			var err error
			var output []byte

			for i := 0; i <= retries; i++ {
				// Clone the input buffer for each attempt to avoid side effects
				// from in-place modifications during a failed process attempt.
				inputCopy := make([]byte, len(input))
				copy(inputCopy, input)

				output, err = next(inputCopy)
				if err == nil {
					return output, nil
				}

				if i < retries {
					time.Sleep(delay)
				}
			}

			return nil, errors.Join(errors.New("operation failed after retries"), err)
		}
	}
}

// RecoveryMiddleware recovers from panics gracefully and turns them into errors.
func RecoveryMiddleware(next func([]byte) ([]byte, error)) func([]byte) ([]byte, error) {
	return func(input []byte) (output []byte, err error) {
		defer func() {
			if r := recover(); r != nil {
				switch x := r.(type) {
				case string:
					err = errors.New(x)
				case error:
					err = x
				default:
					err = errors.New("unknown panic occurred")
				}
			}
		}()

		return next(input)
	}
}
