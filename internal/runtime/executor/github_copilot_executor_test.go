package executor

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	copilotauth "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/copilot"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v6/sdk/translator"
	"github.com/tidwall/gjson"
)

func TestGitHubCopilotApplyHeaders_UsesLatestProfile(t *testing.T) {
	t.Parallel()

	req, err := http.NewRequest(http.MethodPost, "https://example.com", bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}

	e := &GitHubCopilotExecutor{}
	e.applyHeaders(req, "token-123", nil)

	if got := req.Header.Get("User-Agent"); got != copilotUserAgent {
		t.Fatalf("User-Agent = %q, want %q", got, copilotUserAgent)
	}
	if got := req.Header.Get("Editor-Version"); got != copilotEditorVersion {
		t.Fatalf("Editor-Version = %q, want %q", got, copilotEditorVersion)
	}
	if got := req.Header.Get("Editor-Plugin-Version"); got != copilotPluginVersion {
		t.Fatalf("Editor-Plugin-Version = %q, want %q", got, copilotPluginVersion)
	}
	if got := req.Header.Get("Openai-Intent"); got != copilotOpenAIIntent {
		t.Fatalf("Openai-Intent = %q, want %q", got, copilotOpenAIIntent)
	}
	if got := req.Header.Get("Copilot-Integration-Id"); got != copilotIntegrationID {
		t.Fatalf("Copilot-Integration-Id = %q, want %q", got, copilotIntegrationID)
	}
	if got := req.Header.Get("X-Github-Api-Version"); got != copilotGitHubAPIVer {
		t.Fatalf("X-Github-Api-Version = %q, want %q", got, copilotGitHubAPIVer)
	}
	if got := req.Header.Get("X-Initiator"); got != "user" {
		t.Fatalf("X-Initiator = %q, want user", got)
	}
	if got := req.Header.Get("X-Interaction-Type"); got != copilotInteractionType {
		t.Fatalf("X-Interaction-Type = %q, want %q", got, copilotInteractionType)
	}
	if got := req.Header.Get("X-Vscode-User-Agent-Library-Version"); got != copilotUserAgentLibVer {
		t.Fatalf("X-Vscode-User-Agent-Library-Version = %q, want %q", got, copilotUserAgentLibVer)
	}
}

func TestGitHubCopilotNormalizeModel_StripsSuffix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		model     string
		wantModel string
	}{
		{
			name:      "suffix stripped",
			model:     "claude-opus-4.6(medium)",
			wantModel: "claude-opus-4.6",
		},
		{
			name:      "no suffix unchanged",
			model:     "claude-opus-4.6",
			wantModel: "claude-opus-4.6",
		},
		{
			name:      "different suffix stripped",
			model:     "gpt-4o(high)",
			wantModel: "gpt-4o",
		},
		{
			name:      "numeric suffix stripped",
			model:     "gemini-2.5-pro(8192)",
			wantModel: "gemini-2.5-pro",
		},
	}

	e := &GitHubCopilotExecutor{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := []byte(`{"model":"` + tt.model + `","messages":[]}`)
			got := e.normalizeModel(tt.model, body)

			gotModel := gjson.GetBytes(got, "model").String()
			if gotModel != tt.wantModel {
				t.Fatalf("normalizeModel() model = %q, want %q", gotModel, tt.wantModel)
			}
		})
	}
}

func TestUseGitHubCopilotResponsesEndpoint_OpenAIResponseSource(t *testing.T) {
	t.Parallel()
	if !useGitHubCopilotResponsesEndpoint(sdktranslator.FromString("openai-response"), "claude-3-5-sonnet") {
		t.Fatal("expected openai-response source to use /responses")
	}
}

func TestUseGitHubCopilotResponsesEndpoint_CodexModel(t *testing.T) {
	t.Parallel()
	if !useGitHubCopilotResponsesEndpoint(sdktranslator.FromString("openai"), "gpt-5-codex") {
		t.Fatal("expected codex model to use /responses")
	}
}

func TestUseGitHubCopilotResponsesEndpoint_RegistryResponsesOnlyModel(t *testing.T) {
	// Not parallel: shares global model registry with DynamicRegistryWinsOverStatic.
	if !useGitHubCopilotResponsesEndpoint(sdktranslator.FromString("openai"), "gpt-5.4") {
		t.Fatal("expected responses-only registry model to use /responses")
	}
	if !useGitHubCopilotResponsesEndpoint(sdktranslator.FromString("openai"), "gpt-5.4-mini") {
		t.Fatal("expected responses-only registry model to use /responses")
	}
}

func TestUseGitHubCopilotResponsesEndpoint_DynamicRegistryWinsOverStatic(t *testing.T) {
	// Not parallel: mutates global model registry, conflicts with RegistryResponsesOnlyModel.

	reg := registry.GetGlobalRegistry()
	clientID := "github-copilot-test-client"
	reg.RegisterClient(clientID, "github-copilot", []*registry.ModelInfo{
		{
			ID:                 "gpt-5.4",
			SupportedEndpoints: []string{"/chat/completions", "/responses"},
		},
		{
			ID:                 "gpt-5.4-mini",
			SupportedEndpoints: []string{"/chat/completions", "/responses"},
		},
	})
	defer reg.UnregisterClient(clientID)

	if useGitHubCopilotResponsesEndpoint(sdktranslator.FromString("openai"), "gpt-5.4") {
		t.Fatal("expected dynamic registry definition to take precedence over static fallback")
	}

	if useGitHubCopilotResponsesEndpoint(sdktranslator.FromString("openai"), "gpt-5.4-mini") {
		t.Fatal("expected dynamic registry definition to take precedence over static fallback")
	}
}

func TestUseGitHubCopilotResponsesEndpoint_DefaultChat(t *testing.T) {
	t.Parallel()
	if useGitHubCopilotResponsesEndpoint(sdktranslator.FromString("openai"), "claude-3-5-sonnet") {
		t.Fatal("expected default openai source with non-codex model to use /chat/completions")
	}
}

func TestNormalizeGitHubCopilotClaudeThinking_PreservesExistingEffort(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"claude-opus-4.6","thinking":{"type":"adaptive"},"output_config":{"effort":"medium"}}`)
	original := []byte(`{"reasoning_effort":"high"}`)
	got := normalizeGitHubCopilotClaudeThinking("claude-opus-4.6", body, original)

	if effort := gjson.GetBytes(got, "output_config.effort").String(); effort != "medium" {
		t.Fatalf("output_config.effort = %q, want medium", effort)
	}
}

func TestNormalizeGitHubCopilotClaudeThinking_DerivesEffortFromOriginalRequest(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"claude-opus-4.6","thinking":{"type":"adaptive"}}`)
	original := []byte(`{"reasoning_effort":"high"}`)
	got := normalizeGitHubCopilotClaudeThinking("claude-opus-4.6", body, original)

	if effort := gjson.GetBytes(got, "output_config.effort").String(); effort != "high" {
		t.Fatalf("output_config.effort = %q, want high", effort)
	}
}

func TestNormalizeGitHubCopilotClaudeThinking_DoesNotEnableThinkingForNonOpusClaude(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"claude-sonnet-4.6","thinking":{"type":"adaptive"}}`)
	got := normalizeGitHubCopilotClaudeThinking("claude-sonnet-4.6", body, nil)

	if thinkingType := gjson.GetBytes(got, "thinking.type").String(); thinkingType != "adaptive" {
		t.Fatalf("thinking.type = %q, want adaptive passthrough", thinkingType)
	}
	if gjson.GetBytes(got, "output_config.effort").Exists() {
		t.Fatalf("output_config.effort should not be synthesized for non-opus claude, body=%s", string(got))
	}
}

func TestNormalizeGitHubCopilotClaudeThinking_ConvertsBudgetTokensToAdaptiveEffort(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"claude-opus-4.6","thinking":{"type":"enabled","budget_tokens":31999}}`)
	got := normalizeGitHubCopilotClaudeThinking("claude-opus-4.6", body, nil)

	if thinkingType := gjson.GetBytes(got, "thinking.type").String(); thinkingType != "adaptive" {
		t.Fatalf("thinking.type = %q, want adaptive", thinkingType)
	}
	if effort := gjson.GetBytes(got, "output_config.effort").String(); effort != "high" {
		t.Fatalf("output_config.effort = %q, want high", effort)
	}
	if gjson.GetBytes(got, "thinking.budget_tokens").Exists() {
		t.Fatalf("thinking.budget_tokens should be removed, body=%s", string(got))
	}
}

func TestNormalizeGitHubCopilotClaudeThinking_DisablesZeroBudgetThinking(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"claude-opus-4.6","thinking":{"type":"enabled","budget_tokens":0}}`)
	got := normalizeGitHubCopilotClaudeThinking("claude-opus-4.6", body, nil)

	if thinkingType := gjson.GetBytes(got, "thinking.type").String(); thinkingType != "disabled" {
		t.Fatalf("thinking.type = %q, want disabled", thinkingType)
	}
	if gjson.GetBytes(got, "thinking.budget_tokens").Exists() {
		t.Fatalf("thinking.budget_tokens should be removed, body=%s", string(got))
	}
	if gjson.GetBytes(got, "output_config.effort").Exists() {
		t.Fatalf("output_config.effort should be removed, body=%s", string(got))
	}
}

func TestResolveGitHubCopilotClaudeThinkingModeAndEffort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		model      string
		body       []byte
		original   []byte
		wantType   string
		wantEffort string
		wantOK     bool
	}{
		{
			name:   "non claude model ignored",
			model:  "gpt-4o",
			body:   []byte(`{"model":"gpt-4o","thinking":{"type":"enabled","budget_tokens":1024}}`),
			wantOK: false,
		},
		{
			name:       "reasoning effort in body maps to adaptive effort",
			model:      "claude-opus-4.6",
			body:       []byte(`{"model":"claude-opus-4.6","reasoning_effort":"medium"}`),
			wantType:   "adaptive",
			wantEffort: "medium",
			wantOK:     true,
		},
		{
			name:       "suffix maps to adaptive effort",
			model:      "claude-opus-4.6(medium)",
			body:       []byte(`{"model":"claude-opus-4.6"}`),
			wantType:   "adaptive",
			wantEffort: "medium",
			wantOK:     true,
		},
		{
			name:       "original disabled wins for opus",
			model:      "claude-opus-4.6",
			body:       []byte(`{"model":"claude-opus-4.6","reasoning_effort":"high"}`),
			original:   []byte(`{"thinking":{"type":"disabled"}}`),
			wantType:   "disabled",
			wantEffort: "",
			wantOK:     true,
		},
		{
			name:       "non opus claude ignored",
			model:      "claude-sonnet-4.6",
			body:       []byte(`{"model":"claude-sonnet-4.6","reasoning_effort":"high"}`),
			wantType:   "",
			wantEffort: "",
			wantOK:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotEffort, gotOK := resolveGitHubCopilotClaudeThinkingModeAndEffort(tt.model, tt.body, tt.original)
			if gotType != tt.wantType || gotEffort != tt.wantEffort || gotOK != tt.wantOK {
				t.Fatalf("got (type=%q, effort=%q, ok=%v), want (type=%q, effort=%q, ok=%v)", gotType, gotEffort, gotOK, tt.wantType, tt.wantEffort, tt.wantOK)
			}
		})
	}
}

func TestNormalizeGitHubCopilotClaudeThinking_PreservesBehaviorAcrossInputShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		model         string
		body          []byte
		original      []byte
		wantType      string
		wantEffort    string
		wantHasBudget bool
	}{
		{
			name:          "non claude model passthrough",
			model:         "gpt-4o",
			body:          []byte(`{"model":"gpt-4o","thinking":{"type":"enabled","budget_tokens":1024}}`),
			wantType:      "enabled",
			wantEffort:    "",
			wantHasBudget: true,
		},
		{
			name:          "reasoning effort in body maps to adaptive effort",
			model:         "claude-opus-4.6",
			body:          []byte(`{"model":"claude-opus-4.6","reasoning_effort":"medium"}`),
			wantType:      "adaptive",
			wantEffort:    "medium",
			wantHasBudget: false,
		},
		{
			name:          "suffix maps to adaptive effort",
			model:         "claude-opus-4.6(medium)",
			body:          []byte(`{"model":"claude-opus-4.6"}`),
			wantType:      "adaptive",
			wantEffort:    "medium",
			wantHasBudget: false,
		},
		{
			name:          "original disabled wins",
			model:         "claude-opus-4.6",
			body:          []byte(`{"model":"claude-opus-4.6","reasoning_effort":"high"}`),
			original:      []byte(`{"thinking":{"type":"disabled"}}`),
			wantType:      "disabled",
			wantEffort:    "",
			wantHasBudget: false,
		},
		{
			name:          "non opus claude passthrough",
			model:         "claude-sonnet-4.6",
			body:          []byte(`{"model":"claude-sonnet-4.6","reasoning_effort":"high"}`),
			wantType:      "",
			wantEffort:    "",
			wantHasBudget: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeGitHubCopilotClaudeThinking(tt.model, tt.body, tt.original)
			if thinkingType := gjson.GetBytes(got, "thinking.type").String(); thinkingType != tt.wantType {
				t.Fatalf("thinking.type = %q, want %q; body=%s", thinkingType, tt.wantType, string(got))
			}
			if effort := gjson.GetBytes(got, "output_config.effort").String(); effort != tt.wantEffort {
				t.Fatalf("output_config.effort = %q, want %q; body=%s", effort, tt.wantEffort, string(got))
			}
			if hasBudget := gjson.GetBytes(got, "thinking.budget_tokens").Exists(); hasBudget != tt.wantHasBudget {
				t.Fatalf("thinking.budget_tokens exists = %v, want %v; body=%s", hasBudget, tt.wantHasBudget, string(got))
			}
		})
	}
}

func TestApplyGitHubCopilotCapabilities_MapsClaudeThinkingSupport(t *testing.T) {
	t.Parallel()

	model := &registry.ModelInfo{ID: "claude-opus-4.6"}
	applyGitHubCopilotCapabilities(model, map[string]any{
		"limits": map[string]any{
			"max_context_window_tokens": float64(144000),
			"max_output_tokens":         float64(64000),
		},
		"supports": map[string]any{
			"adaptive_thinking":   true,
			"min_thinking_budget": float64(1024),
			"max_thinking_budget": float64(32000),
			"reasoning_effort":    []any{"low", "medium", "high"},
		},
	})

	if model.ContextLength != 144000 {
		t.Fatalf("ContextLength = %d, want 144000", model.ContextLength)
	}
	if model.MaxCompletionTokens != 64000 {
		t.Fatalf("MaxCompletionTokens = %d, want 64000", model.MaxCompletionTokens)
	}
	if model.Thinking == nil {
		t.Fatal("Thinking = nil, want populated support")
	}
	if model.Thinking.Min != 1024 || model.Thinking.Max != 32000 {
		t.Fatalf("Thinking budget = [%d,%d], want [1024,32000]", model.Thinking.Min, model.Thinking.Max)
	}
	if got := model.Thinking.Levels; strings.Join(got, ",") != "low,medium,high" {
		t.Fatalf("Thinking.Levels = %v, want [low medium high]", got)
	}
}

func TestApplyGitHubCopilotCapabilities_DoesNotInferThinkingFromHaikuBudgetOnly(t *testing.T) {
	t.Parallel()

	model := &registry.ModelInfo{ID: "claude-haiku-4.5"}
	applyGitHubCopilotCapabilities(model, map[string]any{
		"supports": map[string]any{
			"min_thinking_budget": float64(1024),
			"max_thinking_budget": float64(32000),
		},
	})

	if model.Thinking != nil {
		t.Fatalf("Thinking = %v, want nil for haiku budget-only capability card", model.Thinking)
	}
}

func TestNormalizeGitHubCopilotChatTools_KeepFunctionOnly(t *testing.T) {
	t.Parallel()
	body := []byte(`{"tools":[{"type":"function","function":{"name":"ok"}},{"type":"code_interpreter"}],"tool_choice":"auto"}`)
	got := normalizeGitHubCopilotChatTools(body)
	tools := gjson.GetBytes(got, "tools").Array()
	if len(tools) != 1 {
		t.Fatalf("tools len = %d, want 1", len(tools))
	}
	if tools[0].Get("type").String() != "function" {
		t.Fatalf("tool type = %q, want function", tools[0].Get("type").String())
	}
}

func TestNormalizeGitHubCopilotChatTools_InvalidToolChoiceDowngradeToAuto(t *testing.T) {
	t.Parallel()
	body := []byte(`{"tools":[],"tool_choice":{"type":"function","function":{"name":"x"}}}`)
	got := normalizeGitHubCopilotChatTools(body)
	if gjson.GetBytes(got, "tool_choice").String() != "auto" {
		t.Fatalf("tool_choice = %s, want auto", gjson.GetBytes(got, "tool_choice").Raw)
	}
}

func TestNormalizeGitHubCopilotResponsesInput_MissingInputExtractedFromSystemAndMessages(t *testing.T) {
	t.Parallel()
	body := []byte(`{"system":"sys text","messages":[{"role":"user","content":"user text"},{"role":"assistant","content":[{"type":"text","text":"assistant text"}]}]}`)
	got := normalizeGitHubCopilotResponsesInput(body)
	in := gjson.GetBytes(got, "input")
	if !in.IsArray() {
		t.Fatalf("input type = %v, want array", in.Type)
	}
	raw := in.Raw
	if !strings.Contains(raw, "sys text") || !strings.Contains(raw, "user text") || !strings.Contains(raw, "assistant text") {
		t.Fatalf("input = %s, want structured array with all texts", raw)
	}
	if gjson.GetBytes(got, "messages").Exists() {
		t.Fatal("messages should be removed after conversion")
	}
	if gjson.GetBytes(got, "system").Exists() {
		t.Fatal("system should be removed after conversion")
	}
}

func TestNormalizeGitHubCopilotResponsesInput_NonStringInputStringified(t *testing.T) {
	t.Parallel()
	body := []byte(`{"input":{"foo":"bar"}}`)
	got := normalizeGitHubCopilotResponsesInput(body)
	in := gjson.GetBytes(got, "input")
	if in.Type != gjson.String {
		t.Fatalf("input type = %v, want string", in.Type)
	}
	if !strings.Contains(in.String(), "foo") {
		t.Fatalf("input = %q, want stringified object", in.String())
	}
}

func TestNormalizeGitHubCopilotResponsesInput_StripsServiceTier(t *testing.T) {
	t.Parallel()
	body := []byte(`{"input":"user text","service_tier":"default"}`)
	got := normalizeGitHubCopilotResponsesInput(body)

	if gjson.GetBytes(got, "service_tier").Exists() {
		t.Fatalf("service_tier should be removed, got %s", gjson.GetBytes(got, "service_tier").Raw)
	}
	if gjson.GetBytes(got, "input").String() != "user text" {
		t.Fatalf("input = %q, want %q", gjson.GetBytes(got, "input").String(), "user text")
	}
}

func TestNormalizeGitHubCopilotResponsesTools_FlattenFunctionTools(t *testing.T) {
	t.Parallel()
	body := []byte(`{"tools":[{"type":"function","function":{"name":"sum","description":"d","parameters":{"type":"object"}}},{"type":"web_search"}]}`)
	got := normalizeGitHubCopilotResponsesTools(body)
	tools := gjson.GetBytes(got, "tools").Array()
	if len(tools) != 1 {
		t.Fatalf("tools len = %d, want 1", len(tools))
	}
	if tools[0].Get("name").String() != "sum" {
		t.Fatalf("tools[0].name = %q, want sum", tools[0].Get("name").String())
	}
	if !tools[0].Get("parameters").Exists() {
		t.Fatal("expected parameters to be preserved")
	}
}

func TestNormalizeGitHubCopilotResponsesTools_ClaudeFormatTools(t *testing.T) {
	t.Parallel()
	body := []byte(`{"tools":[{"name":"Bash","description":"Run commands","input_schema":{"type":"object","properties":{"command":{"type":"string"}},"required":["command"]}},{"name":"Read","description":"Read files","input_schema":{"type":"object","properties":{"path":{"type":"string"}}}}]}`)
	got := normalizeGitHubCopilotResponsesTools(body)
	tools := gjson.GetBytes(got, "tools").Array()
	if len(tools) != 2 {
		t.Fatalf("tools len = %d, want 2", len(tools))
	}
	if tools[0].Get("type").String() != "function" {
		t.Fatalf("tools[0].type = %q, want function", tools[0].Get("type").String())
	}
	if tools[0].Get("name").String() != "Bash" {
		t.Fatalf("tools[0].name = %q, want Bash", tools[0].Get("name").String())
	}
	if tools[0].Get("description").String() != "Run commands" {
		t.Fatalf("tools[0].description = %q, want 'Run commands'", tools[0].Get("description").String())
	}
	if !tools[0].Get("parameters").Exists() {
		t.Fatal("expected parameters to be set from input_schema")
	}
	if tools[0].Get("parameters.properties.command").Exists() != true {
		t.Fatal("expected parameters.properties.command to exist")
	}
	if tools[1].Get("name").String() != "Read" {
		t.Fatalf("tools[1].name = %q, want Read", tools[1].Get("name").String())
	}
}

func TestNormalizeGitHubCopilotResponsesTools_FlattenToolChoiceFunctionObject(t *testing.T) {
	t.Parallel()
	body := []byte(`{"tool_choice":{"type":"function","function":{"name":"sum"}}}`)
	got := normalizeGitHubCopilotResponsesTools(body)
	if gjson.GetBytes(got, "tool_choice.type").String() != "function" {
		t.Fatalf("tool_choice.type = %q, want function", gjson.GetBytes(got, "tool_choice.type").String())
	}
	if gjson.GetBytes(got, "tool_choice.name").String() != "sum" {
		t.Fatalf("tool_choice.name = %q, want sum", gjson.GetBytes(got, "tool_choice.name").String())
	}
}

func TestNormalizeGitHubCopilotResponsesTools_InvalidToolChoiceDowngradeToAuto(t *testing.T) {
	t.Parallel()
	body := []byte(`{"tool_choice":{"type":"function"}}`)
	got := normalizeGitHubCopilotResponsesTools(body)
	if gjson.GetBytes(got, "tool_choice").String() != "auto" {
		t.Fatalf("tool_choice = %s, want auto", gjson.GetBytes(got, "tool_choice").Raw)
	}
}

func TestTranslateGitHubCopilotResponsesNonStreamToClaude_TextMapping(t *testing.T) {
	t.Parallel()
	resp := []byte(`{"id":"resp_1","model":"gpt-5-codex","output":[{"type":"message","content":[{"type":"output_text","text":"hello"}]}],"usage":{"input_tokens":3,"output_tokens":5}}`)
	out := translateGitHubCopilotResponsesNonStreamToClaude(resp)
	if gjson.GetBytes(out, "type").String() != "message" {
		t.Fatalf("type = %q, want message", gjson.GetBytes(out, "type").String())
	}
	if gjson.GetBytes(out, "content.0.type").String() != "text" {
		t.Fatalf("content.0.type = %q, want text", gjson.GetBytes(out, "content.0.type").String())
	}
	if gjson.GetBytes(out, "content.0.text").String() != "hello" {
		t.Fatalf("content.0.text = %q, want hello", gjson.GetBytes(out, "content.0.text").String())
	}
}

func TestTranslateGitHubCopilotResponsesNonStreamToClaude_ToolUseMapping(t *testing.T) {
	t.Parallel()
	resp := []byte(`{"id":"resp_2","model":"gpt-5-codex","output":[{"type":"function_call","id":"fc_1","call_id":"call_1","name":"sum","arguments":"{\"a\":1}"}],"usage":{"input_tokens":1,"output_tokens":2}}`)
	out := translateGitHubCopilotResponsesNonStreamToClaude(resp)
	if gjson.GetBytes(out, "content.0.type").String() != "tool_use" {
		t.Fatalf("content.0.type = %q, want tool_use", gjson.GetBytes(out, "content.0.type").String())
	}
	if gjson.GetBytes(out, "content.0.name").String() != "sum" {
		t.Fatalf("content.0.name = %q, want sum", gjson.GetBytes(out, "content.0.name").String())
	}
	if gjson.GetBytes(out, "stop_reason").String() != "tool_use" {
		t.Fatalf("stop_reason = %q, want tool_use", gjson.GetBytes(out, "stop_reason").String())
	}
}

func TestTranslateGitHubCopilotResponsesStreamToClaude_TextLifecycle(t *testing.T) {
	t.Parallel()
	var param any

	created := translateGitHubCopilotResponsesStreamToClaude([]byte(`data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-5-codex"}}`), &param)
	if len(created) == 0 || !strings.Contains(string(created[0]), "message_start") {
		t.Fatalf("created events = %#v, want message_start", created)
	}

	delta := translateGitHubCopilotResponsesStreamToClaude([]byte(`data: {"type":"response.output_text.delta","delta":"he"}`), &param)
	joinedDelta := string(bytes.Join(delta, nil))
	if !strings.Contains(joinedDelta, "content_block_start") || !strings.Contains(joinedDelta, "text_delta") {
		t.Fatalf("delta events = %#v, want content_block_start + text_delta", delta)
	}

	completed := translateGitHubCopilotResponsesStreamToClaude([]byte(`data: {"type":"response.completed","response":{"usage":{"input_tokens":7,"output_tokens":9}}}`), &param)
	joinedCompleted := string(bytes.Join(completed, nil))
	if !strings.Contains(joinedCompleted, "message_delta") || !strings.Contains(joinedCompleted, "message_stop") {
		t.Fatalf("completed events = %#v, want message_delta + message_stop", completed)
	}
}

// --- Tests for X-Initiator detection logic (Problem L) ---

func TestApplyHeaders_XInitiator_AgentWhenUserCountIsNotMultipleOfFive(t *testing.T) {
	t.Parallel()
	e := &GitHubCopilotExecutor{}
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
	body := []byte(`{"messages":[{"role":"system","content":"sys"},{"role":"user","content":"hello"}]}`)
	e.applyHeaders(req, "token", body)
	if got := req.Header.Get("X-Initiator"); got != "agent" {
		t.Fatalf("X-Initiator = %q, want agent", got)
	}
}

func TestApplyHeaders_XInitiator_AgentWhenLastUserButHistoryHasAssistant(t *testing.T) {
	t.Parallel()
	e := &GitHubCopilotExecutor{}
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
	body := []byte(`{"messages":[{"role":"user","content":"hello"},{"role":"assistant","content":"I will read the file"},{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu1","content":"file contents..."}]}]}`)
	e.applyHeaders(req, "token", body)
	if got := req.Header.Get("X-Initiator"); got != "agent" {
		t.Fatalf("X-Initiator = %q, want agent (last user contains tool_result)", got)
	}
}

func TestApplyHeaders_XInitiator_AgentWithToolRole(t *testing.T) {
	t.Parallel()
	e := &GitHubCopilotExecutor{}
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
	// When the last message has role "tool", it's clearly agent-initiated.
	body := []byte(`{"messages":[{"role":"user","content":"hello"},{"role":"tool","content":"result"}]}`)
	e.applyHeaders(req, "token", body)
	if got := req.Header.Get("X-Initiator"); got != "agent" {
		t.Fatalf("X-Initiator = %q, want agent (last role is tool)", got)
	}
}

func TestApplyHeaders_XInitiator_InputArrayLastAssistantMessage(t *testing.T) {
	t.Parallel()
	e := &GitHubCopilotExecutor{}
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
	body := []byte(`{"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"Hi"}]},{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Hello"}]}]}`)
	e.applyHeaders(req, "token", body)
	if got := req.Header.Get("X-Initiator"); got != "agent" {
		t.Fatalf("X-Initiator = %q, want agent (last role is assistant)", got)
	}
}

func TestApplyHeaders_XInitiator_InputArrayAgentWhenLastUserButHistoryHasAssistant(t *testing.T) {
	t.Parallel()
	e := &GitHubCopilotExecutor{}
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
	// Responses API: last item is user-role but history contains assistant → agent.
	body := []byte(`{"input":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"I can help"}]},{"type":"message","role":"user","content":[{"type":"input_text","text":"Do X"}]}]}`)
	e.applyHeaders(req, "token", body)
	if got := req.Header.Get("X-Initiator"); got != "agent" {
		t.Fatalf("X-Initiator = %q, want agent (history has assistant)", got)
	}
}

func TestApplyHeaders_XInitiator_InputArrayLastFunctionCallOutput(t *testing.T) {
	t.Parallel()
	e := &GitHubCopilotExecutor{}
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
	body := []byte(`{"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"Use tool"}]},{"type":"function_call","call_id":"c1","name":"Read","arguments":"{}"},{"type":"function_call_output","call_id":"c1","output":"ok"}]}`)
	e.applyHeaders(req, "token", body)
	if got := req.Header.Get("X-Initiator"); got != "agent" {
		t.Fatalf("X-Initiator = %q, want agent (last item maps to tool role)", got)
	}
}

func TestApplyHeaders_XInitiator_ClaudeNativeToolResult(t *testing.T) {
	t.Parallel()
	e := &GitHubCopilotExecutor{}
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
	body := []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]},{"role":"assistant","content":[{"type":"tool_use","id":"tu_1","name":"Read","input":{}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_1","content":"file contents"}]}]}`)
	e.applyHeaders(req, "token", body)
	if got := req.Header.Get("X-Initiator"); got != "agent" {
		t.Fatalf("X-Initiator = %q, want agent (Claude native tool_result under role=user)", got)
	}
}

func TestApplyHeaders_XInitiator_ClaudeNativeToolUse(t *testing.T) {
	t.Parallel()
	e := &GitHubCopilotExecutor{}
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
	body := []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]},{"role":"assistant","content":[{"type":"tool_use","id":"tu_1","name":"Read","input":{}}]}]}`)
	e.applyHeaders(req, "token", body)
	if got := req.Header.Get("X-Initiator"); got != "agent" {
		t.Fatalf("X-Initiator = %q, want agent (Claude native tool_use under role=assistant)", got)
	}
}

// --- Tests for subagent detection ---

func TestApplyHeaders_XInitiator_ClaudeCodeSubagent(t *testing.T) {
	t.Parallel()
	e := &GitHubCopilotExecutor{}
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
	body := []byte(`{"messages":[{"role":"system","content":"You are a file search specialist. READ-ONLY MODE - NO FILE MODIFICATIONS."},{"role":"user","content":"search for copilot"}]}`)
	e.applyHeaders(req, "token", body)
	if got := req.Header.Get("X-Initiator"); got != "agent" {
		t.Fatalf("X-Initiator = %q, want agent (subagent detected)", got)
	}
}

func TestApplyHeaders_XInitiator_ClaudeCodeSubagentArrayContent(t *testing.T) {
	t.Parallel()
	e := &GitHubCopilotExecutor{}
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
	body := []byte(`{"messages":[{"role":"system","content":[{"type":"text","text":"You are Claude Code."},{"type":"text","text":"This is a READ-ONLY exploration task. Agent threads always have their cwd reset between bash calls."}]},{"role":"user","content":"hello"}]}`)
	e.applyHeaders(req, "token", body)
	if got := req.Header.Get("X-Initiator"); got != "agent" {
		t.Fatalf("X-Initiator = %q, want agent (subagent with array content)", got)
	}
}

func TestApplyHeaders_XInitiator_AgentForNormalUserRequestWhenUserCountIsNotMultipleOfFive(t *testing.T) {
	t.Parallel()
	e := &GitHubCopilotExecutor{}
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
	body := []byte(`{"messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"hello"}]}`)
	e.applyHeaders(req, "token", body)
	if got := req.Header.Get("X-Initiator"); got != "agent" {
		t.Fatalf("X-Initiator = %q, want agent (user count is not a multiple of five)", got)
	}
}

func TestDetectSubagent_Positive(t *testing.T) {
	t.Parallel()
	body := []byte(`{"messages":[{"role":"system","content":"You are a file search specialist. READ-ONLY exploration task."},{"role":"user","content":"search"}]}`)
	if !detectSubagent(body) {
		t.Fatal("expected subagent to be detected")
	}
}

func TestDetectSubagent_PositiveWhenUserCountIsNotMultipleOfTen(t *testing.T) {
	t.Parallel()
	body := []byte(`{"messages":[{"role":"system","content":"You are a helpful coding assistant."},{"role":"user","content":"hello"}]}`)
	if !detectSubagent(body) {
		t.Fatal("expected subagent detection when user message count is not a multiple of ten")
	}
}

func TestDetectSubagent_NegativeWhenUserCountIsMultipleOfTen(t *testing.T) {
	t.Parallel()
	body := []byte(`{"messages":[{"role":"user","content":"u1"},{"role":"assistant","content":"a1"},{"role":"user","content":"u2"},{"role":"assistant","content":"a2"},{"role":"user","content":"u3"},{"role":"assistant","content":"a3"},{"role":"user","content":"u4"},{"role":"assistant","content":"a4"},{"role":"user","content":"u5"},{"role":"assistant","content":"a5"},{"role":"user","content":"u6"},{"role":"assistant","content":"a6"},{"role":"user","content":"u7"},{"role":"assistant","content":"a7"},{"role":"user","content":"u8"},{"role":"assistant","content":"a8"},{"role":"user","content":"u9"},{"role":"assistant","content":"a9"},{"role":"user","content":"u10"}]}`)
	if detectSubagent(body) {
		t.Fatal("expected no subagent detection when user message count is a multiple of ten")
	}
}

func TestDetectSubagent_NegativeEmpty(t *testing.T) {
	t.Parallel()
	if detectSubagent(nil) {
		t.Fatal("expected false for nil body")
	}
}

func TestDetectSubagent_IgnoresUserToolResultForCount(t *testing.T) {
	t.Parallel()
	body := []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"u1"}]},{"role":"assistant","content":"a1"},{"role":"user","content":[{"type":"text","text":"u2"}]},{"role":"assistant","content":"a2"},{"role":"user","content":[{"type":"text","text":"u3"}]},{"role":"assistant","content":"a3"},{"role":"user","content":[{"type":"text","text":"u4"}]},{"role":"assistant","content":"a4"},{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"README.md"}]},{"role":"assistant","content":"a5"},{"role":"user","content":[{"type":"text","text":"u5"}]},{"role":"assistant","content":"a6"},{"role":"user","content":[{"type":"text","text":"u6"}]},{"role":"assistant","content":"a7"},{"role":"user","content":[{"type":"text","text":"u7"}]},{"role":"assistant","content":"a8"},{"role":"user","content":[{"type":"text","text":"u8"}]},{"role":"assistant","content":"a9"},{"role":"user","content":[{"type":"text","text":"u9"}]},{"role":"assistant","content":"a10"},{"role":"user","content":[{"type":"text","text":"u10"}]}]}`)
	if detectSubagent(body) {
		t.Fatal("expected tool_result under role=user to be excluded from user counting")
	}
}

func TestDetectSubagent_IgnoresMixedUserToolResultForCount(t *testing.T) {
	t.Parallel()
	body := []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"u1"}]},{"role":"assistant","content":"a1"},{"role":"user","content":[{"type":"text","text":"u2"}]},{"role":"assistant","content":"a2"},{"role":"user","content":[{"type":"text","text":"u3"}]},{"role":"assistant","content":"a3"},{"role":"user","content":[{"type":"text","text":"u4"}]},{"role":"assistant","content":"a4"},{"role":"user","content":[{"type":"text","text":"user asked"},{"type":"tool_result","tool_use_id":"toolu_1","content":"README.md"}]},{"role":"assistant","content":"a5"},{"role":"user","content":[{"type":"text","text":"u5"}]},{"role":"assistant","content":"a6"},{"role":"user","content":[{"type":"text","text":"u6"}]},{"role":"assistant","content":"a7"},{"role":"user","content":[{"type":"text","text":"u7"}]},{"role":"assistant","content":"a8"},{"role":"user","content":[{"type":"text","text":"u8"}]},{"role":"assistant","content":"a9"},{"role":"user","content":[{"type":"text","text":"u9"}]},{"role":"assistant","content":"a10"},{"role":"user","content":[{"type":"text","text":"u10"}]}]}`)
	if detectSubagent(body) {
		t.Fatal("expected any tool_result under role=user to be excluded from user counting")
	}
}

func TestApplyHeaders_XInitiator_PrefersLastRoleBeforeSubagentHeuristic(t *testing.T) {
	t.Parallel()
	e := &GitHubCopilotExecutor{}
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
	body := []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]},{"role":"assistant","content":[{"type":"tool_use","id":"tu_1","name":"Read","input":{}}]}]}`)
	e.applyHeaders(req, "token", body)
	if got := req.Header.Get("X-Initiator"); got != "agent" {
		t.Fatalf("X-Initiator = %q, want agent (last role should win before subagent heuristic)", got)

	}
}

// --- Tests for x-github-api-version header (Problem M) ---

func TestApplyHeaders_GitHubAPIVersion(t *testing.T) {
	t.Parallel()
	e := &GitHubCopilotExecutor{}
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
	e.applyHeaders(req, "token", nil)
	if got := req.Header.Get("X-Github-Api-Version"); got != "2025-10-01" {
		t.Fatalf("X-Github-Api-Version = %q, want 2025-10-01", got)
	}
}

// --- Tests for vision detection (Problem P) ---

func TestDetectVisionContent_WithImageURL(t *testing.T) {
	t.Parallel()
	body := []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"describe"},{"type":"image_url","image_url":{"url":"data:image/png;base64,abc"}}]}]}`)
	if !detectVisionContent(body) {
		t.Fatal("expected vision content to be detected")
	}
}

func TestDetectVisionContent_WithImageType(t *testing.T) {
	t.Parallel()
	body := []byte(`{"messages":[{"role":"user","content":[{"type":"image","source":{"data":"abc","media_type":"image/png"}}]}]}`)
	if !detectVisionContent(body) {
		t.Fatal("expected image type to be detected")
	}
}

func TestDetectVisionContent_NoVision(t *testing.T) {
	t.Parallel()
	body := []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`)
	if detectVisionContent(body) {
		t.Fatal("expected no vision content")
	}
}

func TestDetectVisionContent_NoMessages(t *testing.T) {
	t.Parallel()
	// After Responses API normalization, messages is removed — detection should return false
	body := []byte(`{"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}]}`)
	if detectVisionContent(body) {
		t.Fatal("expected no vision content when messages field is absent")
	}
}

// --- Tests for Claude-family /v1/messages routing ---

func TestShouldUseGitHubCopilotClaudeMessages_ClaudeFamily(t *testing.T) {
	t.Parallel()
	tests := []struct {
		model string
		want  bool
	}{
		{"claude-opus-4.6", true},
		{"claude-sonnet-4.6", true},
		{"claude-haiku-4.5", true},
		{"claude-3-5-sonnet", true},
		{"claude-opus-4.6(high)", true},
		{"gpt-4o", false},
		{"gpt-5-codex", false},
		{"gemini-2.5-pro", false},
		{"o3", false},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			t.Parallel()
			got := shouldUseGitHubCopilotClaudeMessages(tt.model)
			if got != tt.want {
				t.Fatalf("shouldUseGitHubCopilotClaudeMessages(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

// --- Tests for applyGitHubCopilotResponsesDefaults ---

func TestApplyGitHubCopilotResponsesDefaults_SetsAllDefaults(t *testing.T) {
	t.Parallel()
	body := []byte(`{"input":"hello","reasoning":{"effort":"medium"}}`)
	got := applyGitHubCopilotResponsesDefaults(body)

	if gjson.GetBytes(got, "store").Bool() != false {
		t.Fatalf("store = %v, want false", gjson.GetBytes(got, "store").Raw)
	}
	inc := gjson.GetBytes(got, "include")
	if !inc.IsArray() || inc.Array()[0].String() != "reasoning.encrypted_content" {
		t.Fatalf("include = %s, want [\"reasoning.encrypted_content\"]", inc.Raw)
	}
	if gjson.GetBytes(got, "reasoning.summary").String() != "auto" {
		t.Fatalf("reasoning.summary = %q, want auto", gjson.GetBytes(got, "reasoning.summary").String())
	}
}

func TestApplyGitHubCopilotResponsesDefaults_DoesNotOverrideExisting(t *testing.T) {
	t.Parallel()
	body := []byte(`{"input":"hello","store":true,"include":["other"],"reasoning":{"effort":"high","summary":"concise"}}`)
	got := applyGitHubCopilotResponsesDefaults(body)

	if gjson.GetBytes(got, "store").Bool() != true {
		t.Fatalf("store should not be overridden, got %s", gjson.GetBytes(got, "store").Raw)
	}
	if gjson.GetBytes(got, "include").Array()[0].String() != "other" {
		t.Fatalf("include should not be overridden, got %s", gjson.GetBytes(got, "include").Raw)
	}
	if gjson.GetBytes(got, "reasoning.summary").String() != "concise" {
		t.Fatalf("reasoning.summary should not be overridden, got %q", gjson.GetBytes(got, "reasoning.summary").String())
	}
}

func TestApplyGitHubCopilotResponsesDefaults_NoReasoningEffort(t *testing.T) {
	t.Parallel()
	body := []byte(`{"input":"hello"}`)
	got := applyGitHubCopilotResponsesDefaults(body)

	if gjson.GetBytes(got, "store").Bool() != false {
		t.Fatalf("store = %v, want false", gjson.GetBytes(got, "store").Raw)
	}
	// reasoning.summary should NOT be set when reasoning.effort is absent
	if gjson.GetBytes(got, "reasoning.summary").Exists() {
		t.Fatalf("reasoning.summary should not be set when reasoning.effort is absent, got %q", gjson.GetBytes(got, "reasoning.summary").String())
	}
}

// --- Tests for normalizeGitHubCopilotReasoningField ---

func TestNormalizeReasoningField_NonStreaming(t *testing.T) {
	t.Parallel()
	data := []byte(`{"choices":[{"message":{"content":"hello","reasoning_text":"I think..."}}]}`)
	got := normalizeGitHubCopilotReasoningField(data)
	rc := gjson.GetBytes(got, "choices.0.message.reasoning_content").String()
	if rc != "I think..." {
		t.Fatalf("reasoning_content = %q, want %q", rc, "I think...")
	}
}

func TestNormalizeReasoningField_Streaming(t *testing.T) {
	t.Parallel()
	data := []byte(`{"choices":[{"delta":{"reasoning_text":"thinking delta"}}]}`)
	got := normalizeGitHubCopilotReasoningField(data)
	rc := gjson.GetBytes(got, "choices.0.delta.reasoning_content").String()
	if rc != "thinking delta" {
		t.Fatalf("reasoning_content = %q, want %q", rc, "thinking delta")
	}
}

func TestNormalizeReasoningField_PreservesExistingReasoningContent(t *testing.T) {
	t.Parallel()
	data := []byte(`{"choices":[{"message":{"reasoning_text":"old","reasoning_content":"existing"}}]}`)
	got := normalizeGitHubCopilotReasoningField(data)
	rc := gjson.GetBytes(got, "choices.0.message.reasoning_content").String()
	if rc != "existing" {
		t.Fatalf("reasoning_content = %q, want %q (should not overwrite)", rc, "existing")
	}
}

func TestNormalizeReasoningField_MultiChoice(t *testing.T) {
	t.Parallel()
	data := []byte(`{"choices":[{"message":{"reasoning_text":"thought-0"}},{"message":{"reasoning_text":"thought-1"}}]}`)
	got := normalizeGitHubCopilotReasoningField(data)
	rc0 := gjson.GetBytes(got, "choices.0.message.reasoning_content").String()
	rc1 := gjson.GetBytes(got, "choices.1.message.reasoning_content").String()
	if rc0 != "thought-0" {
		t.Fatalf("choices[0].reasoning_content = %q, want %q", rc0, "thought-0")
	}
	if rc1 != "thought-1" {
		t.Fatalf("choices[1].reasoning_content = %q, want %q", rc1, "thought-1")
	}
}

func TestNormalizeReasoningField_NoChoices(t *testing.T) {
	t.Parallel()
	data := []byte(`{"id":"chatcmpl-123"}`)
	got := normalizeGitHubCopilotReasoningField(data)
	if string(got) != string(data) {
		t.Fatalf("expected no change, got %s", string(got))
	}
}

func TestApplyHeaders_OpenAIIntentValue(t *testing.T) {
	t.Parallel()
	e := &GitHubCopilotExecutor{}
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
	e.applyHeaders(req, "token", nil)
	if got := req.Header.Get("Openai-Intent"); got != copilotOpenAIIntent {
		t.Fatalf("Openai-Intent = %q, want conversation-edits", got)
	}
}

// --- Tests for CountTokens (local tiktoken estimation) ---

func TestCountTokens_ReturnsPositiveCount(t *testing.T) {
	t.Parallel()
	e := &GitHubCopilotExecutor{}
	body := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"Hello, world!"}]}`)
	resp, err := e.CountTokens(context.Background(), nil, cliproxyexecutor.Request{
		Model:   "gpt-4o",
		Payload: body,
	}, cliproxyexecutor.Options{
		SourceFormat: sdktranslator.FromString("openai"),
	})
	if err != nil {
		t.Fatalf("CountTokens() error: %v", err)
	}
	if len(resp.Payload) == 0 {
		t.Fatal("CountTokens() returned empty payload")
	}
	// The response should contain a positive token count.
	tokens := gjson.GetBytes(resp.Payload, "usage.prompt_tokens").Int()
	if tokens <= 0 {
		t.Fatalf("expected positive token count, got %d", tokens)
	}
}

func TestCountTokens_ClaudeSourceFormatTranslates(t *testing.T) {
	t.Parallel()
	e := &GitHubCopilotExecutor{}
	body := []byte(`{"model":"claude-sonnet-4","messages":[{"role":"user","content":"Tell me a joke"}],"max_tokens":1024}`)
	resp, err := e.CountTokens(context.Background(), nil, cliproxyexecutor.Request{
		Model:   "claude-sonnet-4",
		Payload: body,
	}, cliproxyexecutor.Options{
		SourceFormat: sdktranslator.FromString("claude"),
	})
	if err != nil {
		t.Fatalf("CountTokens() error: %v", err)
	}
	// Claude source format → should get input_tokens in response
	inputTokens := gjson.GetBytes(resp.Payload, "input_tokens").Int()
	if inputTokens <= 0 {
		// Fallback: check usage.prompt_tokens (depends on translator registration)
		promptTokens := gjson.GetBytes(resp.Payload, "usage.prompt_tokens").Int()
		if promptTokens <= 0 {
			t.Fatalf("expected positive token count, got payload: %s", resp.Payload)
		}
	}
}

func TestCountTokens_EmptyPayload(t *testing.T) {
	t.Parallel()
	e := &GitHubCopilotExecutor{}
	resp, err := e.CountTokens(context.Background(), nil, cliproxyexecutor.Request{
		Model:   "gpt-4o",
		Payload: []byte(`{"model":"gpt-4o","messages":[]}`),
	}, cliproxyexecutor.Options{
		SourceFormat: sdktranslator.FromString("openai"),
	})
	if err != nil {
		t.Fatalf("CountTokens() error: %v", err)
	}
	tokens := gjson.GetBytes(resp.Payload, "usage.prompt_tokens").Int()
	// Empty messages should return 0 tokens.
	if tokens != 0 {
		t.Fatalf("expected 0 tokens for empty messages, got %d", tokens)
	}
}

func TestStripUnsupportedBetas_RemovesContext1M(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"claude-opus-4.6","betas":["interleaved-thinking-2025-05-14","context-1m-2025-08-07","claude-code-20250219"],"messages":[]}`)
	result := stripUnsupportedBetas(body)

	betas := gjson.GetBytes(result, "betas")
	if !betas.Exists() {
		t.Fatal("betas field should still exist after stripping")
	}
	for _, item := range betas.Array() {
		if item.String() == "context-1m-2025-08-07" {
			t.Fatal("context-1m-2025-08-07 should have been stripped")
		}
	}
	// Other betas should be preserved
	found := false
	for _, item := range betas.Array() {
		if item.String() == "interleaved-thinking-2025-05-14" {
			found = true
		}
	}
	if !found {
		t.Fatal("other betas should be preserved")
	}
}

func TestStripUnsupportedBetas_NoBetasField(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"gpt-4o","messages":[]}`)
	result := stripUnsupportedBetas(body)

	// Should be unchanged
	if string(result) != string(body) {
		t.Fatalf("body should be unchanged when no betas field exists, got %s", string(result))
	}
}

func TestStripUnsupportedBetas_MetadataBetas(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"claude-opus-4.6","metadata":{"betas":["context-1m-2025-08-07","other-beta"]},"messages":[]}`)
	result := stripUnsupportedBetas(body)

	betas := gjson.GetBytes(result, "metadata.betas")
	if !betas.Exists() {
		t.Fatal("metadata.betas field should still exist after stripping")
	}
	for _, item := range betas.Array() {
		if item.String() == "context-1m-2025-08-07" {
			t.Fatal("context-1m-2025-08-07 should have been stripped from metadata.betas")
		}
	}
	if betas.Array()[0].String() != "other-beta" {
		t.Fatal("other betas in metadata.betas should be preserved")
	}
}

func TestStripUnsupportedBetas_AllBetasStripped(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"claude-opus-4.6","betas":["context-1m-2025-08-07"],"messages":[]}`)
	result := stripUnsupportedBetas(body)

	betas := gjson.GetBytes(result, "betas")
	if betas.Exists() {
		t.Fatal("betas field should be deleted when all betas are stripped")
	}
}

func TestCopilotModelEntry_Limits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		capabilities map[string]any
		wantNil      bool
		wantPrompt   int
		wantOutput   int
		wantContext  int
	}{
		{
			name:         "nil capabilities",
			capabilities: nil,
			wantNil:      true,
		},
		{
			name:         "no limits key",
			capabilities: map[string]any{"family": "claude-opus-4.6"},
			wantNil:      true,
		},
		{
			name:         "limits is not a map",
			capabilities: map[string]any{"limits": "invalid"},
			wantNil:      true,
		},
		{
			name: "all zero values",
			capabilities: map[string]any{
				"limits": map[string]any{
					"max_context_window_tokens": float64(0),
					"max_prompt_tokens":         float64(0),
					"max_output_tokens":         float64(0),
				},
			},
			wantNil: true,
		},
		{
			name: "individual account limits (128K prompt)",
			capabilities: map[string]any{
				"limits": map[string]any{
					"max_context_window_tokens": float64(144000),
					"max_prompt_tokens":         float64(128000),
					"max_output_tokens":         float64(64000),
				},
			},
			wantNil:     false,
			wantPrompt:  128000,
			wantOutput:  64000,
			wantContext: 144000,
		},
		{
			name: "business account limits (168K prompt)",
			capabilities: map[string]any{
				"limits": map[string]any{
					"max_context_window_tokens": float64(200000),
					"max_prompt_tokens":         float64(168000),
					"max_output_tokens":         float64(32000),
				},
			},
			wantNil:     false,
			wantPrompt:  168000,
			wantOutput:  32000,
			wantContext: 200000,
		},
		{
			name: "partial limits (only prompt)",
			capabilities: map[string]any{
				"limits": map[string]any{
					"max_prompt_tokens": float64(128000),
				},
			},
			wantNil:    false,
			wantPrompt: 128000,
			wantOutput: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			entry := copilotauth.CopilotModelEntry{
				ID:           "claude-opus-4.6",
				Capabilities: tt.capabilities,
			}
			limits := entry.Limits()
			if tt.wantNil {
				if limits != nil {
					t.Fatalf("expected nil limits, got %+v", limits)
				}
				return
			}
			if limits == nil {
				t.Fatal("expected non-nil limits, got nil")
			}
			if limits.MaxPromptTokens != tt.wantPrompt {
				t.Errorf("MaxPromptTokens = %d, want %d", limits.MaxPromptTokens, tt.wantPrompt)
			}
			if limits.MaxOutputTokens != tt.wantOutput {
				t.Errorf("MaxOutputTokens = %d, want %d", limits.MaxOutputTokens, tt.wantOutput)
			}
			if tt.wantContext > 0 && limits.MaxContextWindowTokens != tt.wantContext {
				t.Errorf("MaxContextWindowTokens = %d, want %d", limits.MaxContextWindowTokens, tt.wantContext)

			}
		})
	}
}

func TestShouldUseGitHubCopilotClaudeMessages_DoesNotAffectResponsesEndpoint(t *testing.T) {
	t.Parallel()
	// A claude model should NOT trigger useGitHubCopilotResponsesEndpoint
	// because the caller should check shouldUseGitHubCopilotClaudeMessages first.
	// But useGitHubCopilotResponsesEndpoint itself doesn't know about Claude messages,
	// so verify GPT/Codex models still route correctly.
	if !useGitHubCopilotResponsesEndpoint(sdktranslator.FromString("openai-response"), "claude-opus-4.6") {
		t.Fatal("openai-response source format should still return true regardless of model")
	}
	if !useGitHubCopilotResponsesEndpoint(sdktranslator.FromString("openai"), "gpt-5-codex") {
		t.Fatal("codex model should still use /responses")
	}
}

// --- Tests for anthropic-beta header on Claude /v1/messages path ---

func TestApplyHeaders_ClaudeMessagesAnthropicBeta(t *testing.T) {
	t.Parallel()
	// Simulate the call-site pattern: applyHeaders + conditional anthropic-beta.
	e := &GitHubCopilotExecutor{}
	req, _ := http.NewRequest(http.MethodPost, "https://example.com/v1/messages", nil)
	e.applyHeaders(req, "token", nil)
	// At the call site, anthropic-beta is added when useClaudeMessages is true.
	req.Header.Set("Anthropic-Beta", copilotClaudeAnthrBeta)

	if got := req.Header.Get("Anthropic-Beta"); got != "advanced-tool-use-2025-11-20" {
		t.Fatalf("Anthropic-Beta = %q, want advanced-tool-use-2025-11-20", got)
	}
}

func TestApplyHeaders_NonClaudeNoAnthropicBeta(t *testing.T) {
	t.Parallel()
	e := &GitHubCopilotExecutor{}
	req, _ := http.NewRequest(http.MethodPost, "https://example.com/chat/completions", nil)
	e.applyHeaders(req, "token", nil)
	// Non-Claude path should NOT set anthropic-beta.
	if got := req.Header.Get("Anthropic-Beta"); got != "" {
		t.Fatalf("Anthropic-Beta should be empty for non-Claude path, got %q", got)
	}
}

func TestNormalizeGitHubCopilotClaudeMessagesBody_StripsContextManagement(t *testing.T) {
	t.Parallel()
	body := []byte(`{
		"model": "claude-opus-4.6",
		"messages": [{"role":"user","content":[{"type":"text","text":"hello"}]}],
		"context_management": [{"type":"compaction","compact_threshold":12000}]
	}`)

	got := normalizeGitHubCopilotClaudeMessagesBody(body)
	if gjson.GetBytes(got, "context_management").Exists() {
		t.Fatal("context_management should be removed for GitHub Copilot Claude messages compatibility")
	}
	if model := gjson.GetBytes(got, "model").String(); model != "claude-opus-4.6" {
		t.Fatalf("model = %q, want claude-opus-4.6", model)
	}
	if text := gjson.GetBytes(got, "messages.0.content.0.text").String(); text != "hello" {
		t.Fatalf("messages[0].content[0].text = %q, want hello", text)
	}
}

func TestNormalizeGitHubCopilotClaudeMessagesBody_NoContextManagement(t *testing.T) {
	t.Parallel()
	body := []byte(`{"model":"claude-sonnet-4.6","messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`)

	got := normalizeGitHubCopilotClaudeMessagesBody(body)
	if text := gjson.GetBytes(got, "messages.0.content.0.text").String(); text != "hi" {
		t.Fatalf("messages[0].content[0].text = %q, want hi", text)
	}
	if gjson.GetBytes(got, "context_management").Exists() {
		t.Fatal("context_management should remain absent")
	}
}

func TestNormalizeGitHubCopilotClaudeMessagesBody_StripsMetadata(t *testing.T) {
	t.Parallel()
	body := []byte(`{"model":"claude-opus-4.6","metadata":{"user_id":"abc"},"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`)

	got := normalizeGitHubCopilotClaudeMessagesBody(body)
	if gjson.GetBytes(got, "metadata").Exists() {
		t.Fatal("metadata should be removed for GitHub Copilot Claude messages compatibility")
	}
	if text := gjson.GetBytes(got, "messages.0.content.0.text").String(); text != "hello" {
		t.Fatalf("messages[0].content[0].text = %q, want hello", text)
	}
}

func TestNormalizeGitHubCopilotClaudeMessagesBody_StripsCacheControlScope(t *testing.T) {
	t.Parallel()
	body := []byte(`{
		"model": "claude-opus-4.6",
		"system": [
			{"type":"text","text":"billing"},
			{"type":"text","text":"instructions","cache_control":{"type":"ephemeral","scope":"global"}}
		],
		"messages": [{"role":"user","content":[{"type":"text","text":"hello"}]}]
	}`)

	got := normalizeGitHubCopilotClaudeMessagesBody(body)
	if gjson.GetBytes(got, "system.1.cache_control.scope").Exists() {
		t.Fatal("system[1].cache_control.scope should be removed for GitHub Copilot Claude messages compatibility")
	}
	if cacheType := gjson.GetBytes(got, "system.1.cache_control.type").String(); cacheType != "ephemeral" {
		t.Fatalf("system[1].cache_control.type = %q, want ephemeral", cacheType)
	}
	if text := gjson.GetBytes(got, "system.1.text").String(); text != "instructions" {
		t.Fatalf("system[1].text = %q, want instructions", text)
	}
}

// --- Tests for Claude usage parsing ---

func TestParseClaudeUsage_BasicUsage(t *testing.T) {
	t.Parallel()
	data := []byte(`{"usage":{"input_tokens":100,"output_tokens":50,"cache_read_input_tokens":10}}`)
	detail := parseClaudeUsage(data)
	if detail.InputTokens != 100 {
		t.Fatalf("InputTokens = %d, want 100", detail.InputTokens)
	}
	if detail.OutputTokens != 50 {
		t.Fatalf("OutputTokens = %d, want 50", detail.OutputTokens)
	}
	if detail.CachedTokens != 10 {
		t.Fatalf("CachedTokens = %d, want 10", detail.CachedTokens)
	}
	if detail.TotalTokens != 150 {
		t.Fatalf("TotalTokens = %d, want 150", detail.TotalTokens)
	}
}

func TestParseClaudeUsage_NoUsage(t *testing.T) {
	t.Parallel()
	data := []byte(`{"content":[{"type":"text","text":"hello"}]}`)
	detail := parseClaudeUsage(data)
	if detail.TotalTokens != 0 {
		t.Fatalf("TotalTokens = %d, want 0 for missing usage", detail.TotalTokens)
	}
}

func TestParseClaudeStreamUsage_MessageDelta(t *testing.T) {
	t.Parallel()
	line := []byte(`data: {"type":"message_delta","usage":{"input_tokens":200,"output_tokens":80}}`)
	detail, ok := parseClaudeStreamUsage(line)
	if !ok {
		t.Fatal("expected parseClaudeStreamUsage to return ok=true")
	}
	if detail.TotalTokens != 280 {
		t.Fatalf("TotalTokens = %d, want 280", detail.TotalTokens)
	}
}

func TestParseClaudeStreamUsage_NoUsage(t *testing.T) {
	t.Parallel()
	line := []byte(`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"hi"}}`)
	_, ok := parseClaudeStreamUsage(line)
	if ok {
		t.Fatal("expected parseClaudeStreamUsage to return ok=false for content_block_delta without usage")
	}
}
