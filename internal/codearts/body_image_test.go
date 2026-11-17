package codearts

// 2026-11-17：CodeArts 出站内容块过滤——图片放行必须按模型、且默认仍丢弃。
// 取证见 body.go 的 imageCapableModels 注释（直连上游单发 A/B 实测）。

import (
	"encoding/json"
	"strings"
	"testing"
)

const imgBlockUser = `{"model":"%s","messages":[{"role":"user","content":[` +
	`{"type":"text","text":"图里是什么数字"},` +
	`{"type":"image_url","image_url":{"url":"data:image/png;base64,AAAA"}}]}]}`

// contentTypes 取出第 0 条消息 content 数组里各块的 type。
func contentTypes(t *testing.T, out []byte) []string {
	t.Helper()
	var obj map[string]any
	if err := json.Unmarshal(out, &obj); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	msgs, _ := obj["messages"].([]any)
	if len(msgs) == 0 {
		t.Fatal("messages 为空")
	}
	blocks, _ := msgs[0].(map[string]any)["content"].([]any)
	types := make([]string, 0, len(blocks))
	for _, b := range blocks {
		blk, _ := b.(map[string]any)
		tp, _ := blk["type"].(string)
		types = append(types, tp)
	}
	return types
}

func hasType(types []string, want string) bool {
	for _, tp := range types {
		if tp == want {
			return true
		}
	}
	return false
}

// TestImageBlockKeptForVisionModel 可识图模型必须保留 image_url 块。
func TestImageBlockKeptForVisionModel(t *testing.T) {
	in := []byte(strings.Replace(imgBlockUser, "%s", "deepseek-v4.1-flash", 1))
	out, err := buildChatBody(in, "deepseek-v4.1-flash", "s1")
	if err != nil {
		t.Fatal(err)
	}
	types := contentTypes(t, out)
	if !hasType(types, "image_url") {
		t.Fatalf("deepseek-v4.1-flash 必须保留 image_url，实际块 = %v", types)
	}
	if !hasType(types, "text") {
		t.Fatalf("text 块不得被丢，实际块 = %v", types)
	}
}

// TestImageBlockDroppedForTextModel 反向护栏：纯文本模型必须继续丢弃图片块。
func TestImageBlockDroppedForTextModel(t *testing.T) {
	in := []byte(strings.Replace(imgBlockUser, "%s", "GLM-5.2", 1))
	out, err := buildChatBody(in, "GLM-5.2", "s1")
	if err != nil {
		t.Fatal(err)
	}
	types := contentTypes(t, out)
	if hasType(types, "image_url") {
		t.Fatalf("GLM-5.2 是纯文本模型，image_url 必须丢弃，实际块 = %v", types)
	}
	if !hasType(types, "text") {
		t.Fatalf("text 块不得被丢，实际块 = %v", types)
	}
}

// TestImageModelMatchIgnoresChannelPrefix 带渠道前缀的模型名也要命中白名单。
func TestImageModelMatchIgnoresChannelPrefix(t *testing.T) {
	in := []byte(strings.Replace(imgBlockUser, "%s", "huawei/deepseek-v4.1-flash", 1))
	out, err := buildChatBody(in, "huawei/deepseek-v4.1-flash", "s1")
	if err != nil {
		t.Fatal(err)
	}
	if types := contentTypes(t, out); !hasType(types, "image_url") {
		t.Fatalf("前缀模型名必须命中白名单，实际块 = %v", types)
	}
}

// TestInputImageAliasKept input_image 别名同样按模型放行（Responses 系客户端形态）。
func TestInputImageAliasKept(t *testing.T) {
	in := []byte(`{"model":"deepseek-v4.1-flash","messages":[{"role":"user","content":[` +
		`{"type":"text","text":"x"},{"type":"input_image","image_url":"data:image/png;base64,AAAA"}]}]}`)
	out, err := buildChatBody(in, "deepseek-v4.1-flash", "s1")
	if err != nil {
		t.Fatal(err)
	}
	if types := contentTypes(t, out); !hasType(types, "input_image") {
		t.Fatalf("input_image 必须放行，实际块 = %v", types)
	}
}
