package home

import (
	"path/filepath"
	"sort"
)

// MemHome is an in-memory Home for testing.
type MemHome struct {
	Files map[string][]byte
}

// NewMem returns a MemHome with an empty file map.
func NewMem() *MemHome {
	return &MemHome{Files: make(map[string][]byte)}
}

func (m *MemHome) Get(name string) ([]byte, error) {
	data, ok := m.Files[name]
	if !ok {
		return nil, ErrNotFound
	}
	return data, nil
}

func (m *MemHome) Search(pattern string) ([]string, error) {
	var matches []string
	for name := range m.Files {
		ok, err := filepath.Match(pattern, name)
		if err != nil {
			return nil, err
		}
		if ok {
			matches = append(matches, name)
		}
	}
	sort.Strings(matches)
	return matches, nil
}

func (m *MemHome) Upsert(name string, data []byte) error {
	m.Files[name] = data
	return nil
}

func (m *MemHome) Delete(name string) error {
	if _, ok := m.Files[name]; !ok {
		return ErrNotFound
	}
	delete(m.Files, name)
	return nil
}
