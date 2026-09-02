package postgres

import (
	"fmt"
	"strings"

	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/dbal/sqlite"
)

type Compiler struct{}

func (Compiler) QuoteIdentifier(value string) (string, error) {
	return (sqlite.Compiler{}).QuoteIdentifier(value)
}

func (Compiler) Placeholder(index int) string { return fmt.Sprintf("$%d", index) }

func (Compiler) CompileSelect(query dbal.Select) (string, []dbal.Value, error) {
	dateBucket := func(bucket, column string) (string, error) {
		switch bucket {
		case "day", "week", "month":
			return "date_trunc('" + bucket + "', " + column + ")", nil
		default:
			return "", fmt.Errorf("invalid date bucket %q", bucket)
		}
	}
	statement, arguments, err := (sqlite.Compiler{DateBucket: dateBucket}).CompileSelect(query)
	if err != nil {
		return "", nil, err
	}
	statement = strings.ReplaceAll(statement, " LIKE ", " ILIKE ")
	return numberPlaceholders(statement), arguments, nil
}

func numberPlaceholders(statement string) string {
	var output strings.Builder
	index := 1
	for _, character := range statement {
		if character == '?' {
			output.WriteString(fmt.Sprintf("$%d", index))
			index++
		} else {
			output.WriteRune(character)
		}
	}
	return output.String()
}
