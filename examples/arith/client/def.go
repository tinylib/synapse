package main

//go:generate msgp

type Num struct {
	Value float64 `msg:"val"`
}
