package words

import (
	"context"
	"strings"
	"unicode"

	"github.com/kljensen/snowball/english"
)

func NormalizePhrase(ctx context.Context, str string) ([]string, error) {
	var res []string
	seen := make(map[string]struct{})

	words := strings.FieldsFunc(str, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	for i, word := range words {
		// Будем проверять контекст каждые 10 итераций
		if i%10 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}

		lowerWord := strings.ToLower(word)
		if english.IsStopWord(lowerWord) {
			continue
		}
		stemmed := english.Stem(lowerWord, true)

		if _, ok := seen[stemmed]; !ok {
			seen[stemmed] = struct{}{}
			res = append(res, stemmed)
		}
	}
	return res, nil
}
