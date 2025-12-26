package main

import "fmt"

type A struct {
	a string
}

func (f *A) h() *A {
	f.a = "hello"
	return f
}

type B struct {
	*A
}

func b() *B {
	return &B{
		&A{
			a: "",
		},
	}
}

func main() {
	b := b()
	fmt.Println(b.a)
}
