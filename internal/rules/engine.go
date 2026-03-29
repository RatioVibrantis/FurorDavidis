// internal/rules/engine.go
// Р”РµС‚РµСЂРјРёРЅРёСЂРѕРІР°РЅРЅС‹Р№ РґРІРёР¶РѕРє РїСЂР°РІРёР» вЂ” СЂРµС€Р°РµС‚ РљРћР“Р”Рђ РґРµР№СЃС‚РІРѕРІР°С‚СЊ.
// AI СЂРµС€Р°РµС‚ Р§РўРћ РґРµР»Р°С‚СЊ. Go rules СЂРµС€Р°СЋС‚ РљРћР“Р”Рђ.
package rules

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yourorg/furor-davidis/internal/ai"
	"github.com/yourorg/furor-davidis/internal/cover"
	"github.com/yourorg/furor-davidis/internal/logger"
	"github.com/yourorg/furor-davidis/internal/memory"
	"github.com/yourorg/furor-davidis/internal/monitor"
	"github.com/yourorg/furor-davidis/internal/profile"
)

// Engine вЂ” РѕСЂРєРµСЃС‚СЂР°С‚РѕСЂ РїСЂР°РІРёР» Рё РґРµР№СЃС‚РІРёР№.
type Engine struct {
	log          *logger.Logger
	mon          *monitor.Monitor
	exec         *cover.Executor
	mu           sync.RWMutex
	physIP       string
	lastAction   string
	sessionStart time.Time
	cancelCover  context.CancelFunc
	stopCh       chan struct{}
	adapt        adaptiveState
}

type adaptiveParams struct {
	SessionMin    int
	URLCount      int
	ChainDepth    int
	SiteCap       int
	RAGWeight     float64
	TimeoutPolicy string
}

type adaptiveState struct {
	mu             sync.Mutex
	params         adaptiveParams
	lastAdjust     time.Time
	recentAIErrors int
}

type AdaptiveSnapshot struct {
	Enabled       bool    `json:"enabled"`
	Mode          string  `json:"mode"`
	SessionMin    int     `json:"session_min"`
	URLCount      int     `json:"url_count"`
	ChainDepth    int     `json:"chain_depth"`
	SiteCap       int     `json:"site_cap"`
	RAGWeight     float64 `json:"rag_weight"`
	TimeoutPolicy string  `json:"timeout_policy"`
}

func NewEngine(log *logger.Logger) *Engine {
	return &Engine{
		log:          log,
		exec:         cover.NewExecutor(log),
		sessionStart: time.Now(),
		stopCh:       make(chan struct{}, 1),
	}
}

// Run РіР»Р°РІРЅС‹Р№ С†РёРєР» РѕСЂРєРµСЃС‚СЂР°С‚РѕСЂР°. Р—Р°РїСѓСЃРєР°РµС‚СЃСЏ РІ РіРѕСЂСѓС‚РёРЅРµ.
func (e *Engine) Run(ctx context.Context, p profile.Profile, aiEng *ai.Engine, mem *memory.Store) {
	e.mon = monitor.NewMonitor(p.AWGInterface)
	e.exec.SetAWGInterface(p.AWGInterface)

	// РРЅС‚РµСЂРІР°Р» СЃР±РѕСЂР° РјРµС‚СЂРёРє
	collectInterval := 30 * time.Second
	// РРЅС‚РµСЂРІР°Р» Р·Р°РїСЂРѕСЃР° Рє AI (РЅРµ С‡Р°С‰Рµ С‡РµРј РєР°Р¶РґС‹Рµ 60 СЃРµРє)
	aiInterval := 60 * time.Second

	collectTicker := time.NewTicker(collectInterval)
	aiTicker := time.NewTicker(aiInterval)
	defer collectTicker.Stop()
	defer aiTicker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ctx.Done():
			return

		case <-collectTicker.C:
			metrics := e.mon.Collect(p.VPSHost)
			e.log.Debug(monitor.FormatMetrics(metrics))

			if !metrics.Connected {
				break // AWG РЅРµ РїРѕРґРєР»СЋС‡С‘РЅ вЂ” cover traffic Р±РµСЃСЃРјС‹СЃР»РµРЅРµРЅ
			}

			// РџСЂР°РІРёР»Рѕ: RTT РІС‹СЂРѕСЃ в†’ РЅРµРјРµРґР»РµРЅРЅС‹Р№ cover traffic
			if metrics.RTTTrend == "rising" {
				e.log.Info("[Rules] RTT rising -> run cover traffic")
				e.triggerCover(ctx, p, aiEng, mem, metrics)
			}

		case <-aiTicker.C:
			// РћР±РЅРѕРІР»СЏРµРј connected-state РїРµСЂРµРґ РїР»Р°РЅРѕРІС‹Рј trigger, С‡С‚РѕР±С‹ РЅРµ РёСЃРїРѕР»СЊР·РѕРІР°С‚СЊ stale Р·РЅР°С‡РµРЅРёРµ
			cur := e.mon.Collect(p.VPSHost)
			if !cur.Connected {
				e.log.Debug("[Rules] AWG not connected, skip cover")
				break
			}
			// РџР»Р°РЅРѕРІС‹Р№ Р·Р°РїСѓСЃРє cover traffic РїРѕ СЂР°СЃРїРёСЃР°РЅРёСЋ
			e.log.Debug("[Rules] Scheduled cover traffic")
			e.triggerCover(ctx, p, aiEng, mem, cur)
		}
	}
}

// triggerCover Р·Р°РїСЂР°С€РёРІР°РµС‚ Сѓ AI РїРѕСЃР»РµРґРѕРІР°С‚РµР»СЊРЅРѕСЃС‚СЊ Рё Р·Р°РїСѓСЃРєР°РµС‚ РёСЃРїРѕР»РЅРµРЅРёРµ.
func (e *Engine) triggerCover(ctx context.Context, p profile.Profile, aiEng *ai.Engine, mem *memory.Store, metrics monitor.Metrics) {
	if !aiEng.IsReady() {
		e.log.Debug("[Rules] AI backend not ready, skip cover")
		return
	}

	now := time.Now()
	dayType := "weekday"
	if wd := now.Weekday(); wd == time.Saturday || wd == time.Sunday {
		dayType = "weekend"
	}
	ap := e.nextAdaptiveParams(p, mem, metrics)

	req := ai.Request{
		Profile:      p.EffectiveBehaviorProfile(),
		SessionMin:   ap.SessionMin,
		Sites:        p.EffectiveCoverSites(),
		RTTTrend:     metrics.RTTTrend,
		Hour:         now.Hour(),
		DayType:      dayType,
		SystemPrompt: p.AISystemPrompt,
		URLCount:     ap.URLCount,
		ChainDepth:   ap.ChainDepth,
		SiteCap:      ap.SiteCap,
		RAGWeight:    ap.RAGWeight,
	}

	items, err := aiEng.Generate(req, mem)
	if err != nil {
		e.log.Errorf("[Rules] AI error: %v", err)
		return
	}
	e.log.Infof("[Rules] AI generated %d URL(s)", len(items))

	// Р—Р°РїРёСЃС‹РІР°РµРј РІ РїР°РјСЏС‚СЊ (РѕС†РµРЅРёРј С‡РµСЂРµР· 5 РјРёРЅ)
	memCtx := memory.Context{
		Hour: now.Hour(), DayType: dayType,
		RTTTrend: metrics.RTTTrend, Profile: p.EffectiveBehaviorProfile(),
	}
	var urls []string
	for _, it := range items {
		urls = append(urls, it.URL)
	}
	rttBefore := monitor.MeasureRTTMedian(p.VPSHost, 3, 400*time.Millisecond)
	if rttBefore <= 0 {
		rttBefore = metrics.RTTms
	}
	entryID := mem.Record(memCtx, memory.Action{CoverURLs: urls}, rttBefore, timeoutDropByPolicy(ap.TimeoutPolicy))

	e.setLastAction("cover traffic x" + itoa(len(items)))

	// РћС‚РјРµРЅСЏРµРј РїСЂРµРґС‹РґСѓС‰РёР№ cover РµСЃР»Рё РµС‰С‘ РёРґС‘С‚
	if e.cancelCover != nil {
		e.cancelCover()
	}
	coverCtx, cancel := context.WithCancel(ctx)
	e.cancelCover = cancel

	go func() {
		e.exec.Run(coverCtx, items)
	}()

	// Evaluate independently from cover cancellation.
	// Otherwise frequent re-triggers cancel previous sessions and memory stays mostly neutral.
	go func(entry string, rttBefore int, vpsHost string) {
		time.Sleep(5 * time.Minute)
		rttAfter := monitor.MeasureRTTMedian(vpsHost, 3, 400*time.Millisecond)
		mem.Evaluate(entry, rttAfter)
		e.log.Debugf("[Rules] Evaluation: RTT median before=%d after=%d", rttBefore, rttAfter)
	}(entryID, rttBefore, p.VPSHost)
}

// Stop РѕСЃС‚Р°РЅР°РІР»РёРІР°РµС‚ РґРІРёР¶РѕРє.
func (e *Engine) nextAdaptiveParams(p profile.Profile, mem *memory.Store, metrics monitor.Metrics) adaptiveParams {
	base := baselineAdaptiveParams(p)
	if !p.AdaptiveControl {
		return base
	}

	e.adapt.mu.Lock()
	defer e.adapt.mu.Unlock()

	if e.adapt.params.SessionMin == 0 {
		e.adapt.params = base
		e.adapt.lastAdjust = time.Now()
		return e.adapt.params
	}

	if time.Since(e.adapt.lastAdjust) < 10*time.Minute {
		return e.adapt.params
	}

	recent := mem.GetRecent(20)
	total := len(recent)
	if total == 0 {
		return e.adapt.params
	}
	failures, timeouts := 0, 0
	for _, r := range recent {
		if r.Outcome == memory.OutcomeFailure {
			failures++
		}
		if r.EvalTimedOut {
			timeouts++
		}
	}
	failRate := float64(failures) / float64(total)
	timeoutRate := float64(timeouts) / float64(total)

	bad := metrics.RTTTrend == "rising" || timeoutRate > 0.35 || failRate > 0.55
	good := metrics.RTTTrend != "rising" && timeoutRate < 0.15 && failRate < 0.35

	cur := e.adapt.params
	switch {
	case bad:
		cur.SessionMin = clampInt(cur.SessionMin-5, 12, 45)
		cur.URLCount = clampInt(cur.URLCount-1, 3, 8)
		cur.ChainDepth = 2
		cur.SiteCap = clampInt(cur.SiteCap-1, 5, 12)
		cur.RAGWeight = clampFloat(cur.RAGWeight-0.15, 0.30, 1.0)
		cur.TimeoutPolicy = tightenTimeoutPolicy(cur.TimeoutPolicy)
		e.adapt.lastAdjust = time.Now()
		e.log.Infof("[Adaptive] tighten: session=%d url=%d depth=%d sitecap=%d rag=%.2f policy=%s (fail=%.2f timeout=%.2f trend=%s)",
			cur.SessionMin, cur.URLCount, cur.ChainDepth, cur.SiteCap, cur.RAGWeight, cur.TimeoutPolicy, failRate, timeoutRate, metrics.RTTTrend)
	case good:
		cur.SessionMin = clampInt(cur.SessionMin+5, 12, 45)
		cur.URLCount = clampInt(cur.URLCount+1, 3, 8)
		if cur.URLCount >= 5 {
			cur.ChainDepth = 3
		}
		cur.SiteCap = clampInt(cur.SiteCap+1, 5, 12)
		cur.RAGWeight = clampFloat(cur.RAGWeight+0.10, 0.30, 1.0)
		cur.TimeoutPolicy = relaxTimeoutPolicy(cur.TimeoutPolicy)
		e.adapt.lastAdjust = time.Now()
		e.log.Infof("[Adaptive] relax: session=%d url=%d depth=%d sitecap=%d rag=%.2f policy=%s (fail=%.2f timeout=%.2f trend=%s)",
			cur.SessionMin, cur.URLCount, cur.ChainDepth, cur.SiteCap, cur.RAGWeight, cur.TimeoutPolicy, failRate, timeoutRate, metrics.RTTTrend)
	}
	e.adapt.params = cur
	return cur
}

func baselineAdaptiveParams(p profile.Profile) adaptiveParams {
	session := clampInt(p.SessionMinutes, 12, 45)
	urlCount := clampInt(session/4, 3, 8)
	siteCap := clampInt(session/3, 5, 12)
	mode := strings.ToLower(strings.TrimSpace(p.AdaptiveMode))
	out := adaptiveParams{
		SessionMin:    session,
		URLCount:      urlCount,
		ChainDepth:    3,
		SiteCap:       siteCap,
		RAGWeight:     0.70,
		TimeoutPolicy: p.MemoryTimeoutPolicy,
	}
	switch mode {
	case "conservative":
		out.ChainDepth = 2
		out.RAGWeight = 0.55
		if out.TimeoutPolicy == "" {
			out.TimeoutPolicy = "low"
		}
	case "aggressive":
		out.ChainDepth = 3
		out.RAGWeight = 0.85
		if out.TimeoutPolicy == "" {
			out.TimeoutPolicy = "high"
		}
	default:
		if out.TimeoutPolicy == "" {
			out.TimeoutPolicy = "base"
		}
	}
	return out
}

func timeoutDropByPolicy(policy string) float64 {
	switch strings.ToLower(strings.TrimSpace(policy)) {
	case "low":
		return 0.10
	case "high":
		return 0.20
	default:
		return 0.15
	}
}

func tightenTimeoutPolicy(policy string) string {
	switch strings.ToLower(strings.TrimSpace(policy)) {
	case "low":
		return "base"
	case "base":
		return "high"
	default:
		return "high"
	}
}

func relaxTimeoutPolicy(policy string) string {
	switch strings.ToLower(strings.TrimSpace(policy)) {
	case "high":
		return "base"
	case "base":
		return "low"
	default:
		return "low"
	}
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (e *Engine) CurrentAdaptive(p profile.Profile) AdaptiveSnapshot {
	base := baselineAdaptiveParams(p)
	out := AdaptiveSnapshot{
		Enabled:       p.AdaptiveControl,
		Mode:          p.AdaptiveMode,
		SessionMin:    base.SessionMin,
		URLCount:      base.URLCount,
		ChainDepth:    base.ChainDepth,
		SiteCap:       base.SiteCap,
		RAGWeight:     base.RAGWeight,
		TimeoutPolicy: base.TimeoutPolicy,
	}
	if !p.AdaptiveControl {
		return out
	}
	e.adapt.mu.Lock()
	defer e.adapt.mu.Unlock()
	if e.adapt.params.SessionMin > 0 {
		out.SessionMin = e.adapt.params.SessionMin
		out.URLCount = e.adapt.params.URLCount
		out.ChainDepth = e.adapt.params.ChainDepth
		out.SiteCap = e.adapt.params.SiteCap
		out.RAGWeight = e.adapt.params.RAGWeight
		out.TimeoutPolicy = e.adapt.params.TimeoutPolicy
	}
	return out
}

func (e *Engine) Stop() {
	select {
	case e.stopCh <- struct{}{}:
	default:
	}
	if e.cancelCover != nil {
		e.cancelCover()
	}
}

func (e *Engine) PhysIP() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.physIP
}

func (e *Engine) LastAction() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.lastAction
}

func (e *Engine) SessionMin() int {
	return int(time.Since(e.sessionStart).Minutes())
}

func (e *Engine) setLastAction(s string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.lastAction = s
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

// TODO: HotSwap РІС‹Р·С‹РІР°РµС‚СЃСЏ РёР· hotswap/server.go РїРѕ С‚Р°Р№РјРµСЂСѓ
// TODO: HotSwap С‚СЂРёРіРіРµСЂ РїСЂРё session > 30 РјРёРЅ РЅР° РѕРґРЅРѕРј РґРѕРјРµРЅРµ
