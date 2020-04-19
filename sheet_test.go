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

	fmt.Printf("%v\n", sg.GenerateOne())
	fmt.Printf("%v", sg.GenerateOne())

	// Output:
	// [[b d] [c g]]
	// [[g h] [a e]]
}
