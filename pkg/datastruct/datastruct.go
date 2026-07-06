package datastruct

import (
	"sync"
)

type List struct {
	mu    sync.RWMutex
	items []string
}

func NewList() *List {
	return &List{
		items: make([]string, 0),
	}
}

func (l *List) LPush(values ...string) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, value := range values {
		l.items = append([]string{value}, l.items...)
	}
	return len(l.items)
}

func (l *List) RPush(values ...string) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.items = append(l.items, values...)
	return len(l.items)
}

func (l *List) LPop() (string, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.items) == 0 {
		return "", false
	}

	value := l.items[0]
	l.items = l.items[1:]
	return value, true
}

func (l *List) RPop() (string, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.items) == 0 {
		return "", false
	}

	value := l.items[len(l.items)-1]
	l.items = l.items[:len(l.items)-1]
	return value, true
}

func (l *List) LRange(start, stop int) []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	length := len(l.items)
	if length == 0 {
		return []string{}
	}

	if start < 0 {
		start = length + start
	}
	if stop < 0 {
		stop = length + stop
	}

	if start < 0 {
		start = 0
	}
	if stop >= length {
		stop = length - 1
	}
	if start > stop || start >= length {
		return []string{}
	}

	result := make([]string, stop-start+1)
	copy(result, l.items[start:stop+1])
	return result
}

func (l *List) LLen() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.items)
}

func (l *List) LIndex(index int) (string, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	length := len(l.items)
	if index < 0 {
		index = length + index
	}

	if index < 0 || index >= length {
		return "", false
	}

	return l.items[index], true
}

func (l *List) LSet(index int, value string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	length := len(l.items)
	if index < 0 {
		index = length + index
	}

	if index < 0 || index >= length {
		return false
	}

	l.items[index] = value
	return true
}

func (l *List) LRem(count int, value string) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	removed := 0
	newItems := make([]string, 0, len(l.items))

	if count == 0 {
		for _, item := range l.items {
			if item != value {
				newItems = append(newItems, item)
			} else {
				removed++
			}
		}
	} else if count > 0 {
		for _, item := range l.items {
			if item == value && removed < count {
				removed++
			} else {
				newItems = append(newItems, item)
			}
		}
	} else {
		count = -count
		for i := len(l.items) - 1; i >= 0; i-- {
			if l.items[i] == value && removed < count {
				removed++
			} else {
				newItems = append([]string{l.items[i]}, newItems...)
			}
		}
	}

	l.items = newItems
	return removed
}

type Set struct {
	mu    sync.RWMutex
	items map[string]struct{}
}

func NewSet() *Set {
	return &Set{
		items: make(map[string]struct{}),
	}
}

func (s *Set) SAdd(members ...string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	added := 0
	for _, member := range members {
		if _, exists := s.items[member]; !exists {
			s.items[member] = struct{}{}
			added++
		}
	}
	return added
}

func (s *Set) SRem(members ...string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	removed := 0
	for _, member := range members {
		if _, exists := s.items[member]; exists {
			delete(s.items, member)
			removed++
		}
	}
	return removed
}

func (s *Set) SIsMember(member string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, exists := s.items[member]
	return exists
}

func (s *Set) SMembers() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	members := make([]string, 0, len(s.items))
	for member := range s.items {
		members = append(members, member)
	}
	return members
}

func (s *Set) SCard() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}

func (s *Set) SPop() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.items) == 0 {
		return "", false
	}

	for member := range s.items {
		delete(s.items, member)
		return member, true
	}
	return "", false
}

func (s *Set) SInter(others ...*Set) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(others) == 0 {
		return s.SMembers()
	}

	result := make([]string, 0)
	for member := range s.items {
		inAll := true
		for _, other := range others {
			if !other.SIsMember(member) {
				inAll = false
				break
			}
		}
		if inAll {
			result = append(result, member)
		}
	}
	return result
}

func (s *Set) SUnion(others ...*Set) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	seen := make(map[string]struct{})

	for member := range s.items {
		seen[member] = struct{}{}
	}

	for _, other := range others {
		for _, member := range other.SMembers() {
			seen[member] = struct{}{}
		}
	}

	result := make([]string, 0, len(seen))
	for member := range seen {
		result = append(result, member)
	}
	return result
}

func (s *Set) SDiff(others ...*Set) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]string, 0)
	for member := range s.items {
		inOther := false
		for _, other := range others {
			if other.SIsMember(member) {
				inOther = true
				break
			}
		}
		if !inOther {
			result = append(result, member)
		}
	}
	return result
}

type Hash struct {
	mu     sync.RWMutex
	fields map[string]string
}

func NewHash() *Hash {
	return &Hash{
		fields: make(map[string]string),
	}
}

func (h *Hash) HSet(field, value string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	_, exists := h.fields[field]
	h.fields[field] = value
	return !exists
}

func (h *Hash) HGet(field string) (string, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	value, exists := h.fields[field]
	return value, exists
}

func (h *Hash) HDel(fields ...string) int {
	h.mu.Lock()
	defer h.mu.Unlock()

	deleted := 0
	for _, field := range fields {
		if _, exists := h.fields[field]; exists {
			delete(h.fields, field)
			deleted++
		}
	}
	return deleted
}

func (h *Hash) HExists(field string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	_, exists := h.fields[field]
	return exists
}

func (h *Hash) HGetAll() map[string]string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make(map[string]string, len(h.fields))
	for k, v := range h.fields {
		result[k] = v
	}
	return result
}

func (h *Hash) HKeys() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	keys := make([]string, 0, len(h.fields))
	for k := range h.fields {
		keys = append(keys, k)
	}
	return keys
}

func (h *Hash) HVals() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	values := make([]string, 0, len(h.fields))
	for _, v := range h.fields {
		values = append(values, v)
	}
	return values
}

func (h *Hash) HLen() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.fields)
}

func (h *Hash) HMSet(pairs map[string]string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for field, value := range pairs {
		h.fields[field] = value
	}
}

func (h *Hash) HMGet(fields ...string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	values := make([]string, len(fields))
	for i, field := range fields {
		if value, exists := h.fields[field]; exists {
			values[i] = value
		} else {
			values[i] = ""
		}
	}
	return values
}

func (h *Hash) HIncrBy(field string, increment int) (int, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	current := 0
	if value, exists := h.fields[field]; exists {
		var err error
		current, err = parseInt(value)
		if err != nil {
			return 0, err
		}
	}

	newValue := current + increment
	h.fields[field] = intToString(newValue)
	return newValue, nil
}

func parseInt(s string) (int, error) {
	if s == "" {
		return 0, nil
	}

	result := 0
	negative := false
	start := 0

	if len(s) > 0 && s[0] == '-' {
		negative = true
		start = 1
	} else if len(s) > 0 && s[0] == '+' {
		start = 1
	}

	for i := start; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, &IntConversionError{Value: s}
		}
		result = result*10 + int(s[i]-'0')
	}

	if negative {
		result = -result
	}
	return result, nil
}

func intToString(n int) string {
	if n == 0 {
		return "0"
	}

	negative := n < 0
	if negative {
		n = -n
	}

	digits := make([]byte, 0, 12)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}

	for i := 0; i < len(digits)/2; i++ {
		digits[i], digits[len(digits)-1-i] = digits[len(digits)-1-i], digits[i]
	}

	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}

type IntConversionError struct {
	Value string
}

func (e *IntConversionError) Error() string {
	return "hash value is not an integer"
}
