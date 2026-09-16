package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
type apiMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type thinkingPayload struct {
	Type   string `json:"type"`
	Effort string `json:"effort,omitempty"`
}
type apiReq struct {
	Model       string           `json:"model"`
	Messages    []apiMsg         `json:"messages"`
	Temperature float64          `json:"temperature,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Stream      bool             `json:"stream"`
	Thinking    *thinkingPayload `json:"thinking,omitempty"`
}
type apiResp struct {
	Choices []struct {
		Message struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage Usage `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    any    `json:"code"`
	} `json:"error,omitempty"`
}

func thinkingFor(mode string) *thinkingPayload {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "low":
		return &thinkingPayload{Type: "enabled", Effort: "low"}
	case "high":
		return &thinkingPayload{Type: "enabled", Effort: "high"}
	case "max":
		return &thinkingPayload{Type: "enabled", Effort: "max"}
	case "开启", "enabled":
		return &thinkingPayload{Type: "enabled"}
	default:
		return &thinkingPayload{Type: "disabled"}
	}
}
func maxTokensForTarget(n int) int {
	if n <= 0 {
		n = 3500
	}
	v := n * 2
	if v < 1024 {
		v = 1024
	}
	if v > 24000 {
		v = 24000
	}
	return v
}

func callDeepSeek(ctx context.Context, client *http.Client, s APISettings, key, system, user string, maxTokens int) (string, Usage, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", Usage{}, errors.New("请先在设置页填写 DeepSeek API Key")
	}
	endpoint := strings.TrimSpace(s.Endpoint)
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	model := strings.TrimSpace(s.Model)
	if model == "" {
		model = "deepseek-chat"
	}
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Minute}
	}
	reqBody := apiReq{Model: model, Messages: []apiMsg{{Role: "system", Content: system}, {Role: "user", Content: user}}, Temperature: 0.82, MaxTokens: maxTokens, Stream: false, Thinking: thinkingFor(s.Thinking)}
	raw, e := json.Marshal(reqBody)
	if e != nil {
		return "", Usage{}, e
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if e != nil {
		return "", Usage{}, e
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("User-Agent", "XinyueNovel/"+AppVersion)
	resp, e := client.Do(req)
	if e != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return "", Usage{}, errors.New("已停止生成")
		}
		return "", Usage{}, fmt.Errorf("网络请求失败：%w", e)
	}
	defer resp.Body.Close()
	body, e := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if e != nil {
		return "", Usage{}, e
	}
	var ar apiResp
	_ = json.Unmarshal(body, &ar)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if ar.Error != nil && strings.TrimSpace(ar.Error.Message) != "" {
			msg = ar.Error.Message
		}
		if len(msg) > 800 {
			msg = msg[:800] + "…"
		}
		return "", ar.Usage, fmt.Errorf("DeepSeek API 错误（HTTP %d）：%s", resp.StatusCode, msg)
	}
	if e = json.Unmarshal(body, &ar); e != nil {
		return "", Usage{}, fmt.Errorf("API 返回解析失败：%w", e)
	}
	if len(ar.Choices) == 0 {
		return "", ar.Usage, errors.New("API 没有返回 choices")
	}
	text := strings.TrimSpace(ar.Choices[0].Message.Content)
	if text == "" {
		return "", ar.Usage, errors.New("API 返回正文为空。可尝试关闭思考模式、增加输出长度或切换模型。")
	}
	return text, ar.Usage, nil
}

func GenerateNovel(ctx context.Context, client *http.Client, s APISettings, key string, r NovelRequest) (string, Usage, error) {
	sys, user := BuildNovelPrompt(r)
	return callDeepSeek(ctx, client, s, key, sys, user, maxTokensForTarget(r.TargetChars))
}
func TestAPI(ctx context.Context, client *http.Client, s APISettings, key string) (string, error) {
	ss := s
	ss.Thinking = "关闭"
	text, _, e := callDeepSeek(ctx, client, ss, key, "你是API连接测试助手。", "只回复四个字：连接成功", 128)
	if e != nil {
		return "", e
	}
	return text, nil
}
func GenerateOutline(ctx context.Context, client *http.Client, s APISettings, key string, p Project) (string, error) {
	sys := "你是中文网文策划编辑。请根据用户设定输出可直接用于写作的总纲与卷纲，结构清晰、冲突递进、避免空泛。"
	user := fmt.Sprintf("项目：%s\n类型：%s\n流派：%s\n文风：%s\n主角：%s\n剧情脉络：%s\n世界观：%s\n境界：%s\n请输出总纲、主要人物弧线、核心冲突、分卷规划与关键转折。", p.Name, p.Genre, p.Flow, p.Style, p.Protagonist, p.Plot, p.World, p.Realms)
	t, _, e := callDeepSeek(ctx, client, s, key, sys, user, 5000)
	return t, e
}
func GenerateRealms(ctx context.Context, client *http.Client, s APISettings, key string, p Project) (string, error) {
	sys := "你是玄幻/仙侠世界观设计师。请设计简洁、递进清晰、能支撑长期网文升级的境界体系。"
	user := fmt.Sprintf("小说类型：%s\n流派：%s\n世界观：%s\n剧情脉络：%s\n请生成境界名称、每境界核心变化、寿命/战力或能力边界，以及跨境界差异。", p.Genre, p.Flow, p.World, p.Plot)
	t, _, e := callDeepSeek(ctx, client, s, key, sys, user, 3500)
	return t, e
}
