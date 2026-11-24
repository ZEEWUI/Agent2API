package app

import (
	"io"
	"log"
	"testing"

	"github.com/576469377/Agent2API/internal/config"
	"github.com/576469377/Agent2API/internal/llm"
)

func systemPromptTestApp(t *testing.T, prompt, mode string) *App {
	t.Helper()
	cfg := config.Default()
	cfg.MetricsFile = ""
	cfg.Gateway.SystemPrompt = prompt
	cfg.Gateway.SystemPromptMode = mode
	logger := log.New(io.Discard, "", 0)
	hub, err := NewHub([]*PlatformRuntime{{ID: "fake", Adapter: &fakeAdapter{}}}, logger)
	if err != nil {
		t.Fatalf("构造 hub: %v", err)
	}
	return New(cfg, hub, logger)
}

// 三种合并模式的纯函数语义。
func TestGatewaySystemPromptMerge(t *testing.T) {
	cases := []struct {
		name, prompt, mode, base, want string
	}{
		{"默认空配置不改写", "", "prepend", "CLIENT", "CLIENT"},
		{"prepend", "GW", "prepend", "CLIENT", "GW\n\nCLIENT"},
		{"prepend 无客户端", "GW", "prepend", "", "GW"},
		{"append", "GW", "append", "CLIENT", "CLIENT\n\nGW"},
		{"append 无客户端", "GW", "append", "", "GW"},
		{"replace", "GW", "replace", "CLIENT", "GW"},
		{"未知模式按 prepend", "GW", "bogus", "CLIENT", "GW\n\nCLIENT"},
		{"首尾空白视为未配置", "   ", "replace", "CLIENT", "CLIENT"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := config.GatewayConfig{SystemPrompt: tc.prompt, SystemPromptMode: tc.mode}
			if got := g.SystemPromptMerge(tc.base); got != tc.want {
				t.Fatalf("SystemPromptMerge(%q)=%q want %q", tc.base, got, tc.want)
			}
		})
	}
}

// 未配置时请求必须逐字段原样返回，保证默认行为与上线前一致。
func TestApplyGatewaySystemPromptNoop(t *testing.T) {
	a := systemPromptTestApp(t, "", "")
	req := llm.RequestMessages{
		Model:        "m",
		SystemPrompt: "CLIENT",
		SystemBlocks: []llm.Content{{Type: llm.ContentText, Text: "CLIENT"}},
	}
	got := a.applyGatewaySystemPrompt(req)
	if got.SystemPrompt != "CLIENT" || len(got.SystemBlocks) != 1 || got.SystemBlocks[0].Text != "CLIENT" {
		t.Fatalf("空配置改写了请求: %+v", got)
	}
}

// SystemBlocks 非空时（Anthropic 分块缓存）也必须生效，否则优先读 blocks 的适配器会忽略提示词。
func TestApplyGatewaySystemPromptUpdatesBlocks(t *testing.T) {
	base := []llm.Content{{Type: llm.ContentText, Text: "CLIENT"}}

	prepended := systemPromptTestApp(t, "GW", "prepend").applyGatewaySystemPrompt(
		llm.RequestMessages{SystemPrompt: "CLIENT", SystemBlocks: base})
	if prepended.SystemPrompt != "GW\n\nCLIENT" {
		t.Fatalf("SystemPrompt=%q", prepended.SystemPrompt)
	}
	if len(prepended.SystemBlocks) != 2 || prepended.SystemBlocks[0].Text != "GW" || prepended.SystemBlocks[1].Text != "CLIENT" {
		t.Fatalf("prepend blocks=%+v", prepended.SystemBlocks)
	}
	if len(base) != 1 {
		t.Fatalf("入参 slice 被就地修改: %+v", base)
	}

	appended := systemPromptTestApp(t, "GW", "append").applyGatewaySystemPrompt(
		llm.RequestMessages{SystemPrompt: "CLIENT", SystemBlocks: base})
	if len(appended.SystemBlocks) != 2 || appended.SystemBlocks[0].Text != "CLIENT" || appended.SystemBlocks[1].Text != "GW" {
		t.Fatalf("append blocks=%+v", appended.SystemBlocks)
	}

	replaced := systemPromptTestApp(t, "GW", "replace").applyGatewaySystemPrompt(
		llm.RequestMessages{SystemPrompt: "CLIENT", SystemBlocks: base})
	if replaced.SystemPrompt != "GW" || replaced.SystemBlocks != nil {
		t.Fatalf("replace=%+v blocks=%+v", replaced.SystemPrompt, replaced.SystemBlocks)
	}
}

// 无 blocks 时只改 SystemPrompt，不动其它字段。
func TestApplyGatewaySystemPromptTextOnly(t *testing.T) {
	a := systemPromptTestApp(t, "GW", "prepend")
	got := a.applyGatewaySystemPrompt(llm.RequestMessages{Model: "m", SystemPrompt: "CLIENT"})
	if got.Model != "m" || got.SystemPrompt != "GW\n\nCLIENT" {
		t.Fatalf("got=%+v", got)
	}
}
