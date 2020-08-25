package main

import "fmt"

// Windows doesn't handle the block-style very well
var (
	simpleProgressStyle = "double-"
	r, l                = "[", "]"
)

func color(content ...interface{}) string {
	return fmt.Sprint(content...)
}
