package pubsub

import "sync"

type Message struct {
	Channel string
	Payload string
}

type PubSub struct {
	mu   sync.RWMutex
	subs map[string]map[chan Message]struct{}
}

func New() *PubSub {
	return &PubSub{subs: make(map[string]map[chan Message]struct{})}
}

func (p *PubSub) Subscribe(channel string) chan Message {
	ch := make(chan Message, 16)

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.subs[channel] == nil {
		p.subs[channel] = make(map[chan Message]struct{})
	}
	p.subs[channel][ch] = struct{}{}
	return ch
}

func (p *PubSub) Unsubscribe(channel string, ch chan Message) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if subs, ok := p.subs[channel]; ok {
		delete(subs, ch)
		if len(subs) == 0 {
			delete(p.subs, channel)
		}
	}
	close(ch)
}

func (p *PubSub) Publish(channel, payload string) int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	subs, ok := p.subs[channel]
	if !ok {
		return 0
	}

	delivered := 0
	for ch := range subs {
		select {
		case ch <- Message{Channel: channel, Payload: payload}:
			delivered++
		default:
		}
	}
	return delivered
}
