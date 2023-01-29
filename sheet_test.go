package main

import "fmt"

func ExampleGenerateOne() {
	sg := NewSheetGenerator([]string{
		"a",
		"b",
		"c",
		"d",
		"e",
		"f",
		"g",
		"h",
	}, 12)

	fmt.Printf("%v\n", sg.GenerateOne(2))
	fmt.Printf("%v", sg.GenerateOne(2))

	// Output:
	// [[b d] [c g]]
	// [[g h] [a e]]
}
