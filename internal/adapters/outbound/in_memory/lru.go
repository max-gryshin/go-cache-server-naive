package in_memory

import "container/list"

type lruList struct {
	list  *list.List
	items map[string]*list.Element
}

func newLRUList() *lruList {
	return &lruList{
		list:  list.New(),
		items: make(map[string]*list.Element),
	}
}

func (l *lruList) add(key string) {
	if el, ok := l.items[key]; ok {
		l.list.MoveToFront(el)
		return
	}
	el := l.list.PushFront(key)
	l.items[key] = el
}

func (l *lruList) touch(key string) {
	if el, ok := l.items[key]; ok {
		l.list.MoveToFront(el)
	}
}

func (l *lruList) remove(key string) {
	if el, ok := l.items[key]; ok {
		l.list.Remove(el)
		delete(l.items, key)
	}
}

// evict removes the least recently used element and returns its key.
// Returns empty string if the list is empty.
func (l *lruList) evict() string {
	back := l.list.Back()
	if back == nil {
		return ""
	}
	key := back.Value.(string)
	l.list.Remove(back)
	delete(l.items, key)
	return key
}
