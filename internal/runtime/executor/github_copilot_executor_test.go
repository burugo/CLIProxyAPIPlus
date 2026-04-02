package executor

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
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
	t.Parallel()
	if !useGitHubCopilotResponsesEndpoint(sdktranslator.FromString("openai"), "gpt-5.4") {
		t.Fatal("expected responses-only registry model to use /responses")
	}
	if !useGitHubCopilotResponsesEndpoint(sdktranslator.FromString("openai"), "gpt-5.4-mini") {
		t.Fatal("expected responses-only registry model to use /responses")
	}
}

func TestUseGitHubCopilotResponsesEndpoint_DynamicRegistryWinsOverStatic(t *testing.T) {
	t.Parallel()

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

func TestNormalizeGitHubCopilotClaudeThinking_DefaultsToHighForAdaptiveThinking(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"claude-sonnet-4.6","thinking":{"type":"adaptive"}}`)
	got := normalizeGitHubCopilotClaudeThinking("claude-sonnet-4.6", body, nil)

	if effort := gjson.GetBytes(got, "output_config.effort").String(); effort != "high" {
		t.Fatalf("output_config.effort = %q, want high", effort)
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

func TestApplyHeaders_XInitiator_AgentWhenLastRoleIsUserButUserCountIsNotMultipleOfFive(t *testing.T) {
	t.Parallel()
	e := &GitHubCopilotExecutor{}
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
	body := []byte(`{"messages":[{"role":"user","content":"hello"},{"role":"assistant","content":"I will read the file"},{"role":"user","content":"tool result here"}]}`)
	e.applyHeaders(req, "token", body)
	if got := req.Header.Get("X-Initiator"); got != "agent" {
		t.Fatalf("X-Initiator = %q, want agent (user count is not a multiple of five)", got)
	}
}

func TestApplyHeaders_XInitiator_AgentWithToolRole(t *testing.T) {
	t.Parallel()
	e := &GitHubCopilotExecutor{}
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
	body := []byte(`{"messages":[{"role":"user","content":"hello"},{"role":"tool","content":"result"}]}`)
	e.applyHeaders(req, "token", body)
	if got := req.Header.Get("X-Initiator"); got != "agent" {
		t.Fatalf("X-Initiator = %q, want agent (tool role exists)", got)
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

func TestApplyHeaders_XInitiator_InputArrayLastUserMessage(t *testing.T) {
	t.Parallel()
	e := &GitHubCopilotExecutor{}
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
	body := []byte(`{"input":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"I can help"}]},{"type":"message","role":"user","content":[{"type":"input_text","text":"Do X"}]}]}`)
	e.applyHeaders(req, "token", body)
	if got := req.Header.Get("X-Initiator"); got != "user" {
		t.Fatalf("X-Initiator = %q, want user (last role is user)", got)
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

func TestDetectSubagent_PositiveWhenUserCountIsNotMultipleOfFive(t *testing.T) {
	t.Parallel()
	body := []byte(`{"messages":[{"role":"system","content":"You are a helpful coding assistant."},{"role":"user","content":"hello"}]}`)
	if !detectSubagent(body) {
		t.Fatal("expected subagent detection when user message count is not a multiple of five")
	}
}

func TestDetectSubagent_NegativeWhenUserCountIsMultipleOfFive(t *testing.T) {
	t.Parallel()
	body := []byte(`{"messages":[{"role":"user","content":"u1"},{"role":"assistant","content":"a1"},{"role":"user","content":"u2"},{"role":"assistant","content":"a2"},{"role":"user","content":"u3"},{"role":"assistant","content":"a3"},{"role":"user","content":"u4"},{"role":"assistant","content":"a4"},{"role":"user","content":"u5"}]}`)
	if detectSubagent(body) {
		t.Fatal("expected no subagent detection when user message count is a multiple of five")
	}
}

func TestDetectSubagent_NegativeEmpty(t *testing.T) {
	t.Parallel()
	if detectSubagent(nil) {
		t.Fatal("expected false for nil body")
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
