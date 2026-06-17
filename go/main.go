package main

/*
#include <stdint.h>
#include <stdlib.h>

typedef struct {
	void* ptr;
	size_t len;
} cliproxy_buffer;

typedef int (*cliproxy_plugin_call_fn)(char*, uint8_t*, size_t, cliproxy_buffer*);
typedef void (*cliproxy_plugin_free_fn)(void*, size_t);
typedef void (*cliproxy_plugin_shutdown_fn)(void);

typedef struct {
	uint32_t abi_version;
	void* host_ctx;
	void* call;
	void* free_buffer;
} cliproxy_host_api;

typedef struct {
	uint32_t abi_version;
	cliproxy_plugin_call_fn call;
	cliproxy_plugin_free_fn free_buffer;
	cliproxy_plugin_shutdown_fn shutdown;
} cliproxy_plugin_api;

extern int cliproxyPluginCall(char*, uint8_t*, size_t, cliproxy_buffer*);
extern void cliproxyPluginFree(void*, size_t);
extern void cliproxyPluginShutdown(void);
*/
import "C"

import (
	"encoding/json"
	"strings"
	"sync/atomic"
	"unsafe"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginabi"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
	"gopkg.in/yaml.v3"
)

const pluginIdentifier = "codex-force-websearch"

var currentConfig atomic.Value

type envelope struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *envelopeError  `json:"error,omitempty"`
}

type envelopeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type lifecycleRequest struct {
	ConfigYAML []byte `json:"config_yaml"`
}

type pluginConfig struct {
	Enabled                  bool     `yaml:"enabled"`
	ForceAllRequests         bool     `yaml:"force_all_requests"`
	RequireCodexFormat       bool     `yaml:"require_codex_format"`
	TargetFormats            []string `yaml:"target_formats"`
	TargetModels             []string `yaml:"target_models"`
	InjectBeforeAuth         bool     `yaml:"inject_before_auth"`
	InjectAfterAuth          bool     `yaml:"inject_after_auth"`
	AllowChatCompletions     bool     `yaml:"allow_chat_completions"`
	ForceOverwriteTool       bool     `yaml:"force_overwrite_tool"`
	ToolChoiceRequired       bool     `yaml:"tool_choice_required"`
	SetParallelToolCalls     bool     `yaml:"set_parallel_tool_calls"`
	MaxToolCalls             int      `yaml:"max_tool_calls"`
	SearchContextSize        string   `yaml:"search_context_size"`
	ReturnTokenBudget        string   `yaml:"return_token_budget"`
	DisableLiveWebAccess     bool     `yaml:"disable_live_web_access"`
	AllowedDomains           []string `yaml:"allowed_domains"`
	BlockedDomains           []string `yaml:"blocked_domains"`
	IncludeActionSources     bool     `yaml:"include_action_sources"`
	IncludeRawResults        bool     `yaml:"include_raw_results"`
	EnableImageSearch        bool     `yaml:"enable_image_search"`
	ImageMaxResults          int      `yaml:"image_max_results"`
	ImageCaptions            bool     `yaml:"image_captions"`
	AddInstruction           bool     `yaml:"add_instruction"`
	Instruction              string   `yaml:"instruction"`
	ChatSearchModel          string   `yaml:"chat_search_model"`
	RewriteChatToSearchModel bool     `yaml:"rewrite_chat_to_search_model"`
}

type registration struct {
	SchemaVersion uint32                 `json:"schema_version"`
	Metadata      pluginapi.Metadata     `json:"metadata"`
	Capabilities  registrationCapability `json:"capabilities"`
}

type registrationCapability struct {
	RequestInterceptor bool `json:"request_interceptor"`
}

func main() {}

//export cliproxy_plugin_init
func cliproxy_plugin_init(_ *C.cliproxy_host_api, plugin *C.cliproxy_plugin_api) C.int {
	if plugin == nil {
		return 1
	}
	plugin.abi_version = C.uint32_t(pluginabi.ABIVersion)
	plugin.call = C.cliproxy_plugin_call_fn(C.cliproxyPluginCall)
	plugin.free_buffer = C.cliproxy_plugin_free_fn(C.cliproxyPluginFree)
	plugin.shutdown = C.cliproxy_plugin_shutdown_fn(C.cliproxyPluginShutdown)
	return 0
}

//export cliproxyPluginCall
func cliproxyPluginCall(method *C.char, request *C.uint8_t, requestLen C.size_t, response *C.cliproxy_buffer) C.int {
	if response != nil {
		response.ptr = nil
		response.len = 0
	}
	if method == nil {
		writeResponse(response, errorEnvelope("invalid_method", "method is required"))
		return 1
	}

	var requestBytes []byte
	if request != nil && requestLen > 0 {
		requestBytes = C.GoBytes(unsafe.Pointer(request), C.int(requestLen))
	}

	raw, err := handleMethod(C.GoString(method), requestBytes)
	if err != nil {
		writeResponse(response, errorEnvelope("plugin_error", err.Error()))
		return 1
	}
	writeResponse(response, raw)
	return 0
}

//export cliproxyPluginFree
func cliproxyPluginFree(ptr unsafe.Pointer, _ C.size_t) {
	if ptr != nil {
		C.free(ptr)
	}
}

//export cliproxyPluginShutdown
func cliproxyPluginShutdown() {}

func handleMethod(method string, request []byte) ([]byte, error) {
	switch method {
	case pluginabi.MethodPluginRegister, pluginabi.MethodPluginReconfigure:
		if err := configure(request); err != nil {
			return nil, err
		}
		return okEnvelope(pluginRegistration())
	case pluginabi.MethodRequestInterceptBefore:
		return interceptRequest(request, "before")
	case pluginabi.MethodRequestInterceptAfter:
		return interceptRequest(request, "after")
	default:
		return errorEnvelope("unknown_method", "unknown method: "+method), nil
	}
}

func defaultPluginConfig() pluginConfig {
	return pluginConfig{
		Enabled:              true,
		RequireCodexFormat:   true,
		TargetFormats:        []string{"codex"},
		TargetModels:         []string{"gpt-5*", "gpt-4.1*", "o3*", "o4*"},
		InjectBeforeAuth:     false,
		InjectAfterAuth:      true,
		AllowChatCompletions: false,
		ToolChoiceRequired:   true,
		SetParallelToolCalls: true,
		MaxToolCalls:         4,
		SearchContextSize:    "medium",
		IncludeActionSources: true,
		AddInstruction:       true,
		ChatSearchModel:      "gpt-5-search-api",
		Instruction:          "For any question that may require current, external, API, package, repository, changelog, security, pricing, policy, or source-grounded information, use the hosted web_search tool before answering. Prefer official documentation and primary sources. Include citations when the response format supports them.",
	}
}

func configure(raw []byte) error {
	var req lifecycleRequest
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &req); err != nil {
			return err
		}
	}
	cfg := defaultPluginConfig()
	if len(req.ConfigYAML) > 0 {
		if err := yaml.Unmarshal(req.ConfigYAML, &cfg); err != nil {
			return err
		}
	}
	cfg.SearchContextSize = normalizeOneOf(cfg.SearchContextSize, []string{"low", "medium", "high"})
	cfg.ReturnTokenBudget = normalizeOneOf(cfg.ReturnTokenBudget, []string{"default", "unlimited"})
	cfg.Instruction = strings.TrimSpace(cfg.Instruction)
	if cfg.Instruction == "" {
		cfg.Instruction = defaultPluginConfig().Instruction
	}
	if strings.TrimSpace(cfg.ChatSearchModel) == "" {
		cfg.ChatSearchModel = defaultPluginConfig().ChatSearchModel
	}
	currentConfig.Store(cfg)
	return nil
}

func loadedConfig() pluginConfig {
	raw := currentConfig.Load()
	if cfg, ok := raw.(pluginConfig); ok {
		return cfg
	}
	return defaultPluginConfig()
}

func pluginRegistration() registration {
	return registration{
		SchemaVersion: pluginabi.SchemaVersion,
		Metadata: pluginapi.Metadata{
			Name:             pluginIdentifier,
			Version:          "0.1.0",
			Author:           "local",
			GitHubRepository: "https://github.com/router-for-me/CLIProxyAPI",
			ConfigFields: []pluginapi.ConfigField{
				{Name: "enabled", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Enable or disable this plugin."},
				{Name: "force_all_requests", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Inject web search into every intercepted request. Dangerous for non-Codex providers."},
				{Name: "require_codex_format", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Only mutate requests whose SourceFormat or ToFormat is codex, unless force_all_requests is true."},
				{Name: "target_formats", Type: pluginapi.ConfigFieldTypeArray, Description: "Format patterns to match. Default: codex."},
				{Name: "target_models", Type: pluginapi.ConfigFieldTypeArray, Description: "Wildcard model patterns to match when require_codex_format is false or a Codex format is detected."},
				{Name: "inject_before_auth", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Rewrite before credential selection. Usually false."},
				{Name: "inject_after_auth", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Rewrite after credential selection. Usually true for Codex provider payloads."},
				{Name: "allow_chat_completions", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Also mutate Chat Completions-shaped bodies by setting web_search_options. Codex CLI should normally use Responses."},
				{Name: "force_overwrite_tool", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Replace an existing web_search tool definition instead of preserving user-supplied options."},
				{Name: "tool_choice_required", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Set tool_choice=required on Responses bodies."},
				{Name: "set_parallel_tool_calls", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Set parallel_tool_calls=true."},
				{Name: "max_tool_calls", Type: pluginapi.ConfigFieldTypeInteger, Description: "Set max_tool_calls when greater than zero."},
				{Name: "search_context_size", Type: pluginapi.ConfigFieldTypeString, Description: "Responses web_search context size: low, medium, or high."},
				{Name: "return_token_budget", Type: pluginapi.ConfigFieldTypeString, Description: "Optional Responses web_search return_token_budget: default or unlimited."},
				{Name: "disable_live_web_access", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Set external_web_access=false for cache/index-only web search."},
				{Name: "allowed_domains", Type: pluginapi.ConfigFieldTypeArray, Description: "Optional Responses web_search filters.allowed_domains, up to 100. Omit https://."},
				{Name: "blocked_domains", Type: pluginapi.ConfigFieldTypeArray, Description: "Optional Responses web_search filters.blocked_domains, up to 100. Omit https://."},
				{Name: "include_action_sources", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Add web_search_call.action.sources to include[]."},
				{Name: "include_raw_results", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Add web_search_call.results to include[]. Useful for image search and debugging."},
				{Name: "enable_image_search", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Set search_content_types=[text,image] on web_search."},
				{Name: "image_max_results", Type: pluginapi.ConfigFieldTypeInteger, Description: "Optional image_settings.max_results when image search is enabled."},
				{Name: "image_captions", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Set image_settings.caption=true when image search is enabled."},
				{Name: "add_instruction", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Append an instruction asking Codex to use web_search for current/source-grounded tasks."},
				{Name: "instruction", Type: pluginapi.ConfigFieldTypeString, Description: "Instruction text injected into Responses instructions or chat system messages."},
				{Name: "chat_search_model", Type: pluginapi.ConfigFieldTypeString, Description: "Chat Completions search model used only if rewrite_chat_to_search_model is enabled."},
				{Name: "rewrite_chat_to_search_model", Type: pluginapi.ConfigFieldTypeBoolean, Description: "For Chat Completions bodies, rewrite model to chat_search_model. Off by default."},
			},
		},
		Capabilities: registrationCapability{RequestInterceptor: true},
	}
}

func interceptRequest(raw []byte, phase string) ([]byte, error) {
	var req pluginapi.RequestInterceptRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	cfg := loadedConfig()
	if !cfg.Enabled || (phase == "before" && !cfg.InjectBeforeAuth) || (phase == "after" && !cfg.InjectAfterAuth) {
		return okEnvelope(pluginapi.RequestInterceptResponse{})
	}
	if !isTargetRequest(cfg, req) {
		return okEnvelope(pluginapi.RequestInterceptResponse{})
	}
	nextBody, changed := mutateJSONBody(cfg, req.Body)
	if !changed {
		return okEnvelope(pluginapi.RequestInterceptResponse{})
	}
	return okEnvelope(pluginapi.RequestInterceptResponse{Body: nextBody})
}

func isTargetRequest(cfg pluginConfig, req pluginapi.RequestInterceptRequest) bool {
	if cfg.ForceAllRequests {
		return true
	}
	formatMatches := false
	for _, candidate := range []string{req.SourceFormat, req.ToFormat} {
		c := strings.ToLower(strings.TrimSpace(candidate))
		if c == "" {
			continue
		}
		if c == "codex" {
			formatMatches = true
			break
		}
		for _, pattern := range cfg.TargetFormats {
			if wildcardMatch(strings.ToLower(strings.TrimSpace(pattern)), c) {
				formatMatches = true
				break
			}
		}
		if formatMatches {
			break
		}
	}
	if cfg.RequireCodexFormat && !formatMatches {
		return false
	}
	if formatMatches {
		return true
	}
	for _, candidate := range []string{req.Model, req.RequestedModel} {
		c := strings.ToLower(strings.TrimSpace(candidate))
		if c == "" {
			continue
		}
		for _, pattern := range cfg.TargetModels {
			if wildcardMatch(strings.ToLower(strings.TrimSpace(pattern)), c) {
				return true
			}
		}
	}
	return false
}

func mutateJSONBody(cfg pluginConfig, body []byte) ([]byte, bool) {
	var obj map[string]any
	if len(body) == 0 || json.Unmarshal(body, &obj) != nil {
		return nil, false
	}

	_, hasInput := obj["input"]
	_, hasMessages := obj["messages"]
	if !hasInput && (!hasMessages || !cfg.AllowChatCompletions) {
		return nil, false
	}

	changed := false
	if hasInput {
		if ensureTool(obj, webSearchTool(cfg), cfg.ForceOverwriteTool) {
			changed = true
		}
		if cfg.ToolChoiceRequired {
			if old, ok := obj["tool_choice"].(string); !ok || old != "required" {
				obj["tool_choice"] = "required"
				changed = true
			}
		}
		if cfg.SetParallelToolCalls {
			if old, ok := obj["parallel_tool_calls"].(bool); !ok || !old {
				obj["parallel_tool_calls"] = true
				changed = true
			}
		}
		if cfg.MaxToolCalls > 0 {
			if old, ok := obj["max_tool_calls"].(float64); !ok || int(old) != cfg.MaxToolCalls {
				obj["max_tool_calls"] = cfg.MaxToolCalls
				changed = true
			}
		}
		if cfg.IncludeActionSources {
			if ensureInclude(obj, "web_search_call.action.sources") {
				changed = true
			}
		}
		if cfg.IncludeRawResults {
			if ensureInclude(obj, "web_search_call.results") {
				changed = true
			}
		}
		if cfg.AddInstruction && cfg.Instruction != "" {
			if addResponsesInstruction(obj, cfg.Instruction) {
				changed = true
			}
		}
	} else if hasMessages && cfg.AllowChatCompletions {
		if ensureChatWebSearchOptions(obj) {
			changed = true
		}
		if cfg.RewriteChatToSearchModel {
			if old, _ := obj["model"].(string); old != cfg.ChatSearchModel {
				obj["model"] = cfg.ChatSearchModel
				changed = true
			}
		}
		if cfg.AddInstruction && cfg.Instruction != "" {
			if addChatSystemInstruction(obj, cfg.Instruction) {
				changed = true
			}
		}
	}
	if !changed {
		return nil, false
	}
	out, err := json.Marshal(obj)
	if err != nil {
		return nil, false
	}
	return out, true
}

func webSearchTool(cfg pluginConfig) map[string]any {
	tool := map[string]any{"type": "web_search"}
	if cfg.SearchContextSize != "" {
		tool["search_context_size"] = cfg.SearchContextSize
	}
	if cfg.ReturnTokenBudget != "" && cfg.ReturnTokenBudget != "default" {
		tool["return_token_budget"] = cfg.ReturnTokenBudget
	}
	if cfg.DisableLiveWebAccess {
		tool["external_web_access"] = false
	}
	filters := map[string]any{}
	if len(cfg.AllowedDomains) > 0 {
		filters["allowed_domains"] = limitStrings(cleanDomains(cfg.AllowedDomains), 100)
	}
	if len(cfg.BlockedDomains) > 0 {
		filters["blocked_domains"] = limitStrings(cleanDomains(cfg.BlockedDomains), 100)
	}
	if len(filters) > 0 {
		tool["filters"] = filters
	}
	if cfg.EnableImageSearch {
		tool["search_content_types"] = []string{"text", "image"}
		imageSettings := map[string]any{}
		if cfg.ImageMaxResults > 0 {
			imageSettings["max_results"] = cfg.ImageMaxResults
		}
		if cfg.ImageCaptions {
			imageSettings["caption"] = true
		}
		if len(imageSettings) > 0 {
			tool["image_settings"] = imageSettings
		}
	}
	return tool
}

func ensureTool(obj map[string]any, tool map[string]any, overwrite bool) bool {
	toolType, _ := tool["type"].(string)
	rawTools, _ := obj["tools"].([]any)
	for i, existing := range rawTools {
		existingMap, ok := existing.(map[string]any)
		if !ok {
			continue
		}
		if t, _ := existingMap["type"].(string); t == toolType {
			if overwrite {
				rawTools[i] = tool
				obj["tools"] = rawTools
				return true
			}
			return false
		}
	}
	obj["tools"] = append(rawTools, tool)
	return true
}

func ensureInclude(obj map[string]any, value string) bool {
	rawInclude, _ := obj["include"].([]any)
	for _, item := range rawInclude {
		if s, _ := item.(string); s == value {
			return false
		}
	}
	obj["include"] = append(rawInclude, value)
	return true
}

func ensureChatWebSearchOptions(obj map[string]any) bool {
	if opts, ok := obj["web_search_options"].(map[string]any); ok && opts != nil {
		return false
	}
	obj["web_search_options"] = map[string]any{}
	return true
}

func addResponsesInstruction(obj map[string]any, instruction string) bool {
	if existing, ok := obj["instructions"].(string); ok && strings.Contains(existing, instruction) {
		return false
	}
	if existing, ok := obj["instructions"].(string); ok && strings.TrimSpace(existing) != "" {
		obj["instructions"] = existing + "\n\n" + instruction
	} else {
		obj["instructions"] = instruction
	}
	return true
}

func addChatSystemInstruction(obj map[string]any, instruction string) bool {
	rawMessages, ok := obj["messages"].([]any)
	if !ok {
		return false
	}
	for _, msg := range rawMessages {
		m, ok := msg.(map[string]any)
		if !ok {
			continue
		}
		if role, _ := m["role"].(string); role == "system" {
			if content, _ := m["content"].(string); strings.Contains(content, instruction) {
				return false
			}
			if content, _ := m["content"].(string); strings.TrimSpace(content) != "" {
				m["content"] = content + "\n\n" + instruction
			} else {
				m["content"] = instruction
			}
			return true
		}
	}
	obj["messages"] = append([]any{map[string]any{"role": "system", "content": instruction}}, rawMessages...)
	return true
}

func normalizeOneOf(value string, allowed []string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	for _, candidate := range allowed {
		if value == candidate {
			return value
		}
	}
	return ""
}

func cleanDomains(domains []string) []string {
	out := make([]string, 0, len(domains))
	for _, domain := range domains {
		d := strings.TrimSpace(strings.ToLower(domain))
		d = strings.TrimPrefix(d, "https://")
		d = strings.TrimPrefix(d, "http://")
		d = strings.TrimSuffix(d, "/")
		if d != "" {
			out = append(out, d)
		}
	}
	return out
}

func limitStrings(in []string, max int) []string {
	out := make([]string, 0, minInt(len(in), max))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
			if len(out) == max {
				break
			}
		}
	}
	return out
}

func wildcardMatch(pattern, value string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	if pattern == "*" {
		return true
	}
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == value
	}
	if !strings.HasPrefix(value, parts[0]) {
		return false
	}
	pos := len(parts[0])
	for _, part := range parts[1 : len(parts)-1] {
		if part == "" {
			continue
		}
		idx := strings.Index(value[pos:], part)
		if idx < 0 {
			return false
		}
		pos += idx + len(part)
	}
	last := parts[len(parts)-1]
	return last == "" || strings.HasSuffix(value, last)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func okEnvelope(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return json.Marshal(envelope{OK: true, Result: raw})
}

func errorEnvelope(code, message string) []byte {
	raw, _ := json.Marshal(envelope{OK: false, Error: &envelopeError{Code: code, Message: message}})
	return raw
}

func writeResponse(response *C.cliproxy_buffer, raw []byte) {
	if response == nil || len(raw) == 0 {
		return
	}
	ptr := C.CBytes(raw)
	if ptr == nil {
		return
	}
	response.ptr = ptr
	response.len = C.size_t(len(raw))
}
