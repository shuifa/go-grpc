package service

import (
	"sync"
)

type RatingStore interface {
	Add(laptopID string, score float64) (*Rating, error)
}

type Rating struct {
	Count uint64
	Sum   float64
}

type InmemoryRatingStore struct {
	mutex   sync.RWMutex
	ratings map[string]*Rating
}

func (store *InmemoryRatingStore) Add(laptopID string, score float64) (*Rating, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	laptopRating := store.ratings[laptopID]

	if laptopRating == nil {
		laptopRating = &Rating{
			Count: 1,
			Sum:   score,
		}
	} else {
		laptopRating.Count++
		laptopRating.Sum += score
	}
	store.ratings[laptopID] = laptopRating

	return laptopRating, nil
}

func NewInmemoryRatingStore() *InmemoryRatingStore {
	return &InmemoryRatingStore{
		ratings: make(map[string]*Rating),
	}
}
