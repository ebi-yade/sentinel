package main

import (
	"log/slog"
	"os"

	"github.com/ebi-yade/sentinel"
)

// Example usage
type Nested struct {
	Secret string `sentinel:"true"`
	Data   string `json:"data"`
}

type Custom struct {
	Enum int
}

func (c Custom) LogValue() slog.Value {
	var v string
	switch c.Enum {
	case 1:
		v = "One"
	case 2:
		v = "Two"
	default:
		v = "Unknown"
	}
	return slog.GroupValue(slog.String("enum", v))
}

type Example struct {
	ID      int
	Name    string
	Nested  *Nested
	Custom  Custom
	Friends []string `sentinel:"true"`
}

func main() {

	data := Example{
		ID:   123,
		Name: "Alice",
		Custom: Custom{
			Enum: 2,
		},
		Nested: &Nested{
			Secret: "TopSecret",
			Data:   "PublicData",
		},
		Friends: []string{"Bob", "Charlie"},
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: sentinel.ReplaceAttr,
	})
	logger := slog.New(handler)
	logger.Info("Logging example", "data", data)
}
