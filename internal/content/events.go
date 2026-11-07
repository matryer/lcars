package content

// Subscribe registers before returning the current revision. A slow subscriber
// retains only the latest revision, since clients fetch complete state.
func (s *Store) Subscribe() (<-chan string, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.subscribers == nil {
		s.subscribers = map[chan string]struct{}{}
	}
	ch := make(chan string, 1)
	s.subscribers[ch] = struct{}{}
	ch <- s.state.Revision
	return ch, func() { s.mu.Lock(); delete(s.subscribers, ch); s.mu.Unlock() }
}
func (s *Store) notify() {
	for ch := range s.subscribers {
		select {
		case <-ch:
		default:
		}
		ch <- s.state.Revision
	}
}
