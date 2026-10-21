package main

import "fmt"

func main() {
	n := -1
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("recovered: %v`n", r)
		}
	}()
	m := make(map[int]int, n)
	m[1] = 1
	fmt.Println("no panic, len", len(m))
}
