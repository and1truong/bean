//go:build js && wasm

// Command beanwasm is the browser entry point for the Bean playground. It
// exposes a single synchronous global function, beanPlayground(requestJSON),
// that dispatches versioned bridge requests against an in-memory Session.
package main

import (
	"encoding/json"

	"syscall/js"

	"github.com/beanruntime/bean/internal/playground"
)

func main() {
	session := playground.NewSession()
	js.Global().Set("beanPlayground", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) != 1 || args[0].Type() != js.TypeString {
			result, _ := json.Marshal(playground.Response{V: playground.ProtocolVersion, OK: false, Error: &playground.Failure{Code: playground.CodeRequest, Message: "request must be a JSON string"}})
			return string(result)
		}
		var request playground.Request
		if err := json.Unmarshal([]byte(args[0].String()), &request); err != nil {
			result, _ := json.Marshal(playground.Response{V: playground.ProtocolVersion, OK: false, Error: &playground.Failure{Code: playground.CodeRequest, Message: "malformed request JSON"}})
			return string(result)
		}
		result, err := json.Marshal(session.Handle(request))
		if err != nil {
			fallback, _ := json.Marshal(playground.Response{V: playground.ProtocolVersion, OK: false, Error: &playground.Failure{Code: playground.CodeComposition, Message: "response encoding failed"}})
			return string(fallback)
		}
		return string(result)
	}))
	select {}
}
