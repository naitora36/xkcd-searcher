package core

var SortByFrequency = sortByFrequency

func (s *Service) InternalIndex() map[string][]int {
	return s.index.Load().Index
}
