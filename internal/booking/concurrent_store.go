package booking

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

var ErrSessionNotFound = errors.New("session not found or expired")

// Compile-time check: fails to build if ConcurentStore stops satisfying BookingStore.
var _ BookingStore = (*ConcurentStore)(nil)

// ConcurentStore is an in-memory BookingStore guarded by a RWMutex.
type ConcurentStore struct {
	bookings map[string]Booking // seatKey (movieID:seatID) -> booking
	sessions map[string]string  // sessionID -> seatKey
	sync.RWMutex
}

func NewConcurentStore() *ConcurentStore {
	return &ConcurentStore{
		bookings: map[string]Booking{},
		sessions: map[string]string{},
	}
}

func seatKey(movieID, seatID string) string {
	return movieID + ":" + seatID
}

// expired reports whether a hold has passed its TTL. Confirmed bookings never expire.
func expired(b Booking) bool {
	return b.Status == "held" && time.Now().After(b.ExpiresAt)
}

func (s *ConcurentStore) Book(b Booking) (Booking, error) {
	s.Lock()
	defer s.Unlock()

	key := seatKey(b.MovieID, b.SeatID)

	if existing, ok := s.bookings[key]; ok {
		if !expired(existing) {
			return Booking{}, ErrSeatAlreadyBooked
		}
		// stale hold: clean it up and let this booking take the seat
		delete(s.sessions, existing.ID)
		delete(s.bookings, key)
	}

	b.ID = uuid.New().String()
	b.Status = "held"
	b.ExpiresAt = time.Now().Add(defaultHoldTTL)

	s.bookings[key] = b
	s.sessions[b.ID] = key

	return b, nil
}

func (s *ConcurentStore) ListBookings(movieID string) []Booking {
	s.RLock()
	defer s.RUnlock()

	var result []Booking
	for _, b := range s.bookings {
		if b.MovieID == movieID && !expired(b) {
			result = append(result, b)
		}
	}
	return result
}

// getSession must be called with the write lock held.
func (s *ConcurentStore) getSession(sessionID, userID string) (Booking, string, error) {
	key, ok := s.sessions[sessionID]
	if !ok {
		return Booking{}, "", ErrSessionNotFound
	}

	b, ok := s.bookings[key]
	if !ok || expired(b) {
		delete(s.sessions, sessionID)
		delete(s.bookings, key)
		return Booking{}, "", ErrSessionNotFound
	}

	if b.UserID != userID {
		return Booking{}, "", ErrNotSessionOwner
	}

	return b, key, nil
}

func (s *ConcurentStore) Confirm(ctx context.Context, sessionID string, userID string) (Booking, error) {
	s.Lock()
	defer s.Unlock()

	b, key, err := s.getSession(sessionID, userID)
	if err != nil {
		return Booking{}, err
	}

	b.Status = "confirmed"
	b.ExpiresAt = time.Time{} // no expiry once confirmed
	s.bookings[key] = b

	return b, nil
}

func (s *ConcurentStore) Release(ctx context.Context, sessionID string, userID string) error {
	s.Lock()
	defer s.Unlock()

	_, key, err := s.getSession(sessionID, userID)
	if err != nil {
		return err
	}

	delete(s.bookings, key)
	delete(s.sessions, sessionID)
	return nil
}