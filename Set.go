package main

type Set[T comparable] map[T]struct{}

func NewSet[T comparable]() Set[T] {
    return make(Set[T])
}

func (s Set[T]) Add(item T) {
    s[item] = struct{}{}
}

func (s Set[T]) Remove(item T) {
    delete(s, item)
}

func (s Set[T]) Contains(item T) bool {
    _, exists := s[item]
    return exists
}

func (s Set[T]) Size() int {
    return len(s)
}