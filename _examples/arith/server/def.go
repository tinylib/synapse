package main

//go:generate msgp -io=false

type Num struct {
	Value float64 `msg:"val"`
}
