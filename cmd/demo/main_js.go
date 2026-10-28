//go:build js && wasm

package main

import (
	"github.com/Alifarid0011/sheetframe/internal/playground"
	"syscall/js"
)

func main() {
	engine := &playground.Engine{}
	run := js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) != 1 || args[0].Type() != js.TypeString {
			return `{"error":"expected one JSON request string"}`
		}
		return engine.RunJSON(args[0].String())
	})
	js.Global().Set("sheetframeRun", run)
	select {}
}
