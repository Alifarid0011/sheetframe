package sheetframe_test

import (
	"fmt"
	sf "github.com/Alifarid0011/sheetframe"
)

func ExampleFrame_Query() {
	orders, err := sf.New([]string{"team", "amount"}, [][]any{
		{"Design", 600}, {"Engineering", 900}, {"Design", 700}, {"Engineering", 100},
	})
	if err != nil {
		panic(err)
	}
	totals, err := orders.Query().
		Filter("amount", sf.Ge, 500).
		GroupBy("team").
		Aggregate(sf.Aggregation{Column: "amount", Op: sf.Sum, As: "total"}).
		Sort("total", true).
		Collect()
	if err != nil {
		panic(err)
	}
	fmt.Println(totals.Records())
	// Output: [[Design 1300] [Engineering 900]]
}
