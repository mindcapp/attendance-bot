package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

type Student struct {
	Number int    `json:"number"`
	Name   string `json:"name"`
}

type Students struct {
	list     []Student
	byNumber map[int]Student
}

// LoadStudents reads the student list from a JSON file.
func LoadStudents(path string) (*Students, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("чтение %s: %w", path, err)
	}

	var list []Student
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("разбор %s: %w", path, err)
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("%s: список студентов пуст", path)
	}

	byNumber := make(map[int]Student, len(list))
	for _, s := range list {
		s.Name = strings.TrimSpace(s.Name)
		if s.Number <= 0 || s.Name == "" {
			return nil, fmt.Errorf("%s: некорректная запись %+v", path, s)
		}
		if _, dup := byNumber[s.Number]; dup {
			return nil, fmt.Errorf("%s: номер %d встречается дважды", path, s.Number)
		}
		byNumber[s.Number] = s
	}

	sort.Slice(list, func(i, j int) bool { return list[i].Number < list[j].Number })
	return &Students{list: list, byNumber: byNumber}, nil
}

// All returns students sorted by number.
func (s *Students) All() []Student {
	return s.list
}

func (s *Students) Get(number int) (Student, bool) {
	st, ok := s.byNumber[number]
	return st, ok
}
