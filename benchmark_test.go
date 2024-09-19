package sentinel

import (
	"log/slog"
	"strconv"
	"testing"
)

type Nested struct {
	Secret string `sentinel:"true"`
	Data   string
}

type Example struct {
	ID      int
	Name    string
	Nested  Nested
	Friends []string `sentinel:"true"`
}

func BenchmarkReplaceAttr(b *testing.B) {
	data := Example{
		ID:   123,
		Name: "Alice",
		Nested: Nested{
			Secret: "TopSecret",
			Data:   "PublicData",
		},
		Friends: []string{"Bob", "Charlie"},
	}

	attr := slog.Attr{
		Key:   "data",
		Value: slog.AnyValue(data),
	}

	groups := []string{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ReplaceAttr(groups, attr)
	}
}

func generateComplexData() Example {
	nested := Nested{
		Secret: "SuperSecret",
		Data:   "SomeData",
	}

	var friends []string
	for i := 0; i < 1000; i++ {
		friends = append(friends, "Friend"+strconv.Itoa(i))
	}

	return Example{
		ID:      999,
		Name:    "ComplexExample",
		Nested:  nested,
		Friends: friends,
	}
}

func BenchmarkReplaceAttrComplex(b *testing.B) {
	data := generateComplexData()

	attr := slog.Attr{
		Key:   "data",
		Value: slog.AnyValue(data),
	}

	groups := []string{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ReplaceAttr(groups, attr)
	}
}
