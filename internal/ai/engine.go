// internal/ai/engine.go
// РљР»РёРµРЅС‚ Рє Р»РѕРєР°Р»СЊРЅРѕРјСѓ AI Р±СЌРєРµРЅРґСѓ (llamafile, LM Studio РёР»Рё Ollama).
// AI Р·Р°РґР°С‡Р°: РіРµРЅРµСЂРёСЂРѕРІР°С‚СЊ JSON РїРѕСЃР»РµРґРѕРІР°С‚РµР»СЊРЅРѕСЃС‚Рё URL РґР»СЏ cover traffic.
package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yourorg/furor-davidis/internal/logger"
	"github.com/yourorg/furor-davidis/internal/memory"
)

// DefaultModel вЂ” СЂРµРєРѕРјРµРЅРґСѓРµРјР°СЏ РјРѕРґРµР»СЊ (РјР°Р»РµРЅСЊРєР°СЏ, Р±С‹СЃС‚СЂР°СЏ, С…РѕСЂРѕС€Рѕ СЃР»РµРґСѓРµС‚ РёРЅСЃС‚СЂСѓРєС†РёСЏРј).
const DefaultModel = "lmstudio-community/Qwen2.5-1.5B-Instruct-GGUF"
const maxSystemPromptChars = 700

// CoverItem вЂ” РѕРґРёРЅ URL СЃ Р·Р°РґРµСЂР¶РєРѕР№ С‡С‚РµРЅРёСЏ.
type CoverItem struct {
	URL     string `json:"url"`
	Referer string `json:"referer,omitempty"`
	ReadSec int    `json:"read_sec"`
}

// Request вЂ” РІС…РѕРґРЅС‹Рµ РґР°РЅРЅС‹Рµ РґР»СЏ AI.
type Request struct {
	Profile       string   `json:"profile"`
	SessionMin    int      `json:"session_min"`
	Sites         []string `json:"sites"`
	RTTTrend      string   `json:"rtt_trend"`
	Hour          int      `json:"hour"`
	DayType       string   `json:"day_type"`
	MemoryContext string   `json:"-"`
	SystemPrompt  string   `json:"-"`
	URLCount      int      `json:"-"`
	ChainDepth    int      `json:"-"`
	SiteCap       int      `json:"-"`
	RAGWeight     float64  `json:"-"`
}

// EngineConfig вЂ” РєРѕРЅС„РёРіСѓСЂР°С†РёСЏ AI РґРІРёР¶РєР°.
type EngineConfig struct {
	LMStudioModel string // HuggingFace РёРґРµРЅС‚РёС„РёРєР°С‚РѕСЂ РёР»Рё РёРјСЏ РјРѕРґРµР»Рё; "" = DefaultModel
	BaseURL       string // URL СЃРµСЂРІРµСЂР° (РЅР°РїСЂРёРјРµСЂ, http://localhost:11434 РёР»Рё :1234)
}

// Engine вЂ” AI РґРІРёР¶РѕРє (LM Studio).
type Engine struct {
	log     *logger.Logger
	mu      sync.Mutex
	client  *http.Client
	baseURL string
	model   string
	ready   bool
}

// OpenAI-СЃРѕРІРјРµСЃС‚РёРјС‹Р№ chat completion С„РѕСЂРјР°С‚.
type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func NewEngine(log *logger.Logger) *Engine {
	return &Engine{
		log:    log,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// Start РїРѕРґРєР»СЋС‡Р°РµС‚СЃСЏ Рє LM Studio, РїСЂРё РЅРµРѕР±С…РѕРґРёРјРѕСЃС‚Рё Р·Р°РіСЂСѓР¶Р°РµС‚ РјРѕРґРµР»СЊ.
func (e *Engine) Start(cfg EngineConfig) error {
	e.baseURL = cfg.BaseURL
	if e.baseURL == "" {
		e.baseURL = "http://localhost:1234" // LM Studio default local server
	}

	wantModel := cfg.LMStudioModel
	if wantModel == "" {
		wantModel = DefaultModel
	}

	// РџСЂРѕРІРµСЂСЏРµРј С‡С‚Рѕ СЃРµСЂРІРµСЂ Р·Р°РїСѓС‰РµРЅ
	if _, err := e.client.Get(e.baseURL + "/v1/models"); err != nil {
		return fmt.Errorf("AI server is not reachable at %s: %w", e.baseURL, err)
	}

	loaded := e.getLoadedModels(e.baseURL)
	if cfg.LMStudioModel != "" {
		// РЇРІРЅРѕ РІС‹Р±СЂР°РЅРЅР°СЏ РјРѕРґРµР»СЊ РёРјРµРµС‚ РїСЂРёРѕСЂРёС‚РµС‚ РЅР°Рґ "РїРµСЂРІРѕР№ РІ СЃРїРёСЃРєРµ".
		if containsModel(loaded, wantModel) {
			e.model = wantModel
			e.ready = true
			e.log.Infof("[AI] LM Studio ready, selected model: %s", wantModel)
			return nil
		}

		e.log.Infof("[AI] Loading selected model: %s", wantModel)
		if err := e.loadModel(e.baseURL, wantModel); err != nil {
			return fmt.Errorf("failed to load selected model %q: %w", wantModel, err)
		}
		if !e.waitForModel(wantModel, 60*time.Second) {
			return fmt.Errorf("model %q did not become available within 60s", wantModel)
		}
		e.model = wantModel
		e.ready = true
		e.log.Infof("[AI] LM Studio ready, selected model: %s", wantModel)
		return nil
	}

	// Р’ Р°РІС‚Рѕ-СЂРµР¶РёРјРµ РёСЃРїРѕР»СЊР·СѓРµРј РїРµСЂРІСѓСЋ СѓР¶Рµ Р·Р°РіСЂСѓР¶РµРЅРЅСѓСЋ РјРѕРґРµР»СЊ.
	if len(loaded) > 0 {
		e.model = loaded[0]
		e.ready = true
		e.log.Infof("[AI] LM Studio ready, using loaded model: %s", loaded[0])
		return nil
	}

	// Р•СЃР»Рё РЅРёС‡РµРіРѕ РЅРµ Р·Р°РіСЂСѓР¶РµРЅРѕ вЂ” РїСЂРѕР±СѓРµРј СЂРµРєРѕРјРµРЅРґРѕРІР°РЅРЅСѓСЋ РјРѕРґРµР»СЊ.
	e.log.Infof("[AI] No model loaded, trying: %s", wantModel)
	if err := e.loadModel(e.baseURL, wantModel); err != nil {
		e.log.Infof("[AI] WARN load: %v", err)
		e.log.Infof("[AI] Open LM Studio, load a model, then press Start Server")
		return fmt.Errorf("model is not loaded: open LM Studio and load a model")
	}
	if !e.waitForModel(wantModel, 60*time.Second) {
		e.log.Infof("[AI] WARN: model was not ready within 60s, AI disabled")
		return fmt.Errorf("model %q did not become available within 60s", wantModel)
	}

	e.model = wantModel
	e.ready = true
	e.log.Infof("[AI] LM Studio ready, model: %s", wantModel)
	return nil
}

// ListModels РІРѕР·РІСЂР°С‰Р°РµС‚ СЃРїРёСЃРѕРє Р·Р°РіСЂСѓР¶РµРЅРЅС‹С… РІ LM Studio РјРѕРґРµР»РµР№.
func (e *Engine) ListModels(baseURL string) ([]string, error) {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "http://localhost:1234"
	}
	resp, err := e.client.Get(baseURL + "/v1/models")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		if strings.TrimSpace(m.ID) != "" {
			out = append(out, m.ID)
		}
	}
	return out, nil
}

// getLoadedModels РІРѕР·РІСЂР°С‰Р°РµС‚ СЃРїРёСЃРѕРє Р·Р°РіСЂСѓР¶РµРЅРЅС‹С… РјРѕРґРµР»РµР№ РёР»Рё РїСѓСЃС‚РѕР№ СЃРїРёСЃРѕРє.
func (e *Engine) getLoadedModels(baseURL string) []string {
	resp, err := e.client.Get(baseURL + "/v1/models")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || len(result.Data) == 0 {
		return nil
	}
	out := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		if strings.TrimSpace(m.ID) != "" {
			out = append(out, m.ID)
		}
	}
	return out
}

// loadModel Р·Р°РїСЂР°С€РёРІР°РµС‚ LM Studio Р·Р°РіСЂСѓР·РёС‚СЊ РјРѕРґРµР»СЊ С‡РµСЂРµР· РЅР°С‚РёРІРЅС‹Р№ API.
func (e *Engine) loadModel(baseURL, model string) error {
	payload, _ := json.Marshal(map[string]string{"identifier": model})
	resp, err := e.client.Post(
		baseURL+"/api/v0/models/load",
		"application/json",
		bytes.NewReader(payload),
	)
	if err != nil {
		return fmt.Errorf("load request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("load failed %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (e *Engine) waitForModel(model string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(3 * time.Second)
		if containsModel(e.getLoadedModels(e.baseURL), model) {
			return true
		}
	}
	return false
}

func containsModel(models []string, want string) bool {
	for _, m := range models {
		if m == want {
			return true
		}
	}
	return false
}

// Stop вЂ” LM Studio СЃРёСЃС‚РµРјРЅС‹Р№, РЅРµ РѕСЃС‚Р°РЅР°РІР»РёРІР°РµРј.
func (e *Engine) Stop() {
	e.ready = false
}

// IsReady РІРѕР·РІСЂР°С‰Р°РµС‚ true РµСЃР»Рё РјРѕРґРµР»СЊ РіРѕС‚РѕРІР° Рє РёРЅС„РµСЂРµРЅСЃСѓ.
func (e *Engine) IsReady() bool { return e.ready }

// Backend РІРѕР·РІСЂР°С‰Р°РµС‚ Р°РєС‚РёРІРЅС‹Р№ Р±СЌРєРµРЅРґ.
func (e *Engine) Backend() string {
	if e.ready {
		return "lmstudio"
	}
	return ""
}

// Generate РіРµРЅРµСЂРёСЂСѓРµС‚ РїРѕСЃР»РµРґРѕРІР°С‚РµР»СЊРЅРѕСЃС‚СЊ URL РґР»СЏ cover traffic.
func (e *Engine) Generate(req Request, mem *memory.Store) ([]CoverItem, error) {
	if !e.ready {
		return nil, fmt.Errorf("AI backend is not ready")
	}

	if !e.mu.TryLock() {
		return nil, fmt.Errorf("AI engine is busy with previous request")
	}
	defer e.mu.Unlock()

	memCtx := mem.BuildPromptContext(memory.Context{
		Hour:      req.Hour,
		DayType:   req.DayType,
		RTTTrend:  req.RTTTrend,
		Profile:   req.Profile,
		Intensity: "medium",
	})
	memCtx = applyRAGWeight(memCtx, req.RAGWeight)

	prompt := buildPrompt(req, memCtx)
	sysPrompt := EffectiveSystemPrompt(req.SystemPrompt)

	body, err := json.Marshal(chatRequest{
		Model: e.model,
		Messages: []chatMessage{
			{Role: "system", Content: sysPrompt},
			{Role: "user", Content: prompt},
		},
		Stream: false,
	})
	if err != nil {
		return nil, err
	}

	resp, err := e.client.Post(
		e.baseURL+"/v1/chat/completions",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("AI request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var cr chatResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if len(cr.Choices) == 0 {
		return nil, fmt.Errorf("empty response from model")
	}

	content := cr.Choices[0].Message.Content
	start := bytes.IndexByte([]byte(content), '[')
	end := bytes.LastIndexByte([]byte(content), ']')
	if start < 0 || end < 0 || end <= start {
		return nil, fmt.Errorf("JSON array not found in model response: %s", content[:min(100, len(content))])
	}
	var items []CoverItem
	if err := json.Unmarshal([]byte(content[start:end+1]), &items); err != nil {
		return nil, fmt.Errorf("parse items: %w\nresponse: %s", err, content[:min(200, len(content))])
	}
	return normalizeItems(items), nil
}

const systemPrompt = `Output ONLY valid JSON array of objects.
No prose, no markdown, no comments.
Each object schema: {"url":"https://...","referer":"https://... or empty","read_sec":integer}.`

func EffectiveSystemPrompt(override string) string {
	s := strings.TrimSpace(override)
	if s == "" {
		return systemPrompt
	}
	r := []rune(s)
	if len(r) > maxSystemPromptChars {
		return strings.TrimSpace(string(r[:maxSystemPromptChars]))
	}
	return s
}

func buildPrompt(req Request, memCtx string) string {
	// Adaptive site cap: longer sessions can include more sites,
	// but keep an upper bound for small-model prompt stability.
	siteLimit := req.SiteCap
	if siteLimit <= 0 {
		siteLimit = req.SessionMin / 3
	}
	if siteLimit < 5 {
		siteLimit = 5
	}
	if siteLimit > 12 {
		siteLimit = 12
	}
	sites := req.Sites
	if len(sites) > siteLimit {
		sites = sites[:siteLimit]
	}
	// РљРѕР»РёС‡РµСЃС‚РІРѕ URL = РїСЂРёРјРµСЂРЅРѕ 1 РЅР° РєР°Р¶РґС‹Рµ 4 РјРёРЅСѓС‚С‹ СЃРµСЃСЃРёРё, РЅРѕ РЅРµ Р±РѕР»СЊС€Рµ 8
	count := req.URLCount
	if count <= 0 {
		count = req.SessionMin / 4
	}
	if count < 3 {
		count = 3
	}
	if count > 8 {
		count = 8
	}
	depthLine := "For each entry point, build a chain of 2-3 deep links on the same site."
	if req.ChainDepth > 0 && req.ChainDepth <= 2 {
		depthLine = "For each entry point, build a chain of exactly 2 deep links on the same site."
	}

	return fmt.Sprintf(
		`Context: %s
Generate a realistic browsing session for a %s user at %02d:00.
Create %d URLs total.
Use 1-2 entry points from this site list: %v
%s
For each item provide:
- url: full https URL
- referer: previous page URL in the chain, or empty for entry points
- read_sec: non-round integer (e.g. 47, 63, 112), range 20..180
Return JSON only, example:
[{"url":"https://en.wikipedia.org/wiki/Python_(programming_language)","referer":"","read_sec":47},
 {"url":"https://en.wikipedia.org/wiki/Guido_van_Rossum","referer":"https://en.wikipedia.org/wiki/Python_(programming_language)","read_sec":112}]`,
		memCtx, req.Profile, req.Hour, count, sites, depthLine,
	)
}

func applyRAGWeight(memCtx string, weight float64) string {
	s := strings.TrimSpace(memCtx)
	if s == "" {
		return ""
	}
	w := weight
	if w <= 0 {
		w = 0.7
	}
	if w < 0.05 {
		return ""
	}
	if w > 1.0 {
		w = 1.0
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= 2 {
		return s
	}
	header := lines[0]
	items := lines[1:]
	keep := int(float64(len(items))*w + 0.5)
	if keep < 1 {
		keep = 1
	}
	if keep > len(items) {
		keep = len(items)
	}
	return header + "\n" + strings.Join(items[:keep], "\n")
}

func normalizeItems(items []CoverItem) []CoverItem {
	out := make([]CoverItem, 0, len(items))
	for _, it := range items {
		it.URL = strings.TrimSpace(it.URL)
		it.Referer = strings.TrimSpace(it.Referer)
		if it.URL == "" {
			continue
		}
		if it.ReadSec < 20 {
			it.ReadSec = 20
		}
		if it.ReadSec > 180 {
			it.ReadSec = 180
		}
		out = append(out, it)
	}
	return out
}
