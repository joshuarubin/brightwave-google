package text

import (
	"bytes"
	"io"
	"unicode"

	"github.com/aaaton/golem/v4"
	"github.com/aaaton/golem/v4/dicts/en"
	"github.com/jdkato/prose/v2"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	"golang.org/x/text/unicode/rangetable"
)

func Normalize(data []byte) ([]byte, error) {
	t := transform.Chain(
		norm.NFD,
		runes.Remove(runes.In(rangetable.Merge(
			unicode.Cs, // remove surrogate
			unicode.Pe, // remove punctuation, close
			unicode.Pf, // remove punctuation, final
			unicode.Pi, // remove punctuation, initial
			unicode.Po, // remove punctuation, other
			unicode.Ps, // remove punctuation, open
			unicode.Mn, // remove accents
		))),
		norm.NFC,
		cases.Lower(language.English),
	)
	r := transform.NewReader(bytes.NewReader(data), t)
	return io.ReadAll(r)
}

func Tokenize(data []byte) ([]string, error) {
	doc, err := prose.NewDocument(
		string(data),
		prose.WithSegmentation(false),
		prose.WithExtraction(false),
	)
	if err != nil {
		return nil, err
	}

	lemmatizer, err := golem.New(en.New())
	if err != nil {
		return nil, err
	}

	tokens := map[string]struct{}{}

	for _, tok := range doc.Tokens() {
		switch tok.Tag {
		case "DT", "CC", "IN", "TO":
			// ignore:
			// - determiner
			// - conjunction, coordinating
			// - conjunction, subordinating or preposition
			// - infinitival to
		default:
			word := lemmatizer.Lemma(tok.Text)
			tokens[word] = struct{}{}
		}
	}

	ret := make([]string, 0, len(tokens))
	for t := range tokens {
		ret = append(ret, t)
	}

	return ret, nil
}
