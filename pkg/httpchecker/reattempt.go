package httpchecker

import (
	"math/rand"
	"time"

	log "github.com/sirupsen/logrus"
)

func init() {
	rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec
}

type stop struct {
	error
}

// Retry mechanism
func Reattempt(attempts int, sleep time.Duration, f func() error) error {
	if err := f(); err != nil {
		if s, ok := err.(stop); ok {
			// Log the original error for later checking
			log.Errorf("connection check failed with error: %s", s.error)
		}

		if attempts--; attempts > 0 {
			// Add some randomness to prevent creating a Thundering Herd
			jitter := time.Duration(rand.Int63n(int64(sleep))) //nolint:gosec
			sleep = sleep + jitter/2

			time.Sleep(time.Duration(time.Duration(sleep.Seconds()).Seconds()))
			return Reattempt(attempts, 2*sleep, f)
		}
		return err
	}
	return nil
}
