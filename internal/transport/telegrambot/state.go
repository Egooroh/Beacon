package telegrambot

import "sync"

type step int

const (
	stepIdle step = iota
	stepAwaitingProjectName
)

type userState struct {
	step step
	lang string
}

type stateStore struct {
	mu   sync.Mutex
	data map[int64]*userState
}

func newStateStore() *stateStore {
	return &stateStore{data: make(map[int64]*userState)}
}

func (s *stateStore) get(chatID int64) *userState {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st, ok := s.data[chatID]; ok {
		return st
	}
	return &userState{step: stepIdle}
}

func (s *stateStore) set(chatID int64, st *userState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[chatID] = st
}

func (s *stateStore) clear(chatID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, chatID)
}
