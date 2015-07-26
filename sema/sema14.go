// +build go1.4

package sema

type Point uint32

//go:noescape
func Wait(p *Point)

//go:noescape
func Wake(p *Point)
