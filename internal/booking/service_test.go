package booking

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/Gaurish_Maheshwari/cinema-ticket-booking/internal/adapters/redis"
)

func TestConcurrentBooking_ExactlyOneWins(t *testing.T) {
	store := NewRedisStore(redis.NewClient("localhost:6379"))
	svc := NewService(store)

	movieID := "test-" + uuid.New().String() // fresh key every run

	const numGoroutines = 100_000 // 100k users trying to book a seat at the same time

	var (
		successes atomic.Int64
		failures  atomic.Int64
		wg        sync.WaitGroup
	)

	wg.Add(numGoroutines)
	for i := range numGoroutines {
		go func(userNum int) {
			defer wg.Done()
			_, err := svc.Book(Booking{
				MovieID: movieID,
				SeatID:  "A1",
				UserID:  uuid.New().String(),
			})
			if err == nil {
				successes.Add(1)
			} else {
				failures.Add(1)
			}
		}(i)
	}
	wg.Wait()

	if got := successes.Load(); got != 1 {
		t.Errorf("expected exactly 1 success, got %d", got)
	}
	if got := failures.Load(); got != int64(numGoroutines-1) {
		t.Errorf("expected %d failures, got %d", numGoroutines-1, got)
	}
}