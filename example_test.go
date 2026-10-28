package sheetframe_test

import (
	"fmt"
	sf "github.com/Alifarid0011/sheetframe"
)

func ExampleFrame_Filter() {
	frame, err := sf.New([]string{"region", "revenue"}, [][]any{
		{"North", 1200}, {"South", 900}, {"North", nil},
	})
	if err != nil {
		panic(err)
	}
	result, err := frame.Filter("revenue", sf.Ge, 1000)
	if err != nil {
		panic(err)
	}
	fmt.Println(result.Records())
	// Output: [[North 1200]]
}

func ExampleGrouped_Aggregate() {
	frame, err := sf.New([]string{"region", "revenue"}, [][]any{
		{"North", 1200}, {"South", 900}, {"North", 800},
	})
	if err != nil {
		panic(err)
	}
	groups, err := frame.GroupBy("region")
	if err != nil {
		panic(err)
	}
	result, err := groups.Aggregate(sf.Aggregation{
		Column: "revenue", Op: sf.Sum, As: "total",
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(result.Columns())
	fmt.Println(result.Records())
	// Output:
	// [region total]
	// [[North 2000] [South 900]]
}
