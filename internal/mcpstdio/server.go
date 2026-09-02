// Package mcpstdio adapts Bean Agent Protocol operations to MCP stdio.
package mcpstdio

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/beanruntime/bean/internal/agentprotocol"
)

const ProtocolVersion = "2026-07-28"

var legacyVersions = map[string]bool{
	"2024-11-05": true,
	"2025-03-26": true,
	"2025-06-18": true,
	"2025-11-25": true,
}

type Config struct {
	Service   *agentprotocol.Service
	Principal agentprotocol.Principal
	Version   string
}

type Server struct {
	config        Config
	legacyVersion string
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type callParams struct {
	Name           string                     `json:"name"`
	Arguments      json.RawMessage            `json:"arguments"`
	Meta           json.RawMessage            `json:"_meta"`
	InputResponses map[string]json.RawMessage `json:"inputResponses,omitempty"`
	RequestState   string                     `json:"requestState,omitempty"`
}

func New(config Config) *Server {
	if config.Service == nil {
		config.Service = agentprotocol.New()
	}
	if config.Version == "" {
		config.Version = "0.15.0-alpha"
	}
	return &Server{config: config}
}

// Serve processes one newline-delimited JSON-RPC message at a time and exits
// cleanly when the client closes input.
func (s *Server) Serve(ctx context.Context, input io.Reader, output io.Writer) error {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		result, reply := s.handle(ctx, line)
		if reply {
			if err := encoder.Encode(result); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

func (s *Server) handle(ctx context.Context, line []byte) (response, bool) {
	var document any
	if err := json.Unmarshal(line, &document); err != nil {
		return errorResponse(json.RawMessage("null"), -32700, "Parse error", nil), true
	}
	if _, object := document.(map[string]any); !object {
		return errorResponse(json.RawMessage("null"), -32600, "Invalid Request", nil), true
	}
	var req request
	_ = json.Unmarshal(line, &req)
	if req.JSONRPC != "2.0" || req.Method == "" || !validRequestID(req.ID) {
		id := req.ID
		if len(id) == 0 || !validRequestID(id) {
			id = json.RawMessage("null")
		}
		return errorResponse(id, -32600, "Invalid Request", nil), true
	}
	notification := len(req.ID) == 0
	if notification {
		if req.Method == "notifications/initialized" || req.Method == "notifications/cancelled" {
			return response{}, false
		}
		return response{}, false
	}

	if req.Method == "initialize" {
		return s.initialize(req), true
	}
	modern, versionErr := protocolVersion(req.Params)
	if req.Method == "server/discover" {
		if versionErr != nil || modern != ProtocolVersion {
			return errorResponse(req.ID, -32001, "Unsupported protocol version", map[string]any{"supported": []string{ProtocolVersion}}), true
		}
		return successResponse(req.ID, s.discover()), true
	}
	if s.legacyVersion == "" && (versionErr != nil || modern != ProtocolVersion) {
		return errorResponse(req.ID, -32001, "Unsupported protocol version", map[string]any{"supported": []string{ProtocolVersion}}), true
	}

	switch req.Method {
	case "ping":
		return successResponse(req.ID, map[string]any{}), true
	case "tools/list":
		return successResponse(req.ID, s.listTools(s.legacyVersion == "")), true
	case "tools/call":
		return s.callTool(ctx, req, s.legacyVersion == ""), true
	default:
		return errorResponse(req.ID, -32601, "Method not found", nil), true
	}
}

func (s *Server) initialize(req request) response {
	var params struct {
		ProtocolVersion string `json:"protocolVersion"`
		Capabilities    any    `json:"capabilities"`
		ClientInfo      any    `json:"clientInfo"`
		Meta            any    `json:"_meta"`
	}
	if err := decodeObject(req.Params, &params); err != nil {
		return errorResponse(req.ID, -32602, "Invalid params", nil)
	}
	if params.ProtocolVersion == "" || params.Capabilities == nil || params.ClientInfo == nil {
		return errorResponse(req.ID, -32602, "Invalid params", nil)
	}
	selected := params.ProtocolVersion
	if !legacyVersions[selected] {
		selected = "2025-11-25"
	}
	s.legacyVersion = selected
	return successResponse(req.ID, map[string]any{
		"protocolVersion": selected,
		"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
		"serverInfo":      map[string]any{"name": "bean", "version": s.config.Version},
		"instructions":    instructions(),
	})
}

func (s *Server) discover() map[string]any {
	return map[string]any{
		"resultType":        "complete",
		"supportedVersions": []string{ProtocolVersion},
		"capabilities":      map[string]any{"tools": map[string]any{"listChanged": false}},
		"_meta": map[string]any{
			"io.modelcontextprotocol/serverInfo": s.serverInfo(),
		},
		"instructions": instructions(),
		"ttlMs":        300000,
		"cacheScope":   "private",
	}
}

func (s *Server) listTools(modern bool) map[string]any {
	tools := make([]map[string]any, 0)
	for _, operation := range s.config.Service.Operations(s.config.Principal) {
		tools = append(tools, map[string]any{
			"name": operation.Name, "title": operation.Title,
			"description": operation.Description, "inputSchema": operation.InputSchema,
		})
	}
	result := map[string]any{"tools": tools}
	if modern {
		result["resultType"] = "complete"
		result["ttlMs"] = 300000
		result["cacheScope"] = "private"
		result["_meta"] = map[string]any{"io.modelcontextprotocol/serverInfo": s.serverInfo()}
	}
	return result
}

func (s *Server) callTool(ctx context.Context, req request, modern bool) response {
	var params callParams
	if err := decodeObject(req.Params, &params); err != nil || params.Name == "" {
		return errorResponse(req.ID, -32602, "Invalid params", nil)
	}
	allowed := false
	for _, operation := range s.config.Service.Operations(s.config.Principal) {
		if operation.Name == params.Name {
			allowed = true
			break
		}
	}
	if !allowed {
		return errorResponse(req.ID, -32602, "Unknown or unavailable tool", nil)
	}
	if len(bytes.TrimSpace(params.Arguments)) == 0 {
		params.Arguments = json.RawMessage(`{}`)
	}
	outcome := s.config.Service.Call(ctx, params.Name, params.Arguments, s.config.Principal)
	structured := any(outcome.Result)
	if !outcome.OK {
		structured = outcome
	}
	encoded, err := json.Marshal(structured)
	if err != nil {
		return errorResponse(req.ID, -32603, "Internal error", nil)
	}
	result := map[string]any{
		"content":           []map[string]any{{"type": "text", "text": string(encoded)}},
		"structuredContent": structured,
		"isError":           !outcome.OK,
	}
	if modern {
		result["resultType"] = "complete"
		result["_meta"] = map[string]any{"io.modelcontextprotocol/serverInfo": s.serverInfo()}
	}
	return successResponse(req.ID, result)
}

func protocolVersion(raw json.RawMessage) (string, error) {
	var params map[string]json.RawMessage
	if err := json.Unmarshal(raw, &params); err != nil {
		return "", err
	}
	var metadata map[string]json.RawMessage
	if err := json.Unmarshal(params["_meta"], &metadata); err != nil {
		return "", err
	}
	var version string
	if err := json.Unmarshal(metadata["io.modelcontextprotocol/protocolVersion"], &version); err != nil || version == "" {
		return "", fmt.Errorf("missing protocol version")
	}
	var capabilities map[string]any
	if err := json.Unmarshal(metadata["io.modelcontextprotocol/clientCapabilities"], &capabilities); err != nil || capabilities == nil {
		return "", fmt.Errorf("missing client capabilities")
	}
	return version, nil
}

func decodeObject(raw json.RawMessage, value any) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = json.RawMessage(`{}`)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode(value)
}

func validRequestID(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return true
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return false
	}
	switch value.(type) {
	case nil, string, float64:
		return true
	default:
		return false
	}
}

func successResponse(id json.RawMessage, result any) response {
	return response{JSONRPC: "2.0", ID: id, Result: result}
}

func errorResponse(id json.RawMessage, code int, message string, data any) response {
	return response{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: message, Data: data}}
}

func instructions() string {
	return "Discover Bean capabilities, validate and inspect the smallest definition, preview plan and diff, publish, then query only through Views and execute only through Actions."
}

func (s *Server) serverInfo() map[string]any {
	return map[string]any{"name": "bean", "version": s.config.Version}
}
