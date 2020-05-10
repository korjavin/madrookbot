package main

import (
	"log"
	"math"
	"math/rand"
	"time"
)

type SheetGenerator struct {
	words []string
	size  int
}

func NewSheetGenerator(words []string, seed int64) *SheetGenerator {
	rand.Seed(seed)
	if seed == 0 {
		rand.Seed(time.Now().Unix())
	}
	x := int(math.Sqrt(float64(len(words))))
	return &SheetGenerator{
		words: words,
		size:  x,
	}
}
func (s *SheetGenerator) AddWord(str string) {
	if str == "" {
		return
	}
	s.words = append(s.words, str)
	s.size = int(math.Sqrt(float64(len(s.words))))
	log.Printf("sg len=%d size=%d", len(s.words), s.size)
}

func (s *SheetGenerator) GenerateOne(size int) [][]string {
	used := make(map[int]struct{})
	if size == 0 {
		size = s.size
	}
	res := make([][]string, size)
	for i := 0; i < size; i++ {
		row := make([]string, size)
		for j := 0; j < size; j++ {
			var idx int
			for {
				idx = rand.Intn(len(s.words))
				if _, ok := used[idx]; !ok {
					break
				}
				if len(used) >= len(s.words) {
					break
				}
			}
			used[idx] = struct{}{}
			row[j] = s.words[idx]
		}
		res[i] = row
	}
	return res
}
