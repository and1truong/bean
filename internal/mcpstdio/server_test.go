package mcpstdio

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/beanruntime/bean/internal/agentprotocol"
)

func TestModernDiscoveryListAndCall(t *testing.T) {
	metadata := `"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}`
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":"discover","method":"server/discover","params":{` + metadata + `}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{` + metadata + `}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"bean.definition.capabilities","arguments":{},` + metadata + `}}`,
	}, "\n") + "\n"
	var output bytes.Buffer
	server := New(Config{Principal: agentprotocol.Principal{Planes: map[agentprotocol.Plane]bool{agentprotocol.DefinitionPlane: true}}})
	if err := server.Serve(context.Background(), strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	responses := decodeLines(t, output.String())
	if len(responses) != 3 {
		t.Fatalf("responses=%s", output.String())
	}
	discover := resultMap(t, responses[0])
	if discover["resultType"] != "complete" {
		t.Fatalf("discover=%v", discover)
	}
	list := resultMap(t, responses[1])
	tools := list["tools"].([]any)
	if list["resultType"] != "complete" || len(tools) != 4 {
		t.Fatalf("list=%v", list)
	}
	call := resultMap(t, responses[2])
	if call["resultType"] != "complete" || call["isError"] != false || call["structuredContent"] == nil {
		t.Fatalf("call=%v", call)
	}
}

func TestLegacyInitializeCompatibility(t *testing.T) {
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
	}, "\n") + "\n"
	var output bytes.Buffer
	server := New(Config{Principal: agentprotocol.Principal{Planes: agentprotocol.AllPlanes()}})
	if err := server.Serve(context.Background(), strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	responses := decodeLines(t, output.String())
	if len(responses) != 2 {
		t.Fatalf("responses=%s", output.String())
	}
	initialized := resultMap(t, responses[0])
	if initialized["protocolVersion"] != "2025-06-18" {
		t.Fatalf("initialize=%v", initialized)
	}
	listed := resultMap(t, responses[1])
	if _, modern := listed["resultType"]; modern || len(listed["tools"].([]any)) != 10 {
		t.Fatalf("list=%v", listed)
	}
}

func TestModernAndLegacyPing(t *testing.T) {
	metadata := `"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}`
	modernInput := `{"jsonrpc":"2.0","id":1,"method":"ping","params":{` + metadata + `}}` + "\n"
	var output bytes.Buffer
	server := New(Config{Principal: agentprotocol.Principal{Planes: agentprotocol.AllPlanes()}})
	if err := server.Serve(context.Background(), strings.NewReader(modernInput), &output); err != nil {
		t.Fatal(err)
	}
	if result := resultMap(t, decodeLines(t, output.String())[0]); len(result) != 0 {
		t.Fatalf("modern ping result=%v", result)
	}

	legacyInput := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"ping","params":{}}`,
	}, "\n") + "\n"
	output.Reset()
	server = New(Config{Principal: agentprotocol.Principal{Planes: agentprotocol.AllPlanes()}})
	if err := server.Serve(context.Background(), strings.NewReader(legacyInput), &output); err != nil {
		t.Fatal(err)
	}
	responses := decodeLines(t, output.String())
	if len(responses) != 2 {
		t.Fatalf("responses=%s", output.String())
	}
	if result := resultMap(t, responses[1]); len(result) != 0 {
		t.Fatalf("legacy ping result=%v", result)
	}
}

func TestAuthorizationHidesUnavailableTools(t *testing.T) {
	metadata := `"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}`
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"bean.release.publish","arguments":{"file":"missing","target":"missing"},` + metadata + `}}` + "\n"
	var output bytes.Buffer
	server := New(Config{Principal: agentprotocol.Principal{Planes: map[agentprotocol.Plane]bool{agentprotocol.DefinitionPlane: true}}})
	if err := server.Serve(context.Background(), strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	responses := decodeLines(t, output.String())
	errorValue := responses[0]["error"].(map[string]any)
	if errorValue["code"] != float64(-32602) || strings.Contains(output.String(), "missing") {
		t.Fatalf("response=%s", output.String())
	}
}

func TestEachPlaneIsIndependentlyAuthorized(t *testing.T) {
	all := agentprotocol.Principal{Planes: agentprotocol.AllPlanes()}
	overrides := map[string]agentprotocol.Handler{}
	for _, item := range agentprotocol.New().Operations(all) {
		operation := item
		overrides[operation.Name] = func(context.Context, json.RawMessage, agentprotocol.Principal) agentprotocol.Outcome {
			return agentprotocol.Outcome{OK: true, Result: operation.Name}
		}
	}
	service, err := agentprotocol.NewWithHandlers(overrides)
	if err != nil {
		t.Fatal(err)
	}
	representatives := map[agentprotocol.Plane]string{
		agentprotocol.DefinitionPlane:  "bean.definition.capabilities",
		agentprotocol.ReleasePlane:     "bean.release.plan",
		agentprotocol.ApplicationPlane: "bean.application.query",
	}
	counts := map[agentprotocol.Plane]int{agentprotocol.DefinitionPlane: 4, agentprotocol.ReleasePlane: 4, agentprotocol.ApplicationPlane: 2}
	for plane, allowed := range representatives {
		principal := agentprotocol.Principal{Planes: map[agentprotocol.Plane]bool{plane: true}}
		server := New(Config{Service: service, Principal: principal})
		metadata := map[string]any{"io.modelcontextprotocol/protocolVersion": ProtocolVersion, "io.modelcontextprotocol/clientCapabilities": map[string]any{}}
		messages := []map[string]any{
			{"jsonrpc": "2.0", "id": 1, "method": "tools/list", "params": map[string]any{"_meta": metadata}},
			{"jsonrpc": "2.0", "id": 2, "method": "tools/call", "params": map[string]any{"name": allowed, "arguments": map[string]any{}, "_meta": metadata}},
		}
		denied := representatives[agentprotocol.DefinitionPlane]
		if plane == agentprotocol.DefinitionPlane {
			denied = representatives[agentprotocol.ReleasePlane]
		}
		messages = append(messages, map[string]any{"jsonrpc": "2.0", "id": 3, "method": "tools/call", "params": map[string]any{"name": denied, "arguments": map[string]any{"secret": "must-not-leak"}, "_meta": metadata}})
		var input, output bytes.Buffer
		for _, message := range messages {
			encoded, _ := json.Marshal(message)
			input.Write(encoded)
			input.WriteByte('\n')
		}
		if err := server.Serve(context.Background(), &input, &output); err != nil {
			t.Fatal(err)
		}
		responses := decodeLines(t, output.String())
		tools := resultMap(t, responses[0])["tools"].([]any)
		allowedResult := resultMap(t, responses[1])
		if len(tools) != counts[plane] || allowedResult["isError"] != false {
			t.Fatalf("plane=%s responses=%s", plane, output.String())
		}
		deniedError := responses[2]["error"].(map[string]any)
		if deniedError["code"] != float64(-32602) || strings.Contains(output.String(), "must-not-leak") {
			t.Fatalf("plane=%s responses=%s", plane, output.String())
		}
	}
}

func TestMalformedRequestAndEOFKeepStdoutClean(t *testing.T) {
	var output bytes.Buffer
	server := New(Config{Principal: agentprotocol.Principal{Planes: agentprotocol.AllPlanes()}})
	input := "not-json\n" + `{"jsonrpc":"2.0"}` + "\n"
	if err := server.Serve(context.Background(), strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	responses := decodeLines(t, output.String())
	if len(responses) != 2 || responses[0]["jsonrpc"] != "2.0" || responses[1]["id"] != nil {
		t.Fatalf("output=%q", output.String())
	}
	invalid := responses[1]["error"].(map[string]any)
	if invalid["code"] != float64(-32600) {
		t.Fatalf("output=%q", output.String())
	}
	output.Reset()
	if err := server.Serve(context.Background(), strings.NewReader(""), &output); err != nil || output.Len() != 0 {
		t.Fatalf("err=%v output=%q", err, output.String())
	}
}

func TestValidNonObjectAndMissingModernMetadataAreRejected(t *testing.T) {
	input := strings.Join([]string{
		`[]`,
		`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`,
		`{"jsonrpc":"2.0","id":true,"method":"tools/list","params":{}}`,
	}, "\n") + "\n"
	var output bytes.Buffer
	server := New(Config{Principal: agentprotocol.Principal{Planes: agentprotocol.AllPlanes()}})
	if err := server.Serve(context.Background(), strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	responses := decodeLines(t, output.String())
	first := responses[0]["error"].(map[string]any)
	second := responses[1]["error"].(map[string]any)
	third := responses[2]["error"].(map[string]any)
	if first["code"] != float64(-32600) || second["code"] != float64(-32001) || third["code"] != float64(-32600) || responses[2]["id"] != nil {
		t.Fatalf("responses=%s", output.String())
	}
}

func decodeLines(t *testing.T, value string) []map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(value), "\n")
	if strings.TrimSpace(value) == "" {
		return nil
	}
	out := make([]map[string]any, len(lines))
	for index, line := range lines {
		if err := json.Unmarshal([]byte(line), &out[index]); err != nil {
			t.Fatalf("non-JSON stdout line %q: %v", line, err)
		}
	}
	return out
}

func resultMap(t *testing.T, response map[string]any) map[string]any {
	t.Helper()
	result, ok := response["result"].(map[string]any)
	if !ok {
		t.Fatalf("response=%v", response)
	}
	return result
}
