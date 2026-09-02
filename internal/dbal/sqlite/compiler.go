package sqlite

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/beanruntime/bean/internal/dbal"
)

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type Compiler struct {
	DateBucket func(string, string) (string, error)
}

func (Compiler) QuoteIdentifier(s string) (string, error) {
	parts := strings.Split(s, ".")
	for i, p := range parts {
		if !identifier.MatchString(p) {
			return "", fmt.Errorf("invalid identifier %q", s)
		}
		parts[i] = `"` + p + `"`
	}
	return strings.Join(parts, "."), nil
}
func (Compiler) Placeholder(int) string { return "?" }

func (c Compiler) groupExpression(group dbal.Group) (string, error) {
	column, err := c.QuoteIdentifier(group.Column)
	if err != nil {
		return "", err
	}
	if group.Bucket == "" {
		return column, nil
	}
	if c.DateBucket != nil {
		return c.DateBucket(group.Bucket, column)
	}
	switch group.Bucket {
	case "day":
		return "date(" + column + ")", nil
	case "week":
		return "date(" + column + ", '-' || ((strftime('%w', " + column + ") + 6) % 7) || ' days')", nil
	case "month":
		return "strftime('%Y-%m-01', " + column + ")", nil
	default:
		return "", fmt.Errorf("invalid date bucket %q", group.Bucket)
	}
}

func (c Compiler) CompileSelect(q dbal.Select) (string, []dbal.Value, error) {
	table, err := c.QuoteIdentifier(q.Table)
	if err != nil {
		return "", nil, err
	}
	cols := make([]string, 0, len(q.Columns)+len(q.Aggregates))
	for _, col := range q.Columns {
		x, e := c.QuoteIdentifier(col)
		if e != nil {
			return "", nil, e
		}
		if strings.Contains(col, ".") {
			alias, aliasErr := c.QuoteIdentifier(strings.ReplaceAll(col, ".", "__"))
			if aliasErr != nil {
				return "", nil, aliasErr
			}
			x += " AS " + alias
		}
		cols = append(cols, x)
	}
	for _, a := range q.Aggregates {
		fn := strings.ToUpper(a.Function)
		if fn != "COUNT" && fn != "SUM" && fn != "MIN" && fn != "MAX" && fn != "AVG" {
			return "", nil, fmt.Errorf("invalid aggregate %q", a.Function)
		}
		col, e := c.QuoteIdentifier(a.Column)
		if e != nil {
			return "", nil, e
		}
		alias, e := c.QuoteIdentifier(a.Alias)
		if e != nil {
			return "", nil, e
		}
		cols = append(cols, fn+"("+col+") AS "+alias)
	}
	for _, group := range q.GroupBy {
		expression, e := c.groupExpression(group)
		if e != nil {
			return "", nil, e
		}
		alias, e := c.QuoteIdentifier(group.Alias)
		if e != nil {
			return "", nil, e
		}
		cols = append(cols, expression+" AS "+alias)
	}
	if len(cols) == 0 {
		cols = []string{"*"}
	}
	b := strings.Builder{}
	b.WriteString("SELECT ")
	b.WriteString(strings.Join(cols, ", "))
	b.WriteString(" FROM ")
	b.WriteString(table)
	for _, j := range q.Joins {
		jt := strings.ToUpper(j.Type)
		if jt == "" {
			jt = "INNER"
		}
		if jt != "INNER" && jt != "LEFT" {
			return "", nil, fmt.Errorf("invalid join type")
		}
		t, e := c.QuoteIdentifier(j.Table)
		if e != nil {
			return "", nil, e
		}
		l, e := c.QuoteIdentifier(j.Left)
		if e != nil {
			return "", nil, e
		}
		r, e := c.QuoteIdentifier(j.Right)
		if e != nil {
			return "", nil, e
		}
		b.WriteString(" " + jt + " JOIN " + t)
		if j.Alias != "" {
			a, e := c.QuoteIdentifier(j.Alias)
			if e != nil {
				return "", nil, e
			}
			b.WriteString(" AS " + a)
		}
		b.WriteString(" ON " + l + " = " + r)
	}
	args := []dbal.Value{}
	if q.Where != nil {
		w, e := c.predicate(*q.Where, &args)
		if e != nil {
			return "", nil, e
		}
		b.WriteString(" WHERE " + w)
	}
	if len(q.GroupBy) > 0 {
		xs := []string{}
		for _, g := range q.GroupBy {
			x, e := c.groupExpression(g)
			if e != nil {
				return "", nil, e
			}
			xs = append(xs, x)
		}
		b.WriteString(" GROUP BY " + strings.Join(xs, ", "))
	}
	if len(q.OrderBy) > 0 {
		xs := []string{}
		for _, o := range q.OrderBy {
			x, e := c.QuoteIdentifier(o.Column)
			if e != nil {
				return "", nil, e
			}
			if o.Desc {
				x += " DESC"
			} else {
				x += " ASC"
			}
			if o.NullsLast {
				x += " NULLS LAST"
			}
			xs = append(xs, x)
		}
		b.WriteString(" ORDER BY " + strings.Join(xs, ", "))
	}
	if q.Limit > 0 {
		b.WriteString(" LIMIT ?")
		args = append(args, q.Limit)
		if q.Offset > 0 {
			b.WriteString(" OFFSET ?")
			args = append(args, q.Offset)
		}
	}
	return b.String(), args, nil
}

func (c Compiler) predicate(p dbal.Predicate, args *[]dbal.Value) (string, error) {
	if p.Op == "and" || p.Op == "or" {
		if len(p.Children) == 0 {
			return "", fmt.Errorf("empty %s", p.Op)
		}
		xs := []string{}
		for _, ch := range p.Children {
			x, e := c.predicate(ch, args)
			if e != nil {
				return "", e
			}
			xs = append(xs, "("+x+")")
		}
		return strings.Join(xs, " "+strings.ToUpper(string(p.Op))+" "), nil
	}
	if p.Op == "not" {
		if len(p.Children) != 1 {
			return "", fmt.Errorf("not needs one child")
		}
		x, e := c.predicate(p.Children[0], args)
		return "NOT (" + x + ")", e
	}
	col, e := c.QuoteIdentifier(p.Column)
	if e != nil {
		return "", e
	}
	switch p.Op {
	case dbal.OpEQ, dbal.OpNE, dbal.OpGT, dbal.OpGTE, dbal.OpLT, dbal.OpLTE:
		op := map[dbal.Operator]string{dbal.OpEQ: "=", dbal.OpNE: "!=", dbal.OpGT: ">", dbal.OpGTE: ">=", dbal.OpLT: "<", dbal.OpLTE: "<="}[p.Op]
		*args = append(*args, p.Value)
		return col + " " + op + " ?", nil
	case dbal.OpIsNull:
		return col + " IS NULL", nil
	case dbal.OpIsNotNull:
		return col + " IS NOT NULL", nil
	case dbal.OpContains, dbal.OpStartsWith, dbal.OpEndsWith:
		v := fmt.Sprint(p.Value)
		if p.Op == dbal.OpContains {
			v = "%" + v + "%"
		} else if p.Op == dbal.OpStartsWith {
			v = v + "%"
		} else {
			v = "%" + v
		}
		*args = append(*args, v)
		return col + " LIKE ? ESCAPE '\\'", nil
	case dbal.OpIn, dbal.OpNotIn:
		vs, ok := p.Value.([]dbal.Value)
		if !ok || len(vs) == 0 {
			return "", fmt.Errorf("in requires values")
		}
		marks := make([]string, len(vs))
		for i, v := range vs {
			marks[i] = "?"
			*args = append(*args, v)
		}
		op := "IN"
		if p.Op == dbal.OpNotIn {
			op = "NOT IN"
		}
		return col + " " + op + " (" + strings.Join(marks, ",") + ")", nil
	default:
		return "", fmt.Errorf("invalid operator %q", p.Op)
	}
}
