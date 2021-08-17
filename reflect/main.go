package main

import "fmt"

type t1 struct {
	Name string
}
type t2 struct {
	Age int
}
func main() {
	//input :=make([]interface{},0)
	var input []interface{}
	input = append(input, 1)
	input = append(input, "1")
	input = append(input, 1.1)
	input = append(input, true)
	input = append(input, t1{Name: "yds"})
	input = append(input, t2{Age: 20})
	parse(input...)
}
func parse(item... interface{} )  {
	for _, in := range item {
		switch inType := in.(type) {
		case string:
			fmt.Println("string:",inType)
		case int:
			fmt.Println("int:",inType)
		case float64:
			fmt.Println("float64:",inType)
		case bool:
			fmt.Println("bool:",inType)
		case t1:
			fmt.Println("t1:",inType)
		default:
			fmt.Printf("unknown:%T \n",inType)
		}
	}
}