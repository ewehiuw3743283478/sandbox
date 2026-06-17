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

const pluginIdentifier = "codex-grok-force-search"

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
	Enabled              bool     `yaml:"enabled"`
	ForceAllRequests     bool     `yaml:"force_all_requests"`
	EnableCodex          bool     `yaml:"enable_codex"`
	EnableGrok           bool     `yaml:"enable_grok"`
	TargetCodexFormats   []string `yaml:"target_codex_formats"`
	TargetGrokFormats    []string `yaml:"target_grok_formats"`
	TargetCodexModels    []string `yaml:"target_codex_models"`
	TargetGrokModels     []string `yaml:"target_grok_models"`
	InjectBeforeAuth     bool     `yaml:"inject_before_auth"`
	InjectAfterAuth      bool     `yaml:"inject_after_auth"`
	AllowChatCompletions bool     `yaml:"allow_chat_completions"`
	ForceOverwriteTool   bool     `yaml:"force_overwrite_tool"`

	// Model-router controls.
	EnableModelRouter bool   `yaml:"enable_model_router"`
	CodexRouteProvider string `yaml:"codex_route_provider"`
	GrokRouteProvider  string `yaml:"grok_route_provider"`
	CodexRouteModel    string `yaml:"codex_route_model"`
	GrokRouteModel     string `yaml:"grok_route_model"`

	// Shared Responses controls.
	ToolChoiceRequired   bool   `yaml:"tool_choice_required"`
	SetParallelToolCalls bool   `yaml:"set_parallel_tool_calls"`
	MaxToolCalls         int    `yaml:"max_tool_calls"`
	AddInstruction       bool   `yaml:"add_instruction"`
	CodexInstruction     string `yaml:"codex_instruction"`
	GrokInstruction      string `yaml:"grok_instruction"`

	// xAI/Grok reasoning control.
	GrokReasoningEffort string `yaml:"grok_reasoning_effort"`

	// OpenAI/Codex hosted web_search options.
	SearchContextSize       string   `yaml:"search_context_size"`
	ReturnTokenBudget       string   `yaml:"return_token_budget"`
	DisableLiveWebAccess    bool     `yaml:"disable_live_web_access"`
	AllowedDomains          []string `yaml:"allowed_domains"`
	BlockedDomains          []string `yaml:"blocked_domains"`
	IncludeActionSources    bool     `yaml:"include_action_sources"`
	IncludeRawResults       bool     `yaml:"include_raw_results"`
	EnableOpenAIImageSearch bool     `yaml:"enable_openai_image_search"`
	ImageMaxResults         int      `yaml:"image_max_results"`
	ImageCaptions           bool     `yaml:"image_captions"`

	// Chat Completions fallback; Codex should normally use Responses.
	ChatSearchModel          string `yaml:"chat_search_model"`
	RewriteChatToSearchModel bool   `yaml:"rewrite_chat_to_search_model"`

	// xAI/Grok web_search options.
	GrokAllowedDomains           []string `yaml:"grok_allowed_domains"`
	GrokExcludedDomains          []string `yaml:"grok_excluded_domains"`
	GrokEnableImageUnderstanding bool     `yaml:"grok_enable_image_understanding"`
	GrokEnableImageSearch        bool     `yaml:"grok_enable_image_search"`

	// xAI/Grok x_search options.
	GrokAllowedXHandles           []string `yaml:"grok_allowed_x_handles"`
	GrokExcludedXHandles          []string `yaml:"grok_excluded_x_handles"`
	GrokFromDate                  string   `yaml:"grok_from_date"`
	GrokToDate                    string   `yaml:"grok_to_date"`
	GrokXEnableImageUnderstanding bool     `yaml:"grok_x_enable_image_understanding"`
	GrokXEnableVideoUnderstanding bool     `yaml:"grok_x_enable_video_understanding"`
}

type registration struct {
	SchemaVersion uint32                 `json:"schema_version"`
	Metadata      pluginapi.Metadata     `json:"metadata"`
	Capabilities  registrationCapability `json:"capabilities"`
}

type registrationCapability struct {
	RequestInterceptor bool `json:"request_interceptor"`
	ModelRouter        bool `json:"model_router"`
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
	case pluginabi.MethodModelRoute:
		return routeModel(request)
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
		ForceAllRequests:     true,
		EnableCodex:          true,
		EnableGrok:           true,
		EnableModelRouter:    true,
		CodexRouteProvider:   "codex",
		GrokRouteProvider:    "xai",
		TargetCodexFormats:   []string{"codex", "openai"},
		TargetGrokFormats:    []string{"xai", "x-ai", "grok"},
		TargetCodexModels:    []string{"gpt-5*", "gpt-4.1*", "o3*", "o4*"},
		TargetGrokModels:     []string{"grok*", "grok-*", "xai/*", "x-ai/*"},
		InjectBeforeAuth:     false,
		InjectAfterAuth:      true,
		AllowChatCompletions: false,
		ToolChoiceRequired:   true,
		SetParallelToolCalls: true,
		MaxToolCalls:         6,
		AddInstruction:       true,
		GrokReasoningEffort:  "high",
		SearchContextSize:    "medium",
		IncludeActionSources: true,
		ChatSearchModel:      "gpt-5-search-api",
		CodexInstruction:     "For any question that may require current, external, API, package, repository, changelog, security, pricing, policy, or source-grounded information, use the hosted web_search tool before answering. Prefer official documentation and primary sources. Include citations when the response format supports them.",
		GrokInstruction:      "For any question that may require current, external, social/X, API, package, repository, changelog, security, pricing, policy, or source-grounded information, use both web_search and x_search before answering when available. Prefer official documentation, primary sources, and relevant X posts. Include citations when the response format supports them.",
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
	cfg.GrokReasoningEffort = normalizeOneOf(cfg.GrokReasoningEffort, []string{"none", "low", "medium", "high"})

	cfg.CodexInstruction = strings.TrimSpace(cfg.CodexInstruction)
	cfg.GrokInstruction = strings.TrimSpace(cfg.GrokInstruction)
	cfg.CodexRouteProvider = strings.ToLower(strings.TrimSpace(cfg.CodexRouteProvider))
	cfg.GrokRouteProvider = strings.ToLower(strings.TrimSpace(cfg.GrokRouteProvider))
	cfg.CodexRouteModel = strings.TrimSpace(cfg.CodexRouteModel)
	cfg.GrokRouteModel = strings.TrimSpace(cfg.GrokRouteModel)

	if cfg.CodexInstruction == "" {
		cfg.CodexInstruction = defaultPluginConfig().CodexInstruction
	}
	if cfg.GrokInstruction == "" {
		cfg.GrokInstruction = defaultPluginConfig().GrokInstruction
	}
	if cfg.GrokReasoningEffort == "" {
		cfg.GrokReasoningEffort = defaultPluginConfig().GrokReasoningEffort
	}
	if cfg.CodexRouteProvider == "" {
		cfg.CodexRouteProvider = defaultPluginConfig().CodexRouteProvider
	}
	if cfg.GrokRouteProvider == "" {
		cfg.GrokRouteProvider = defaultPluginConfig().GrokRouteProvider
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
			Version:          "0.2.5",
			Author:           "ewehiuw3743283478",
			GitHubRepository: "https://github.com/ewehiuw3743283478/sandbox",
			ConfigFields: []pluginapi.ConfigField{
				{Name: "enabled", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Enable or disable this plugin."},
				{Name: "enable_codex", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Inject OpenAI web_search into Codex/OpenAI Responses requests."},
				{Name: "enable_grok", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Inject xAI web_search and x_search into Grok/xAI Responses requests."},
				{Name: "enable_model_router", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Route matching models to built-in codex/xai providers before provider/auth resolution."},
				{Name: "codex_route_provider", Type: pluginapi.ConfigFieldTypeString, Description: "Built-in provider key for Codex/OpenAI routes. Default: codex."},
				{Name: "grok_route_provider", Type: pluginapi.ConfigFieldTypeString, Description: "Built-in provider key for Grok/xAI routes. Default: xai."},
				{Name: "codex_route_model", Type: pluginapi.ConfigFieldTypeString, Description: "Optional provider-native model override for Codex routes. Empty keeps original model."},
				{Name: "grok_route_model", Type: pluginapi.ConfigFieldTypeString, Description: "Optional provider-native model override for Grok routes. Empty keeps original model."},
				{Name: "force_all_requests", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Inject based on request body/model even without known format."},
				{Name: "target_codex_formats", Type: pluginapi.ConfigFieldTypeArray, Description: "Format patterns that identify Codex/OpenAI requests."},
				{Name: "target_grok_formats", Type: pluginapi.ConfigFieldTypeArray, Description: "Format patterns that identify Grok/xAI requests."},
				{Name: "target_codex_models", Type: pluginapi.ConfigFieldTypeArray, Description: "Model patterns that identify OpenAI/Codex requests."},
				{Name: "target_grok_models", Type: pluginapi.ConfigFieldTypeArray, Description: "Model patterns that identify Grok/xAI requests."},
				{Name: "inject_before_auth", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Rewrite before credential selection. Usually false."},
				{Name: "inject_after_auth", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Rewrite after credential selection. Usually true."},
				{Name: "allow_chat_completions", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Also mutate Chat Completions-shaped bodies for OpenAI search-model fallback."},
				{Name: "force_overwrite_tool", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Replace existing tool definitions instead of preserving user-supplied options."},
				{Name: "tool_choice_required", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Set tool_choice=required on Responses bodies."},
				{Name: "set_parallel_tool_calls", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Set parallel_tool_calls=true."},
				{Name: "max_tool_calls", Type: pluginapi.ConfigFieldTypeInteger, Description: "Set max_tool_calls for Grok/xAI requests only. Codex/OpenAI requests always remove this field."},
				{Name: "grok_reasoning_effort", Type: pluginapi.ConfigFieldTypeString, Description: "xAI Grok reasoning.effort: none, low, medium, or high. Default: high."},
				{Name: "search_context_size", Type: pluginapi.ConfigFieldTypeString, Description: "OpenAI Responses web_search context size: low, medium, or high."},
				{Name: "return_token_budget", Type: pluginapi.ConfigFieldTypeString, Description: "OpenAI Responses web_search return_token_budget: default or unlimited."},
				{Name: "disable_live_web_access", Type: pluginapi.ConfigFieldTypeBoolean, Description: "OpenAI: set external_web_access=false for cache/index-only web search."},
				{Name: "allowed_domains", Type: pluginapi.ConfigFieldTypeArray, Description: "OpenAI web_search filters.allowed_domains."},
				{Name: "blocked_domains", Type: pluginapi.ConfigFieldTypeArray, Description: "OpenAI web_search filters.blocked_domains."},
				{Name: "grok_allowed_domains", Type: pluginapi.ConfigFieldTypeArray, Description: "xAI web_search filters.allowed_domains, max 5."},
				{Name: "grok_excluded_domains", Type: pluginapi.ConfigFieldTypeArray, Description: "xAI web_search filters.excluded_domains, max 5."},
				{Name: "grok_allowed_x_handles", Type: pluginapi.ConfigFieldTypeArray, Description: "xAI x_search allowed_x_handles, max 20."},
				{Name: "grok_excluded_x_handles", Type: pluginapi.ConfigFieldTypeArray, Description: "xAI x_search excluded_x_handles, max 20."},
				{Name: "grok_from_date", Type: pluginapi.ConfigFieldTypeString, Description: "xAI x_search from_date in YYYY-MM-DD format."},
				{Name: "grok_to_date", Type: pluginapi.ConfigFieldTypeString, Description: "xAI x_search to_date in YYYY-MM-DD format."},
				{Name: "grok_enable_image_understanding", Type: pluginapi.ConfigFieldTypeBoolean, Description: "xAI web_search enable_image_understanding."},
				{Name: "grok_enable_image_search", Type: pluginapi.ConfigFieldTypeBoolean, Description: "xAI web_search enable_image_search."},
				{Name: "grok_x_enable_image_understanding", Type: pluginapi.ConfigFieldTypeBoolean, Description: "xAI x_search enable_image_understanding."},
				{Name: "grok_x_enable_video_understanding", Type: pluginapi.ConfigFieldTypeBoolean, Description: "xAI x_search enable_video_understanding."},
				{Name: "include_action_sources", Type: pluginapi.ConfigFieldTypeBoolean, Description: "OpenAI: include web_search_call.action.sources."},
				{Name: "include_raw_results", Type: pluginapi.ConfigFieldTypeBoolean, Description: "OpenAI: include web_search_call.results."},
				{Name: "add_instruction", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Append a model-specific search instruction."},
				{Name: "codex_instruction", Type: pluginapi.ConfigFieldTypeString, Description: "Instruction injected into Codex/OpenAI requests."},
				{Name: "grok_instruction", Type: pluginapi.ConfigFieldTypeString, Description: "Instruction injected into Grok/xAI requests."},
			},
		},
		Capabilities: registrationCapability{
			RequestInterceptor: true,
			ModelRouter:        true,
		},
	}
}

func routeModel(raw []byte) ([]byte, error) {
	var req pluginapi.ModelRouteRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}

	cfg := loadedConfig()
	if !cfg.Enabled || !cfg.EnableModelRouter {
		return okEnvelope(pluginapi.ModelRouteResponse{Handled: false})
	}

	requestedModel := firstNonEmpty(req.RequestedModel, bodyModelName(req.Body))
	if requestedModel == "" {
		return okEnvelope(pluginapi.ModelRouteResponse{Handled: false})
	}

	format := strings.ToLower(strings.TrimSpace(req.SourceFormat))
	modelCandidates := []string{requestedModel}

	if cfg.EnableGrok && (matchesAnyPattern([]string{format}, cfg.TargetGrokFormats) || matchesAnyPattern(modelCandidates, cfg.TargetGrokModels)) {
		if hasProvider(req.AvailableProviders, cfg.GrokRouteProvider) {
			return okEnvelope(pluginapi.ModelRouteResponse{
				Handled:     true,
				TargetKind:  pluginapi.ModelRouteTargetProvider,
				Target:      cfg.GrokRouteProvider,
				TargetModel: routeModelName(cfg.GrokRouteModel, requestedModel),
				Reason:      "matched_grok_force_search",
			})
		}
		return okEnvelope(pluginapi.ModelRouteResponse{Handled: false})
	}

	if cfg.EnableCodex && (matchesAnyPattern([]string{format}, cfg.TargetCodexFormats) || matchesAnyPattern(modelCandidates, cfg.TargetCodexModels)) {
		if hasProvider(req.AvailableProviders, cfg.CodexRouteProvider) {
			return okEnvelope(pluginapi.ModelRouteResponse{
				Handled:     true,
				TargetKind:  pluginapi.ModelRouteTargetProvider,
				Target:      cfg.CodexRouteProvider,
				TargetModel: routeModelName(cfg.CodexRouteModel, requestedModel),
				Reason:      "matched_codex_force_web_search",
			})
		}
		return okEnvelope(pluginapi.ModelRouteResponse{Handled: false})
	}

	return okEnvelope(pluginapi.ModelRouteResponse{Handled: false})
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

	target := targetKind(cfg, req)
	if target == "" {
		return okEnvelope(pluginapi.RequestInterceptResponse{})
	}

	nextBody, changed := mutateJSONBody(cfg, req.Body, target)
	if !changed {
		return okEnvelope(pluginapi.RequestInterceptResponse{})
	}

	return okEnvelope(pluginapi.RequestInterceptResponse{Body: nextBody})
}

func targetKind(cfg pluginConfig, req pluginapi.RequestInterceptRequest) string {
	bodyModel := bodyModelName(req.Body)
	formats := []string{req.SourceFormat, req.ToFormat}
	models := []string{req.Model, req.RequestedModel, bodyModel}

	if cfg.EnableGrok && (matchesAnyPattern(formats, cfg.TargetGrokFormats) || matchesAnyPattern(models, cfg.TargetGrokModels)) {
		return "grok"
	}
	if cfg.EnableCodex && (matchesAnyPattern(formats, cfg.TargetCodexFormats) || matchesAnyPattern(models, cfg.TargetCodexModels)) {
		return "codex"
	}
	if !cfg.ForceAllRequests {
		return ""
	}
	if cfg.EnableGrok && strings.Contains(strings.ToLower(bodyModel), "grok") {
		return "grok"
	}
	if cfg.EnableCodex && bodyModel != "" {
		return "codex"
	}
	return ""
}

func bodyModelName(body []byte) string {
	var obj map[string]any
	if len(body) == 0 || json.Unmarshal(body, &obj) != nil {
		return ""
	}
	model, _ := obj["model"].(string)
	return model
}

func mutateJSONBody(cfg pluginConfig, body []byte, target string) ([]byte, bool) {
	var obj map[string]any
	if len(body) == 0 || json.Unmarshal(body, &obj) != nil {
		return nil, false
	}

	_, hasInput := obj["input"]
	_, hasMessages := obj["messages"]
	if !hasInput && (!hasMessages || !cfg.AllowChatCompletions || target != "codex") {
		return nil, false
	}

	changed := false

	if hasInput {
		if target == "grok" {
			if ensureTool(obj, grokWebSearchTool(cfg), cfg.ForceOverwriteTool) {
				changed = true
			}
			if ensureTool(obj, grokXSearchTool(cfg), cfg.ForceOverwriteTool) {
				changed = true
			}
			if cfg.AddInstruction && cfg.GrokInstruction != "" {
				if addResponsesInstruction(obj, cfg.GrokInstruction) {
					changed = true
				}
			}
			if cfg.GrokReasoningEffort != "" {
				if ensureReasoningEffort(obj, cfg.GrokReasoningEffort) {
					changed = true
				}
			}
		} else {
			if ensureTool(obj, openAIWebSearchTool(cfg), cfg.ForceOverwriteTool) {
				changed = true
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
			if cfg.AddInstruction && cfg.CodexInstruction != "" {
				if addResponsesInstruction(obj, cfg.CodexInstruction) {
					changed = true
				}
			}
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

		// Codex/OpenAI route rejects max_tool_calls in your current path.
		// Grok behavior is unchanged: Grok still receives max_tool_calls when configured.
		if target == "codex" {
			if _, exists := obj["max_tool_calls"]; exists {
				delete(obj, "max_tool_calls")
				changed = true
			}
		} else if cfg.MaxToolCalls > 0 {
			if old, ok := obj["max_tool_calls"].(float64); !ok || int(old) != cfg.MaxToolCalls {
				obj["max_tool_calls"] = cfg.MaxToolCalls
				changed = true
			}
		}
	} else if hasMessages && cfg.AllowChatCompletions && target == "codex" {
		if ensureChatWebSearchOptions(obj) {
			changed = true
		}
		if cfg.RewriteChatToSearchModel {
			if old, _ := obj["model"].(string); old != cfg.ChatSearchModel {
				obj["model"] = cfg.ChatSearchModel
				changed = true
			}
		}
		if cfg.AddInstruction && cfg.CodexInstruction != "" {
			if addChatSystemInstruction(obj, cfg.CodexInstruction) {
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

func openAIWebSearchTool(cfg pluginConfig) map[string]any {
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

	if cfg.EnableOpenAIImageSearch {
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

func grokWebSearchTool(cfg pluginConfig) map[string]any {
	tool := map[string]any{"type": "web_search"}

	filters := map[string]any{}
	if len(cfg.GrokAllowedDomains) > 0 {
		filters["allowed_domains"] = limitStrings(cleanDomains(cfg.GrokAllowedDomains), 5)
	} else if len(cfg.GrokExcludedDomains) > 0 {
		filters["excluded_domains"] = limitStrings(cleanDomains(cfg.GrokExcludedDomains), 5)
	}
	if len(filters) > 0 {
		tool["filters"] = filters
	}

	if cfg.GrokEnableImageUnderstanding {
		tool["enable_image_understanding"] = true
	}
	if cfg.GrokEnableImageSearch {
		tool["enable_image_search"] = true
	}

	return tool
}

func grokXSearchTool(cfg pluginConfig) map[string]any {
	tool := map[string]any{"type": "x_search"}

	if len(cfg.GrokAllowedXHandles) > 0 {
		tool["allowed_x_handles"] = limitStrings(cleanHandles(cfg.GrokAllowedXHandles), 20)
	} else if len(cfg.GrokExcludedXHandles) > 0 {
		tool["excluded_x_handles"] = limitStrings(cleanHandles(cfg.GrokExcludedXHandles), 20)
	}

	if strings.TrimSpace(cfg.GrokFromDate) != "" {
		tool["from_date"] = strings.TrimSpace(cfg.GrokFromDate)
	}
	if strings.TrimSpace(cfg.GrokToDate) != "" {
		tool["to_date"] = strings.TrimSpace(cfg.GrokToDate)
	}
	if cfg.GrokXEnableImageUnderstanding {
		tool["enable_image_understanding"] = true
	}
	if cfg.GrokXEnableVideoUnderstanding {
		tool["enable_video_understanding"] = true
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

func ensureReasoningEffort(obj map[string]any, effort string) bool {
	effort = strings.ToLower(strings.TrimSpace(effort))
	if effort == "" {
		return false
	}

	if existing, ok := obj["reasoning"].(map[string]any); ok && existing != nil {
		if old, _ := existing["effort"].(string); old == effort {
			return false
		}
		existing["effort"] = effort
		return true
	}

	obj["reasoning"] = map[string]any{"effort": effort}
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

func hasProvider(providers []string, target string) bool {
	target = strings.ToLower(strings.TrimSpace(target))
	if target == "" {
		return false
	}
	for _, provider := range providers {
		if strings.ToLower(strings.TrimSpace(provider)) == target {
			return true
		}
	}
	return false
}

func routeModelName(override string, fallback string) string {
	override = strings.TrimSpace(override)
	if override != "" {
		return override
	}
	return strings.TrimSpace(fallback)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func matchesAnyPattern(candidates []string, patterns []string) bool {
	for _, candidate := range candidates {
		c := strings.ToLower(strings.TrimSpace(candidate))
		if c == "" {
			continue
		}
		for _, pattern := range patterns {
			if wildcardMatch(strings.ToLower(strings.TrimSpace(pattern)), c) {
				return true
			}
		}
	}
	return false
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

func cleanHandles(handles []string) []string {
	out := make([]string, 0, len(handles))
	for _, handle := range handles {
		h := strings.TrimSpace(handle)
		h = strings.TrimPrefix(h, "@")
		if h != "" {
			out = append(out, h)
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
