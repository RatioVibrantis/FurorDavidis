package main

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/yourorg/furor-davidis/internal/ai"
	"github.com/yourorg/furor-davidis/internal/connect"
	"github.com/yourorg/furor-davidis/internal/deploy"
	"github.com/yourorg/furor-davidis/internal/diag"
	"github.com/yourorg/furor-davidis/internal/logger"
	"github.com/yourorg/furor-davidis/internal/memory"
	"github.com/yourorg/furor-davidis/internal/profile"
	"github.com/yourorg/furor-davidis/internal/rules"
)

type App struct {
	ctx          context.Context
	profile      *profile.Store
	memory       *memory.Store
	rulesEngine  *rules.Engine
	aiEngine     *ai.Engine
	connManager  *connect.Manager
	log          *logger.Logger
	running      bool
	connected    bool
	uiLang       string
	tray         *trayController
	allowExit    bool
	autoSaveStop chan struct{}
	hotSwapStop  chan struct{}
	hsMu         sync.Mutex
	lastHotSwap  time.Time
}

func NewApp() *App { return &App{} }

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.uiLang = "en"
	a.log = logger.New(func(entry logger.Entry) {
		runtime.EventsEmit(ctx, "log", entry)
	})
	a.profile = profile.NewStore()
	a.memory = memory.NewStore()
	a.rulesEngine = rules.NewEngine(a.log)
	a.aiEngine = ai.NewEngine(a.log)

	if err := a.profile.Load(); err != nil {
		a.log.Info("Profile file not found, using defaults")
	}
	if err := a.memory.Load(); err != nil {
		a.log.Info("Memory is empty")
	}

	p := a.profile.Get()
	a.connManager = connect.NewManager(a.log, p.AWGExePath)
	a.log.Info("Furor Davidis started")
	a.initTray()
	a.refreshTray()

	// Create/update state files next to executable even if app runs in tray for long time.
	_ = a.profile.Save()
	_ = a.memory.Save()

	a.autoSaveStop = make(chan struct{})
	go a.autoSaveLoop()
	a.hotSwapStop = make(chan struct{})
	go a.hotSwapLoop()
}

func (a *App) shutdown(ctx context.Context) {
	if a.autoSaveStop != nil {
		close(a.autoSaveStop)
	}
	if a.hotSwapStop != nil {
		close(a.hotSwapStop)
	}
	if a.tray != nil {
		a.tray.shutdown()
	}
	a.Stop()
	if a.connected {
		a.connManager.Disconnect()
	}
	_ = a.profile.Save()
	_ = a.memory.Save()
}

func (a *App) DeployServer() error {
	p := a.profile.Get()
	if p.VPSHost == "" {
		return fmt.Errorf("VPS host is empty")
	}
	a.log.Info("¡Viva la libertad")
	a.log.Info("Launching Supremo...")

	logFn := func(line string) {
		runtime.EventsEmit(a.ctx, "deploy_log", line)
		a.log.Debug("[Deploy] " + line)
	}

	result, err := deploy.Deploy(p, logFn, a.log)
	if err != nil {
		return err
	}

	p.AWGClientPrivKey = result.AWGClientPrivKey
	p.AWGClientPubKey = result.AWGClientPubKey
	p.AWGServerPubKey = result.AWGServerPubKey
	p.AWGListenPort = result.AWGListenPort
	p.AWGClientConfig = result.ClientConfig
	p.Deployed = true
	a.profile.Set(p)
	_ = a.profile.Save()

	runtime.EventsEmit(a.ctx, "deploy_done", result.AWGListenPort)
	return nil
}

func (a *App) VerifyServer() error {
	p := a.profile.Get()
	logFn := func(line string) {
		runtime.EventsEmit(a.ctx, "deploy_log", line)
	}
	return deploy.Verify(p, logFn)
}

func (a *App) ConnectAWG() error {
	p := a.profile.Get()
	if !a.running {
		if err := a.Start(); err != nil {
			return fmt.Errorf("start AI Cover: %w", err)
		}
	}
	if !a.waitForAIReady(4 * time.Second) {
		return fmt.Errorf("LM Studio не готов: запусти LM Studio, загрузи модель и нажми Start Server")
	}
	if !p.Deployed || p.AWGClientConfig == "" {
		return fmt.Errorf("run deploy first")
	}
	if a.connected {
		return fmt.Errorf("already connected")
	}
	a.log.Info("¡Viva la libertad")
	if err := a.connManager.Connect(p.AWGClientConfig, p.VPSHost); err != nil {
		return err
	}
	a.connected = true
	a.lastHotSwap = time.Now()
	a.log.Info("Libero")
	a.log.Info("AWG connected")
	runtime.EventsEmit(a.ctx, "connected", true)
	a.refreshTray()
	return nil
}

func (a *App) DisconnectAWG() {
	if !a.connected {
		a.Stop()
		a.refreshTray()
		return
	}
	a.connManager.Disconnect()
	a.connected = false
	a.log.Info("AWG disconnected")
	runtime.EventsEmit(a.ctx, "connected", false)
	a.Stop()
	a.refreshTray()
}

func (a *App) Start() error {
	if a.running {
		return nil
	}
	a.running = true

	p := a.profile.Get()
	a.log.Info("AI orchestrator starting...")
	cfg := ai.EngineConfig{LMStudioModel: p.LMStudioModel}
	if err := a.aiEngine.Start(cfg); err != nil {
		a.running = false
		a.log.Errorf("[AI] start: %v", err)
		return err
	}

	go a.rulesEngine.Run(a.ctx, p, a.aiEngine, a.memory)
	a.log.Info("AI orchestrator started")
	a.refreshTray()
	return nil
}

func (a *App) Stop() {
	if !a.running {
		return
	}
	a.running = false
	a.rulesEngine.Stop()
	a.aiEngine.Stop()
	a.log.Info("AI orchestrator stopped")
	a.refreshTray()
}

func (a *App) HotSwapDomain(newDomain string) error {
	return a.doHotSwap(newDomain)
}

func (a *App) GetStatus() StatusInfo {
	p := a.profile.Get()
	ad := a.rulesEngine.CurrentAdaptive(p)
	return StatusInfo{
		Running:               a.running,
		Connected:             a.connected,
		AIReady:               a.aiEngine.IsReady(),
		PhysIP:                a.rulesEngine.PhysIP(),
		LastAction:            a.rulesEngine.LastAction(),
		SessionMin:            a.rulesEngine.SessionMin(),
		ActiveDecoy:           p.ActiveDecoyDomain,
		AdaptiveEnabled:       ad.Enabled,
		AdaptiveMode:          ad.Mode,
		AdaptiveSessionMin:    ad.SessionMin,
		AdaptiveURLCount:      ad.URLCount,
		AdaptiveChainDepth:    ad.ChainDepth,
		AdaptiveSiteCap:       ad.SiteCap,
		AdaptiveRAGWeight:     ad.RAGWeight,
		AdaptiveTimeoutPolicy: ad.TimeoutPolicy,
	}
}

type StatusInfo struct {
	Running               bool    `json:"running"`
	Connected             bool    `json:"connected"`
	AIReady               bool    `json:"ai_ready"`
	PhysIP                string  `json:"phys_ip"`
	LastAction            string  `json:"last_action"`
	SessionMin            int     `json:"session_min"`
	ActiveDecoy           string  `json:"active_decoy"`
	AdaptiveEnabled       bool    `json:"adaptive_enabled"`
	AdaptiveMode          string  `json:"adaptive_mode"`
	AdaptiveSessionMin    int     `json:"adaptive_session_min"`
	AdaptiveURLCount      int     `json:"adaptive_url_count"`
	AdaptiveChainDepth    int     `json:"adaptive_chain_depth"`
	AdaptiveSiteCap       int     `json:"adaptive_site_cap"`
	AdaptiveRAGWeight     float64 `json:"adaptive_rag_weight"`
	AdaptiveTimeoutPolicy string  `json:"adaptive_timeout_policy"`
}

func (a *App) RunDiagnostics() diag.Report {
	p := a.profile.Get()
	return diag.Report{
		Local:  diag.CheckLocal(p.LMStudioModel),
		Server: diag.CheckServer(p.VPSHost, p.AWGListenPort),
	}
}

func (a *App) GetProfile() profile.Profile { return a.profile.Get() }

func (a *App) SaveProfile(p profile.Profile) error {
	a.profile.Set(p)
	a.connManager = connect.NewManager(a.log, p.AWGExePath)
	return a.profile.Save()
}

func (a *App) ListServers() []profile.ServerSummary { return a.profile.ListServers() }

func (a *App) GetActiveServerID() string { return a.profile.ActiveServerID() }

func (a *App) CreateServer(name string) (profile.Profile, error) {
	p := a.profile.CreateServer(strings.TrimSpace(name))
	if err := a.profile.Save(); err != nil {
		return profile.Profile{}, err
	}
	a.afterProfileSwitch(p)
	return p, nil
}

func (a *App) SelectServer(id string) error {
	if err := a.profile.SelectServer(id); err != nil {
		return err
	}
	if err := a.profile.Save(); err != nil {
		return err
	}
	a.afterProfileSwitch(a.profile.Get())
	return nil
}

func (a *App) DeleteServer(id string) error {
	if err := a.profile.DeleteServer(id); err != nil {
		return err
	}
	if err := a.profile.Save(); err != nil {
		return err
	}
	a.afterProfileSwitch(a.profile.Get())
	return nil
}

func (a *App) ListClients() []profile.ClientSummary { return a.profile.ListClients() }

func (a *App) GetActiveClientID() string { return a.profile.ActiveClientID() }

func (a *App) CreateClient(name string) (profile.Profile, error) {
	p := a.profile.CreateClient(strings.TrimSpace(name))
	if err := a.profile.Save(); err != nil {
		return profile.Profile{}, err
	}
	a.afterProfileSwitch(p)
	return p, nil
}

func (a *App) SelectClient(id string) error {
	if err := a.profile.SelectClient(id); err != nil {
		return err
	}
	if err := a.profile.Save(); err != nil {
		return err
	}
	a.afterProfileSwitch(a.profile.Get())
	return nil
}

func (a *App) DeleteClient(id string) error {
	if err := a.profile.DeleteClient(id); err != nil {
		return err
	}
	if err := a.profile.Save(); err != nil {
		return err
	}
	a.afterProfileSwitch(a.profile.Get())
	return nil
}

func (a *App) ExportActiveProfile() error {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export profile",
		DefaultFilename: "furor_profile_export.json",
		Filters:         []runtime.FileFilter{{DisplayName: "JSON", Pattern: "*.json"}},
	})
	if err != nil || path == "" {
		return err
	}
	return a.profile.ExportActive(path)
}

func (a *App) ImportProfile() error {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Import profile",
		Filters: []runtime.FileFilter{{DisplayName: "JSON", Pattern: "*.json"}},
	})
	if err != nil || path == "" {
		return err
	}
	p, err := a.profile.ImportSingle(path)
	if err != nil {
		return err
	}
	if err := a.profile.Save(); err != nil {
		return err
	}
	a.afterProfileSwitch(p)
	return nil
}

func (a *App) GetLMStudioModels() ([]string, error) {
	return a.aiEngine.ListModels("http://localhost:1234")
}

func (a *App) GetEffectiveAIPrompt() string {
	return ai.EffectiveSystemPrompt(a.profile.Get().AISystemPrompt)
}

func (a *App) SetUILanguage(lang string) {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "ru":
		a.uiLang = "ru"
	default:
		a.uiLang = "en"
	}
	a.refreshTray()
}

func (a *App) afterProfileSwitch(p profile.Profile) {
	if a.running {
		a.Stop()
	}
	if a.connected {
		a.connManager.Disconnect()
		a.connected = false
		runtime.EventsEmit(a.ctx, "connected", false)
	}
	a.connManager = connect.NewManager(a.log, p.AWGExePath)
	a.refreshTray()
}

func (a *App) beforeClose(ctx context.Context) bool {
	if a.allowExit {
		return false
	}
	runtime.WindowHide(a.ctx)
	return true
}

func (a *App) quitFromTray() {
	a.allowExit = true
	runtime.Quit(a.ctx)
}

func (a *App) waitForAIReady(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if a.aiEngine != nil && a.aiEngine.IsReady() {
			return true
		}
		time.Sleep(150 * time.Millisecond)
	}
	return a.aiEngine != nil && a.aiEngine.IsReady()
}

func (a *App) GetMemoryStats() memory.Stats          { return a.memory.GetStats() }
func (a *App) GetMemoryEntries(n int) []memory.Entry { return a.memory.GetTop(n) }
func (a *App) ClearMemory()                          { a.memory.Clear() }

func (a *App) ExportMemory() error {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export memory",
		DefaultFilename: "furor_memory.json",
		Filters:         []runtime.FileFilter{{DisplayName: "JSON", Pattern: "*.json"}},
	})
	if err != nil || path == "" {
		return err
	}
	return a.memory.ExportTo(path)
}

func (a *App) ImportMemory() error {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Import memory",
		Filters: []runtime.FileFilter{{DisplayName: "JSON", Pattern: "*.json"}},
	})
	if err != nil || path == "" {
		return err
	}
	return a.memory.ImportFrom(path)
}

func (a *App) autoSaveLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-a.autoSaveStop:
			return
		case <-ticker.C:
			_ = a.profile.Save()
			_ = a.memory.Save()
		}
	}
}

func (a *App) hotSwapLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-a.hotSwapStop:
			return
		case <-ticker.C:
			if !a.running || !a.connected {
				continue
			}
			p := a.profile.Get()
			if !p.Deployed || !p.HotSwapEnabled || p.HotSwapInterval <= 0 || len(p.DecoyDomains) < 2 {
				continue
			}
			interval := time.Duration(p.HotSwapInterval) * time.Minute
			if !a.lastHotSwap.IsZero() && time.Since(a.lastHotSwap) < interval {
				continue
			}
			next := nextDecoyDomain(p.ActiveDecoyDomain, p.DecoyDomains)
			if strings.TrimSpace(next) == "" || next == p.ActiveDecoyDomain {
				continue
			}
			if err := a.doHotSwap(next); err != nil {
				a.log.Errorf("[HotSwap] auto: %v", err)
			} else {
				a.log.Infof("[HotSwap] auto switched -> %s", next)
			}
		}
	}
}

func (a *App) doHotSwap(newDomain string) error {
	a.hsMu.Lock()
	defer a.hsMu.Unlock()

	p := a.profile.Get()
	logFn := func(line string) {
		runtime.EventsEmit(a.ctx, "deploy_log", line)
	}
	if err := deploy.HotSwap(p, newDomain, logFn, a.log); err != nil {
		return err
	}
	p.ActiveDecoyDomain = newDomain
	a.profile.Set(p)
	_ = a.profile.Save()
	a.lastHotSwap = time.Now()
	return nil
}

func nextDecoyDomain(current string, domains []string) string {
	if len(domains) == 0 {
		return ""
	}
	if len(domains) == 1 {
		return domains[0]
	}
	cur := strings.TrimSpace(current)
	if cur == "" {
		return domains[0]
	}
	idx := slices.Index(domains, cur)
	if idx < 0 {
		return domains[0]
	}
	return domains[(idx+1)%len(domains)]
}
