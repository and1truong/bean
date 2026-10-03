//go:build !js

// Command beanwasm only compiles for js/wasm; this stub keeps native builds of
// ./... working.
package main

import "fmt"

func main() { fmt.Println("beanwasm is only available for GOOS=js GOARCH=wasm") }
