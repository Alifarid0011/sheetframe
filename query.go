package sheetframe

// Query chains eager frame operations while retaining the first error. Each
// method returns a new value, so a query can be reused for independent branches.
// Its zero value starts with an empty frame. Call Collect to obtain the result.
// Callbacks run immediately; they are skipped after an earlier operation fails.
type Query struct {
	frame Frame
	err   error
}

// Query starts an optional fluent pipeline over f without changing f.
func (f Frame) Query() Query { return Query{frame: f} }

// Collect returns the result, or an empty frame and the first operation error.
func (q Query) Collect() (Frame, error) {
	if q.err != nil {
		return Frame{}, q.err
	}
	return q.frame, nil
}

// Select projects columns in the requested order.
func (q Query) Select(names ...string) Query {
	if q.err == nil {
		q.frame, q.err = q.frame.Select(names...)
	}
	return q
}

// Filter selects rows using the same typed comparisons as Frame.Filter.
func (q Query) Filter(column string, op Op, value any) Query {
	if q.err == nil {
		q.frame, q.err = q.frame.Filter(column, op, value)
	}
	return q
}

// Sort stably orders rows with nulls last in either direction.
func (q Query) Sort(column string, descending bool) Query {
	if q.err == nil {
		q.frame, q.err = q.frame.Sort(column, descending)
	}
	return q
}

// Head keeps up to n leading rows. Negative n keeps no rows.
func (q Query) Head(n int) Query {
	if q.err == nil {
		q.frame = q.frame.Head(n)
	}
	return q
}

// WithColumn computes or replaces a column, unless an earlier operation failed.
func (q Query) WithColumn(name string, fn func(Row) (any, error)) Query {
	if q.err == nil {
		q.frame, q.err = q.frame.WithColumn(name, fn)
	}
	return q
}

// Join combines the current result with right using Frame.Join semantics.
func (q Query) Join(right Frame, on []string, how JoinKind) Query {
	if q.err == nil {
		q.frame, q.err = q.frame.Join(right, on, how)
	}
	return q
}

// GroupedQuery holds grouping keys and the first pipeline error. Aggregate
// continues the pipeline with one row per group. Its zero value has no keys,
// so Aggregate returns a query carrying an error.
type GroupedQuery struct {
	grouped Grouped
	err     error
}

// GroupBy prepares grouping keys, retaining any earlier pipeline error.
func (q Query) GroupBy(keys ...string) GroupedQuery {
	if q.err != nil {
		return GroupedQuery{err: q.err}
	}
	grouped, err := q.frame.GroupBy(keys...)
	return GroupedQuery{grouped: grouped, err: err}
}

// Aggregate summarizes groups using Grouped.Aggregate semantics.
func (g GroupedQuery) Aggregate(aggs ...Aggregation) Query {
	if g.err != nil {
		return Query{err: g.err}
	}
	frame, err := g.grouped.Aggregate(aggs...)
	return Query{frame: frame, err: err}
}
