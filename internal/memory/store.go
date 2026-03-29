// internal/memory/store.go
// RAG-подобная память Furor Davidis.
//
// Логика:
//
//	Go core оценивает результат действия AI (RTT до vs RTT через 5 мин).
//	Успешные паттерны накапливаются → инжектируются в system prompt.
//	Файл furor_memory.json переносим: можно шарить между машинами.
//
// Скоринг:
//
//	initial = 0.5
//	RTT упал  >15% → +0.3
//	RTT вырос >15% → -0.3
//	score декайится 1%/день → старые записи меньше влияют
//	cap = 500 записей; при превышении удаляем с наименьшим score
package memory

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	memoryFileName      = "furor_memory.json"
	maxEntries          = 500
	pendingEvalTimeout  = 7 * time.Minute
	minPromptRelevance  = 0.35
	minEntriesForPrompt = 10
	defaultTimeoutDrop  = 0.15
)

// Context описывает условия в момент принятия решения.
type Context struct {
	Hour      int    `json:"hour"`      // 0-23
	DayType   string `json:"day_type"`  // "weekday" / "weekend"
	RTTTrend  string `json:"rtt_trend"` // "rising" / "falling" / "stable"
	Profile   string `json:"profile"`   // "developer" / "casual" / "researcher"
	Intensity string `json:"intensity"`
}

// Action описывает что AI решил сделать.
type Action struct {
	CoverURLs   []string `json:"cover_urls"`   // реальные URL которые запросили
	TimingMs    int      `json:"timing_ms"`    // средний интервал между запросами
	HotSwap     bool     `json:"hotswap"`      // была ли смена домена
	DecoyDomain string   `json:"decoy_domain"` // на какой домен переключили
}

// Outcome результат — оценивается Go через 5 мин.
type Outcome string

const (
	OutcomeSuccess Outcome = "success" // RTT стабилизировался/упал
	OutcomeFailure Outcome = "failure" // RTT вырос или разрыв
	OutcomeNeutral Outcome = "neutral" // без изменений
)

// Entry — одна запись памяти.
type Entry struct {
	ID           string    `json:"id"`
	Timestamp    time.Time `json:"timestamp"`
	Context      Context   `json:"context"`
	Action       Action    `json:"action"`
	Outcome      Outcome   `json:"outcome"`
	Score        float64   `json:"score"` // 0.0 - 1.0
	RTTBefore    int       `json:"rtt_before_ms"`
	RTTAfter     int       `json:"rtt_after_ms"`
	EvalTimedOut bool      `json:"eval_timed_out"`
	Note         string    `json:"note"`
}

// Stats — статистика для UI.
type Stats struct {
	Total      int     `json:"total"`
	Successes  int     `json:"successes"`
	Failures   int     `json:"failures"`
	Timeouts   int     `json:"timeouts"`
	AvgScore   float64 `json:"avg_score"`
	OldestDays int     `json:"oldest_days"`
}

type memoryData struct {
	Version  int     `json:"version"`
	Sessions int     `json:"total_sessions"`
	Entries  []Entry `json:"entries"`
}

// Store — потокобезопасное хранилище памяти.
type Store struct {
	mu      sync.RWMutex
	data    memoryData
	pending map[string]pendingEval // ожидают оценки через 5 мин
}

type pendingEval struct {
	entryID     string
	rttBefore   int
	timeoutDrop float64
	evalAt      time.Time
}

func NewStore() *Store {
	s := &Store{
		data:    memoryData{Version: 1},
		pending: make(map[string]pendingEval),
	}
	go s.evalLoop()
	return s
}

// Record записывает новое действие AI (outcome пока neutral, оценим позже).
func (s *Store) Record(ctx Context, action Action, rttBefore int, timeoutDrop float64) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := fmt.Sprintf("%d", time.Now().UnixNano())
	entry := Entry{
		ID:        id,
		Timestamp: time.Now(),
		Context:   ctx,
		Action:    action,
		Outcome:   OutcomeNeutral,
		Score:     0.5,
		RTTBefore: rttBefore,
	}
	s.data.Entries = append(s.data.Entries, entry)
	s.data.Sessions++
	s.trim()

	// запланировать оценку через 5 минут
	s.pending[id] = pendingEval{
		entryID:     id,
		rttBefore:   rttBefore,
		timeoutDrop: sanitizeTimeoutDrop(timeoutDrop),
		// Ждём чуть дольше целевой оценки (5 мин), чтобы уменьшить гонку по времени.
		evalAt: time.Now().Add(pendingEvalTimeout),
	}
	return id
}

// Evaluate вызывается Go core через 5 мин с актуальным RTT.
func (s *Store) Evaluate(entryID string, rttAfter int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	pe, ok := s.pending[entryID]
	if !ok {
		return
	}
	delete(s.pending, entryID)

	for i := range s.data.Entries {
		if s.data.Entries[i].ID != entryID {
			continue
		}
		e := &s.data.Entries[i]
		e.RTTAfter = rttAfter
		e.EvalTimedOut = false
		if pe.rttBefore <= 0 || rttAfter <= 0 {
			e.Outcome = OutcomeFailure
			e.Score = math.Max(0.0, e.Score-sanitizeTimeoutDrop(pe.timeoutDrop))
			e.EvalTimedOut = rttAfter <= 0
			e.Note = fmt.Sprintf("missing RTT sample: before=%dms after=%dms", pe.rttBefore, rttAfter)
			break
		}

		delta := float64(rttAfter-pe.rttBefore) / float64(pe.rttBefore+1)
		switch {
		case delta < -0.15: // RTT упал >15% — хорошо
			e.Score = math.Min(1.0, e.Score+0.3)
			e.Outcome = OutcomeSuccess
			e.Note = fmt.Sprintf("RTT %dms->%dms (-%.0f%%)", pe.rttBefore, rttAfter, -delta*100)
		case delta > 0.15: // RTT вырос >15% — плохо
			e.Score = math.Max(0.0, e.Score-0.3)
			e.Outcome = OutcomeFailure
			e.Note = fmt.Sprintf("RTT %dms->%dms (+%.0f%%)", pe.rttBefore, rttAfter, delta*100)
		default:
			// With very stable links (for example 1ms->1ms) a neutral result
			// can dominate and prevent RAG from accumulating useful signal.
			// Treat stable measured sessions as weak success.
			e.Outcome = OutcomeSuccess
			e.Score = math.Min(1.0, e.Score+0.05)
			e.Note = fmt.Sprintf("RTT %dms->%dms (stable)", pe.rttBefore, rttAfter)
		}
		break
	}
}

// BuildPromptContext строит текст для инжекции в system prompt AI.
// Выбирает top-5 релевантных записей по контексту.
func (s *Store) BuildPromptContext(ctx Context) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type scored struct {
		e     Entry
		score float64
	}
	var candidates []scored
	evaluatedCount := 0

	now := time.Now()
	for _, e := range s.data.Entries {
		if e.Outcome == OutcomeNeutral {
			continue
		}
		evaluatedCount++
		// релевантность по контексту
		rel := 0.0
		if e.Context.DayType == ctx.DayType {
			rel += 0.2
		}
		if e.Context.RTTTrend == ctx.RTTTrend {
			rel += 0.3
		}
		if e.Context.Profile == ctx.Profile {
			rel += 0.2
		}
		if abs(e.Context.Hour-ctx.Hour) <= 2 {
			rel += 0.3
		}
		if rel < minPromptRelevance {
			continue
		}

		// декай: теряем 1% в день
		days := now.Sub(e.Timestamp).Hours() / 24
		decay := math.Pow(0.99, days)

		candidates = append(candidates, scored{e, e.Score * rel * decay})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	if evaluatedCount < minEntriesForPrompt || len(candidates) == 0 {
		return ""
	}
	if len(candidates) > 5 {
		candidates = candidates[:5]
	}

	var sb strings.Builder
	sb.WriteString("## Learned patterns from your network:\n")
	for _, c := range candidates {
		icon := "[OK]"
		if c.e.Outcome == OutcomeFailure {
			icon = "[BAD]"
		}
		urls := strings.Join(c.e.Action.CoverURLs, ", ")
		if len(urls) > 80 {
			urls = urls[:80] + "..."
		}
		sb.WriteString(fmt.Sprintf(
			"%s [score:%.2f] %s %02d:00, %s RTT -> [%s] -> %s\n",
			icon, c.e.Score,
			c.e.Context.DayType, c.e.Context.Hour,
			c.e.Context.RTTTrend,
			urls, c.e.Note,
		))
	}
	return sb.String()
}

// GetTop возвращает N записей с наибольшим score.
func (s *Store) GetTop(n int) []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make([]Entry, len(s.data.Entries))
	copy(cp, s.data.Entries)
	sort.Slice(cp, func(i, j int) bool { return cp[i].Score > cp[j].Score })
	if n > 0 && len(cp) > n {
		cp = cp[:n]
	}
	return cp
}

// GetRecent returns latest N entries by timestamp (newest first).
func (s *Store) GetRecent(n int) []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make([]Entry, len(s.data.Entries))
	copy(cp, s.data.Entries)
	sort.Slice(cp, func(i, j int) bool { return cp[i].Timestamp.After(cp[j].Timestamp) })
	if n > 0 && len(cp) > n {
		cp = cp[:n]
	}
	return cp
}

// GetStats возвращает агрегированную статистику.
func (s *Store) GetStats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st := Stats{Total: len(s.data.Entries)}
	var scoreSum float64
	oldest := time.Now()
	for _, e := range s.data.Entries {
		scoreSum += e.Score
		if e.Outcome == OutcomeSuccess {
			st.Successes++
		} else if e.Outcome == OutcomeFailure {
			st.Failures++
		}
		if e.EvalTimedOut {
			st.Timeouts++
		}
		if e.Timestamp.Before(oldest) {
			oldest = e.Timestamp
		}
	}
	if st.Total > 0 {
		st.AvgScore = scoreSum / float64(st.Total)
		st.OldestDays = int(time.Since(oldest).Hours() / 24)
	}
	return st
}

func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = memoryData{Version: 1}
}

func (s *Store) Load() error {
	b, err := os.ReadFile(memoryFilePath())
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := json.Unmarshal(b, &s.data); err != nil {
		return err
	}
	s.normalizeLoadedEntriesLocked()
	return nil
}

func (s *Store) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(memoryFilePath(), b, 0644)
}

func (s *Store) ExportTo(path string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

func (s *Store) ImportFrom(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var imported memoryData
	if err := json.Unmarshal(b, &imported); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// мёрджим: добавляем записи которых нет по ID
	existing := make(map[string]bool)
	for _, e := range s.data.Entries {
		existing[e.ID] = true
	}
	added := 0
	for _, e := range imported.Entries {
		if !existing[e.ID] {
			s.data.Entries = append(s.data.Entries, e)
			added++
		}
	}
	s.trim()
	return nil
}

// trim удаляет записи с наименьшим score при превышении лимита.
func (s *Store) trim() {
	if len(s.data.Entries) <= maxEntries {
		return
	}
	sort.Slice(s.data.Entries, func(i, j int) bool {
		return s.data.Entries[i].Score > s.data.Entries[j].Score
	})
	s.data.Entries = s.data.Entries[:maxEntries]
}

// evalLoop периодически проверяет pending оценки.
func (s *Store) evalLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		for id, pe := range s.pending {
			if time.Now().After(pe.evalAt) {
				// Если оценка не пришла — считаем это негативным сигналом, но мягче обычного failure.
				s.markMissingEvalLocked(pe)
				delete(s.pending, id)
			}
		}
		s.mu.Unlock()
	}
}

func (s *Store) markMissingEvalLocked(pe pendingEval) {
	for i := range s.data.Entries {
		if s.data.Entries[i].ID != pe.entryID {
			continue
		}
		e := &s.data.Entries[i]
		if e.Outcome != OutcomeNeutral {
			return
		}
		e.Outcome = OutcomeFailure
		e.EvalTimedOut = true
		e.Score = math.Max(0.0, e.Score-sanitizeTimeoutDrop(pe.timeoutDrop))
		e.Note = "evaluation timeout: no RTT-after sample"
		return
	}
}

func (s *Store) normalizeLoadedEntriesLocked() {
	for i := range s.data.Entries {
		e := &s.data.Entries[i]
		if e.Outcome != OutcomeNeutral {
			continue
		}
		if e.RTTBefore <= 0 {
			continue
		}
		if e.RTTAfter > 0 {
			e.Outcome = OutcomeSuccess
			e.EvalTimedOut = false
			e.Score = math.Min(1.0, e.Score+0.05)
			e.Note = fmt.Sprintf("RTT %dms->%dms (stable)", e.RTTBefore, e.RTTAfter)
			continue
		}
		e.Outcome = OutcomeFailure
		e.EvalTimedOut = true
		e.Score = math.Max(0.0, e.Score-defaultTimeoutDrop)
		e.Note = fmt.Sprintf("missing RTT sample: before=%dms after=%dms", e.RTTBefore, e.RTTAfter)
	}
}

func sanitizeTimeoutDrop(v float64) float64 {
	if v <= 0 {
		return defaultTimeoutDrop
	}
	if v < 0.01 {
		return 0.01
	}
	if v > 0.5 {
		return 0.5
	}
	return v
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func memoryFilePath() string {
	exe, err := os.Executable()
	if err != nil || strings.TrimSpace(exe) == "" {
		return memoryFileName
	}
	return filepath.Join(filepath.Dir(exe), memoryFileName)
}
