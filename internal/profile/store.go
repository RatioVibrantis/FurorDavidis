package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const profileFileName = "furor_profile.json"

// Profile is a merged runtime view of active server + active client.
type Profile struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ServerID   string `json:"server_id"`
	ServerName string `json:"server_name"`
	ClientID   string `json:"client_id"`
	ClientName string `json:"client_name"`

	// VPS / SSH
	VPSHost     string `json:"vps_host"`
	VPSPort     int    `json:"vps_port"`
	VPSUser     string `json:"vps_user"`
	VPSPassword string `json:"vps_password"`

	// Deploy state
	Deployed         bool   `json:"deployed"`
	AWGListenPort    string `json:"awg_listen_port"`
	AWGClientPrivKey string `json:"awg_client_priv_key"`
	AWGClientPubKey  string `json:"awg_client_pub_key"`
	AWGServerPubKey  string `json:"awg_server_pub_key"`
	AWGClientConfig  string `json:"awg_client_config"`

	// AWG client
	AWGExePath        string `json:"awg_exe_path"`
	AWGInterface      string `json:"awg_interface"`
	ActiveDecoyDomain string `json:"active_decoy_domain"`

	// HotSwap
	DecoyDomains    []string `json:"decoy_domains"`
	HotSwapEnabled  bool     `json:"hotswap_enabled"`
	HotSwapInterval int      `json:"hotswap_interval_min"`

	// AI backend
	LMStudioModel  string `json:"lmstudio_model"`
	AISystemPrompt string `json:"ai_system_prompt"`

	// Cover traffic
	CoverSites          []string    `json:"cover_sites"`      // legacy compatibility
	BehaviorProfile     string      `json:"behavior_profile"` // legacy compatibility
	CoverLists          []CoverList `json:"cover_lists"`
	ActiveCoverListID   string      `json:"active_cover_list_id"`
	Intensity           string      `json:"intensity"`
	SessionMinutes      int         `json:"session_minutes"`
	MemoryTimeoutPolicy string      `json:"memory_timeout_policy"`
	AdaptiveControl     bool        `json:"adaptive_control"`
	AdaptiveMode        string      `json:"adaptive_mode"`
}

type CoverList struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Sites []string `json:"sites"`
}

func (p Profile) EffectiveCoverSites() []string {
	if len(p.CoverLists) > 0 {
		return append([]string(nil), activeCoverList(p.CoverLists, p.ActiveCoverListID).Sites...)
	}
	return append([]string(nil), p.CoverSites...)
}

func (p Profile) EffectiveBehaviorProfile() string {
	if len(p.CoverLists) > 0 {
		name := strings.TrimSpace(activeCoverList(p.CoverLists, p.ActiveCoverListID).Name)
		if name != "" {
			return name
		}
	}
	if strings.TrimSpace(p.BehaviorProfile) != "" {
		return p.BehaviorProfile
	}
	return "Custom"
}

func (p Profile) TimeoutPenaltyDrop() float64 {
	switch strings.ToLower(strings.TrimSpace(p.MemoryTimeoutPolicy)) {
	case "low":
		return 0.10
	case "high":
		return 0.20
	default:
		return 0.15
	}
}

type Server struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	VPSHost           string   `json:"vps_host"`
	VPSPort           int      `json:"vps_port"`
	VPSUser           string   `json:"vps_user"`
	VPSPassword       string   `json:"vps_password"`
	Deployed          bool     `json:"deployed"`
	AWGListenPort     string   `json:"awg_listen_port"`
	AWGServerPubKey   string   `json:"awg_server_pub_key"`
	ActiveDecoyDomain string   `json:"active_decoy_domain"`
	DecoyDomains      []string `json:"decoy_domains"`
	HotSwapEnabled    bool     `json:"hotswap_enabled"`
	HotSwapInterval   int      `json:"hotswap_interval_min"`
	Clients           []Client `json:"clients"`
	ActiveClientID    string   `json:"active_client_id"`
}

type Client struct {
	ID                  string      `json:"id"`
	Name                string      `json:"name"`
	AWGClientPrivKey    string      `json:"awg_client_priv_key"`
	AWGClientPubKey     string      `json:"awg_client_pub_key"`
	AWGClientConfig     string      `json:"awg_client_config"`
	AWGExePath          string      `json:"awg_exe_path"`
	AWGInterface        string      `json:"awg_interface"`
	LMStudioModel       string      `json:"lmstudio_model"`
	AISystemPrompt      string      `json:"ai_system_prompt"`
	CoverLists          []CoverList `json:"cover_lists"`
	ActiveCoverListID   string      `json:"active_cover_list_id"`
	CoverSites          []string    `json:"cover_sites"`
	BehaviorProfile     string      `json:"behavior_profile"`
	Intensity           string      `json:"intensity"`
	SessionMinutes      int         `json:"session_minutes"`
	MemoryTimeoutPolicy string      `json:"memory_timeout_policy"`
	AdaptiveControl     bool        `json:"adaptive_control"`
	AdaptiveMode        string      `json:"adaptive_mode"`
}

type ServerSummary struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	VPSHost  string `json:"vps_host"`
	Clients  int    `json:"clients"`
	Active   bool   `json:"active"`
	Deployed bool   `json:"deployed"`
}

type ClientSummary struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type fileData struct {
	ActiveServerID string   `json:"active_server_id"`
	Servers        []Server `json:"servers"`
}

// previous v0.6.2 format
type legacyV62File struct {
	ActiveProfileID string    `json:"active_profile_id"`
	Profiles        []Profile `json:"profiles"`
}

type Store struct {
	mu   sync.RWMutex
	data fileData
}

func defaultCoverLists() []CoverList {
	return []CoverList{
		{
			ID:   "developer",
			Name: "Developer",
			Sites: []string{
				"github.com",
				"docs.python.org",
				"stackoverflow.com",
				"developer.mozilla.org",
			},
		},
		{
			ID:   "casual",
			Name: "Casual",
			Sites: []string{
				"youtube.com",
				"wikipedia.org",
				"news.ycombinator.com",
				"reddit.com",
			},
		},
		{
			ID:   "researcher",
			Name: "Researcher",
			Sites: []string{
				"arxiv.org",
				"wikipedia.org",
				"scholar.google.com",
				"pubmed.ncbi.nlm.nih.gov",
			},
		},
	}
}

func defaultServer() Server {
	c := defaultClient()
	return Server{
		ID:              newServerID(),
		Name:            "Server 1",
		VPSPort:         22,
		VPSUser:         "root",
		DecoyDomains:    []string{"microsoft.com", "youtube.com", "github.com", "cloudflare.com", "apple.com"},
		HotSwapEnabled:  true,
		HotSwapInterval: 30,
		Clients:         []Client{c},
		ActiveClientID:  c.ID,
	}
}

func defaultClient() Client {
	lists := defaultCoverLists()
	return Client{
		ID:                  newClientID(),
		Name:                "Client 1",
		AWGExePath:          `awg\amneziawg.exe`,
		AWGInterface:        "furor",
		CoverLists:          lists,
		ActiveCoverListID:   lists[0].ID,
		CoverSites:          append([]string(nil), lists[0].Sites...),
		BehaviorProfile:     lists[0].Name,
		Intensity:           "medium",
		SessionMinutes:      20,
		MemoryTimeoutPolicy: "base",
		AdaptiveControl:     true,
		AdaptiveMode:        "balanced",
	}
}

func NewStore() *Store {
	s := defaultServer()
	return &Store{
		data: fileData{
			ActiveServerID: s.ID,
			Servers:        []Server{s},
		},
	}
}

func (s *Store) Get() Profile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	srv, cli, ok := s.activeServerClientLocked()
	if !ok {
		return defaultMergedProfile()
	}
	return mergeToProfile(srv, cli)
}

func (s *Store) Set(p Profile) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p = withDefaults(p)
	srv, cli, ok := s.activeServerClientLocked()
	if !ok {
		d := defaultServer()
		s.data.Servers = []Server{d}
		s.data.ActiveServerID = d.ID
		srv, cli, _ = s.activeServerClientLocked()
	}

	// server part
	srv.Name = choose(p.ServerName, srv.Name)
	srv.VPSHost = p.VPSHost
	srv.VPSPort = p.VPSPort
	srv.VPSUser = p.VPSUser
	srv.VPSPassword = p.VPSPassword
	srv.Deployed = p.Deployed
	srv.AWGListenPort = p.AWGListenPort
	srv.AWGServerPubKey = p.AWGServerPubKey
	srv.ActiveDecoyDomain = p.ActiveDecoyDomain
	srv.DecoyDomains = append([]string(nil), p.DecoyDomains...)
	srv.HotSwapEnabled = p.HotSwapEnabled
	srv.HotSwapInterval = p.HotSwapInterval

	// client part
	cli.Name = choose(p.ClientName, cli.Name)
	cli.AWGClientPrivKey = p.AWGClientPrivKey
	cli.AWGClientPubKey = p.AWGClientPubKey
	cli.AWGClientConfig = p.AWGClientConfig
	cli.AWGExePath = p.AWGExePath
	cli.AWGInterface = p.AWGInterface
	cli.LMStudioModel = p.LMStudioModel
	cli.AISystemPrompt = p.AISystemPrompt
	cli.CoverLists = normalizeCoverLists(p.CoverLists, p.CoverSites, p.BehaviorProfile)
	cli.ActiveCoverListID = pickActiveCoverListID(cli.CoverLists, p.ActiveCoverListID)
	active := activeCoverList(cli.CoverLists, cli.ActiveCoverListID)
	cli.CoverSites = append([]string(nil), active.Sites...)
	cli.BehaviorProfile = active.Name
	cli.Intensity = p.Intensity
	cli.SessionMinutes = p.SessionMinutes
	cli.MemoryTimeoutPolicy = p.MemoryTimeoutPolicy
	cli.AdaptiveControl = p.AdaptiveControl
	cli.AdaptiveMode = p.AdaptiveMode

	// write back pointers
	s.setServerClientLocked(srv, cli)
}

func (s *Store) ListServers() []ServerSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ServerSummary, 0, len(s.data.Servers))
	for _, srv := range s.data.Servers {
		out = append(out, ServerSummary{
			ID:       srv.ID,
			Name:     srv.Name,
			VPSHost:  srv.VPSHost,
			Clients:  len(srv.Clients),
			Active:   srv.ID == s.data.ActiveServerID,
			Deployed: srv.Deployed,
		})
	}
	return out
}

func (s *Store) ActiveServerID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.ActiveServerID
}

func (s *Store) CreateServer(name string) Profile {
	s.mu.Lock()
	defer s.mu.Unlock()
	base, _, ok := s.activeServerClientLocked()
	var srv Server
	if ok {
		srv = cloneServerTemplate(base)
	} else {
		srv = defaultServer()
	}
	srv.ID = newServerID()
	srv.Name = strings.TrimSpace(name)
	if srv.Name == "" {
		srv.Name = fmt.Sprintf("Server %d", len(s.data.Servers)+1)
	}
	// New server starts not deployed and with a fresh default client.
	srv.Deployed = false
	srv.AWGListenPort = ""
	srv.AWGServerPubKey = ""
	srv.ActiveDecoyDomain = ""
	c := defaultClient()
	srv.Clients = []Client{c}
	srv.ActiveClientID = c.ID

	s.data.Servers = append(s.data.Servers, srv)
	s.data.ActiveServerID = srv.ID
	return mergeToProfile(srv, c)
}

func (s *Store) SelectServer(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.findServerIndexLocked(id) < 0 {
		return fmt.Errorf("server not found")
	}
	s.data.ActiveServerID = id
	return nil
}

func (s *Store) DeleteServer(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.data.Servers) <= 1 {
		return fmt.Errorf("cannot delete the last server")
	}
	idx := s.findServerIndexLocked(id)
	if idx < 0 {
		return fmt.Errorf("server not found")
	}
	s.data.Servers = append(s.data.Servers[:idx], s.data.Servers[idx+1:]...)
	if s.data.ActiveServerID == id {
		s.data.ActiveServerID = s.data.Servers[0].ID
	}
	return nil
}

func (s *Store) ListClients() []ClientSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	srv, ok := s.activeServerLocked()
	if !ok {
		return nil
	}
	out := make([]ClientSummary, 0, len(srv.Clients))
	for _, c := range srv.Clients {
		out = append(out, ClientSummary{
			ID:     c.ID,
			Name:   c.Name,
			Active: c.ID == srv.ActiveClientID,
		})
	}
	return out
}

func (s *Store) ActiveClientID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	srv, ok := s.activeServerLocked()
	if !ok {
		return ""
	}
	return srv.ActiveClientID
}

func (s *Store) CreateClient(name string) Profile {
	s.mu.Lock()
	defer s.mu.Unlock()
	srv, cli, ok := s.activeServerClientLocked()
	if !ok {
		d := defaultServer()
		s.data.Servers = []Server{d}
		s.data.ActiveServerID = d.ID
		srv, cli, _ = s.activeServerClientLocked()
	}
	nc := cloneClientTemplate(cli)
	nc.ID = newClientID()
	nc.Name = strings.TrimSpace(name)
	if nc.Name == "" {
		nc.Name = fmt.Sprintf("Client %d", len(srv.Clients)+1)
	}
	// Keep deploy-neutral client fields empty by default.
	nc.AWGClientPrivKey = ""
	nc.AWGClientPubKey = ""
	nc.AWGClientConfig = ""

	srv.Clients = append(srv.Clients, nc)
	srv.ActiveClientID = nc.ID
	s.setActiveServerLocked(srv)
	return mergeToProfile(srv, nc)
}

func (s *Store) SelectClient(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	srv, ok := s.activeServerLocked()
	if !ok {
		return fmt.Errorf("server not found")
	}
	if findClientIndex(srv.Clients, id) < 0 {
		return fmt.Errorf("client not found")
	}
	srv.ActiveClientID = id
	s.setActiveServerLocked(srv)
	return nil
}

func (s *Store) DeleteClient(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	srv, ok := s.activeServerLocked()
	if !ok {
		return fmt.Errorf("server not found")
	}
	if len(srv.Clients) <= 1 {
		return fmt.Errorf("cannot delete the last client")
	}
	idx := findClientIndex(srv.Clients, id)
	if idx < 0 {
		return fmt.Errorf("client not found")
	}
	srv.Clients = append(srv.Clients[:idx], srv.Clients[idx+1:]...)
	if srv.ActiveClientID == id {
		srv.ActiveClientID = srv.Clients[0].ID
	}
	s.setActiveServerLocked(srv)
	return nil
}

func (s *Store) ExportActive(path string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	srv, cli, ok := s.activeServerClientLocked()
	if !ok {
		return fmt.Errorf("active profile not found")
	}
	p := mergeToProfile(srv, cli)
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

func (s *Store) ImportSingle(path string) (Profile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Profile{}, err
	}
	var p Profile
	if err := json.Unmarshal(b, &p); err != nil {
		return Profile{}, err
	}
	p = withDefaults(p)

	s.mu.Lock()
	defer s.mu.Unlock()
	srv := fromProfileToServer(p)
	cli := fromProfileToClient(p)
	srv.ID = newServerID()
	if strings.TrimSpace(srv.Name) == "" {
		srv.Name = fmt.Sprintf("Imported server %s", time.Now().Format("2006-01-02 15:04"))
	}
	cli.ID = newClientID()
	if strings.TrimSpace(cli.Name) == "" {
		cli.Name = "Imported client"
	}
	srv.Clients = []Client{cli}
	srv.ActiveClientID = cli.ID
	s.data.Servers = append(s.data.Servers, srv)
	s.data.ActiveServerID = srv.ID
	return mergeToProfile(srv, cli), nil
}

func (s *Store) Load() error {
	b, err := os.ReadFile(profileFilePath())
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// v0.6.4+ format: servers[] + active_server_id
	var fd fileData
	if err := json.Unmarshal(b, &fd); err == nil && len(fd.Servers) > 0 {
		for i := range fd.Servers {
			fd.Servers[i] = withServerDefaults(fd.Servers[i], i+1)
		}
		if fd.ActiveServerID == "" || findServerIndex(fd.Servers, fd.ActiveServerID) < 0 {
			fd.ActiveServerID = fd.Servers[0].ID
		}
		s.data = fd
		return nil
	}

	// v0.6.2 format: profiles[] + active_profile_id
	var old legacyV62File
	if err := json.Unmarshal(b, &old); err == nil && len(old.Profiles) > 0 {
		servers := make([]Server, 0, len(old.Profiles))
		var activeServerID string
		for i, p := range old.Profiles {
			p = withDefaults(p)
			srv := fromProfileToServer(p)
			cli := fromProfileToClient(p)
			srv.ID = newServerID()
			cli.ID = newClientID()
			if strings.TrimSpace(srv.Name) == "" {
				srv.Name = fmt.Sprintf("Server %d", i+1)
			}
			if strings.TrimSpace(cli.Name) == "" {
				cli.Name = "Client 1"
			}
			srv.Clients = []Client{cli}
			srv.ActiveClientID = cli.ID
			servers = append(servers, withServerDefaults(srv, i+1))
			if p.ID == old.ActiveProfileID || (old.ActiveProfileID == "" && i == 0) {
				activeServerID = srv.ID
			}
		}
		if activeServerID == "" {
			activeServerID = servers[0].ID
		}
		s.data = fileData{
			ActiveServerID: activeServerID,
			Servers:        servers,
		}
		return nil
	}

	// Legacy single profile.
	var legacy Profile
	if err := json.Unmarshal(b, &legacy); err != nil {
		return err
	}
	legacy = withDefaults(legacy)
	srv := withServerDefaults(fromProfileToServer(legacy), 1)
	cli := fromProfileToClient(legacy)
	cli.ID = newClientID()
	cli.Name = choose(legacy.ClientName, "Client 1")
	srv.Clients = []Client{cli}
	srv.ActiveClientID = cli.ID
	s.data = fileData{
		ActiveServerID: srv.ID,
		Servers:        []Server{srv},
	}
	return nil
}

func (s *Store) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(profileFilePath(), b, 0644)
}

// ---- helpers ----

func mergeToProfile(srv Server, cli Client) Profile {
	cli.CoverLists = normalizeCoverLists(cli.CoverLists, cli.CoverSites, cli.BehaviorProfile)
	cli.ActiveCoverListID = pickActiveCoverListID(cli.CoverLists, cli.ActiveCoverListID)
	active := activeCoverList(cli.CoverLists, cli.ActiveCoverListID)
	return Profile{
		ID:                  cli.ID,
		Name:                cli.Name,
		ServerID:            srv.ID,
		ServerName:          srv.Name,
		ClientID:            cli.ID,
		ClientName:          cli.Name,
		VPSHost:             srv.VPSHost,
		VPSPort:             srv.VPSPort,
		VPSUser:             srv.VPSUser,
		VPSPassword:         srv.VPSPassword,
		Deployed:            srv.Deployed,
		AWGListenPort:       srv.AWGListenPort,
		AWGClientPrivKey:    cli.AWGClientPrivKey,
		AWGClientPubKey:     cli.AWGClientPubKey,
		AWGServerPubKey:     srv.AWGServerPubKey,
		AWGClientConfig:     cli.AWGClientConfig,
		AWGExePath:          cli.AWGExePath,
		AWGInterface:        cli.AWGInterface,
		ActiveDecoyDomain:   srv.ActiveDecoyDomain,
		DecoyDomains:        append([]string(nil), srv.DecoyDomains...),
		HotSwapEnabled:      srv.HotSwapEnabled,
		HotSwapInterval:     srv.HotSwapInterval,
		LMStudioModel:       cli.LMStudioModel,
		AISystemPrompt:      cli.AISystemPrompt,
		CoverLists:          cli.CoverLists,
		ActiveCoverListID:   cli.ActiveCoverListID,
		CoverSites:          append([]string(nil), active.Sites...),
		BehaviorProfile:     active.Name,
		Intensity:           cli.Intensity,
		SessionMinutes:      cli.SessionMinutes,
		MemoryTimeoutPolicy: cli.MemoryTimeoutPolicy,
		AdaptiveControl:     cli.AdaptiveControl,
		AdaptiveMode:        cli.AdaptiveMode,
	}
}

func fromProfileToServer(p Profile) Server {
	return Server{
		ID:                choose(p.ServerID, newServerID()),
		Name:              choose(p.ServerName, "Server"),
		VPSHost:           p.VPSHost,
		VPSPort:           p.VPSPort,
		VPSUser:           p.VPSUser,
		VPSPassword:       p.VPSPassword,
		Deployed:          p.Deployed,
		AWGListenPort:     p.AWGListenPort,
		AWGServerPubKey:   p.AWGServerPubKey,
		ActiveDecoyDomain: p.ActiveDecoyDomain,
		DecoyDomains:      append([]string(nil), p.DecoyDomains...),
		HotSwapEnabled:    p.HotSwapEnabled,
		HotSwapInterval:   p.HotSwapInterval,
	}
}

func fromProfileToClient(p Profile) Client {
	lists := normalizeCoverLists(p.CoverLists, p.CoverSites, p.BehaviorProfile)
	activeID := pickActiveCoverListID(lists, p.ActiveCoverListID)
	active := activeCoverList(lists, activeID)
	return Client{
		ID:                  choose(p.ClientID, choose(p.ID, newClientID())),
		Name:                choose(p.ClientName, choose(p.Name, "Client")),
		AWGClientPrivKey:    p.AWGClientPrivKey,
		AWGClientPubKey:     p.AWGClientPubKey,
		AWGClientConfig:     p.AWGClientConfig,
		AWGExePath:          p.AWGExePath,
		AWGInterface:        p.AWGInterface,
		LMStudioModel:       p.LMStudioModel,
		AISystemPrompt:      p.AISystemPrompt,
		CoverLists:          lists,
		ActiveCoverListID:   activeID,
		CoverSites:          append([]string(nil), active.Sites...),
		BehaviorProfile:     active.Name,
		Intensity:           p.Intensity,
		SessionMinutes:      p.SessionMinutes,
		MemoryTimeoutPolicy: p.MemoryTimeoutPolicy,
		AdaptiveControl:     p.AdaptiveControl,
		AdaptiveMode:        p.AdaptiveMode,
	}
}

func defaultMergedProfile() Profile {
	srv := defaultServer()
	return mergeToProfile(srv, srv.Clients[0])
}

func withDefaults(p Profile) Profile {
	d := defaultMergedProfile()

	p.ID = choose(p.ID, d.ID)
	p.Name = choose(p.Name, d.Name)
	p.ServerID = choose(p.ServerID, d.ServerID)
	p.ServerName = choose(p.ServerName, d.ServerName)
	p.ClientID = choose(p.ClientID, d.ClientID)
	p.ClientName = choose(p.ClientName, d.ClientName)
	if p.VPSPort == 0 {
		p.VPSPort = d.VPSPort
	}
	p.VPSUser = choose(p.VPSUser, d.VPSUser)
	p.AWGExePath = choose(p.AWGExePath, d.AWGExePath)
	p.AWGInterface = choose(p.AWGInterface, d.AWGInterface)
	if p.HotSwapInterval == 0 {
		p.HotSwapInterval = d.HotSwapInterval
	}
	if p.DecoyDomains == nil {
		p.DecoyDomains = append([]string(nil), d.DecoyDomains...)
	}
	p.CoverLists = normalizeCoverLists(p.CoverLists, p.CoverSites, p.BehaviorProfile)
	p.ActiveCoverListID = pickActiveCoverListID(p.CoverLists, p.ActiveCoverListID)
	active := activeCoverList(p.CoverLists, p.ActiveCoverListID)
	p.CoverSites = append([]string(nil), active.Sites...)
	p.BehaviorProfile = active.Name
	if p.Intensity == "" {
		p.Intensity = d.Intensity
	}
	if p.SessionMinutes == 0 {
		p.SessionMinutes = d.SessionMinutes
	}
	if strings.TrimSpace(p.MemoryTimeoutPolicy) == "" {
		p.MemoryTimeoutPolicy = d.MemoryTimeoutPolicy
	}
	if strings.TrimSpace(p.AdaptiveMode) == "" {
		p.AdaptiveMode = d.AdaptiveMode
		p.AdaptiveControl = true
	}
	return p
}

func withServerDefaults(srv Server, idx int) Server {
	ds := defaultServer()
	srv.ID = choose(srv.ID, newServerID())
	if strings.TrimSpace(srv.Name) == "" {
		srv.Name = fmt.Sprintf("Server %d", idx)
	}
	if srv.VPSPort == 0 {
		srv.VPSPort = ds.VPSPort
	}
	srv.VPSUser = choose(srv.VPSUser, ds.VPSUser)
	if srv.DecoyDomains == nil {
		srv.DecoyDomains = append([]string(nil), ds.DecoyDomains...)
	}
	if srv.HotSwapInterval == 0 {
		srv.HotSwapInterval = ds.HotSwapInterval
	}
	if len(srv.Clients) == 0 {
		c := defaultClient()
		srv.Clients = []Client{c}
		srv.ActiveClientID = c.ID
	}
	for i := range srv.Clients {
		srv.Clients[i] = withClientDefaults(srv.Clients[i], i+1)
	}
	if srv.ActiveClientID == "" || findClientIndex(srv.Clients, srv.ActiveClientID) < 0 {
		srv.ActiveClientID = srv.Clients[0].ID
	}
	return srv
}

func withClientDefaults(c Client, idx int) Client {
	d := defaultClient()
	c.ID = choose(c.ID, newClientID())
	if strings.TrimSpace(c.Name) == "" {
		c.Name = fmt.Sprintf("Client %d", idx)
	}
	c.AWGExePath = choose(c.AWGExePath, d.AWGExePath)
	c.AWGInterface = choose(c.AWGInterface, d.AWGInterface)
	c.CoverLists = normalizeCoverLists(c.CoverLists, c.CoverSites, c.BehaviorProfile)
	c.ActiveCoverListID = pickActiveCoverListID(c.CoverLists, c.ActiveCoverListID)
	active := activeCoverList(c.CoverLists, c.ActiveCoverListID)
	c.CoverSites = append([]string(nil), active.Sites...)
	c.BehaviorProfile = active.Name
	if c.Intensity == "" {
		c.Intensity = d.Intensity
	}
	if c.SessionMinutes == 0 {
		c.SessionMinutes = d.SessionMinutes
	}
	if strings.TrimSpace(c.MemoryTimeoutPolicy) == "" {
		c.MemoryTimeoutPolicy = d.MemoryTimeoutPolicy
	}
	if strings.TrimSpace(c.AdaptiveMode) == "" {
		c.AdaptiveMode = d.AdaptiveMode
		c.AdaptiveControl = true
	}
	return c
}

func normalizeCoverLists(lists []CoverList, legacySites []string, legacyName string) []CoverList {
	if len(lists) == 0 {
		if len(legacySites) > 0 {
			id := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(legacyName), " ", "_"))
			if id == "" {
				id = "custom"
			}
			name := strings.TrimSpace(legacyName)
			if name == "" {
				name = "Custom"
			}
			return []CoverList{{ID: id, Name: name, Sites: append([]string(nil), legacySites...)}}
		}
		return append([]CoverList(nil), defaultCoverLists()...)
	}
	out := make([]CoverList, 0, len(lists))
	for i, l := range lists {
		if strings.TrimSpace(l.ID) == "" {
			l.ID = fmt.Sprintf("list-%d", i+1)
		}
		if strings.TrimSpace(l.Name) == "" {
			l.Name = fmt.Sprintf("List %d", i+1)
		}
		if l.Sites == nil {
			l.Sites = []string{}
		}
		out = append(out, CoverList{
			ID:    l.ID,
			Name:  l.Name,
			Sites: append([]string(nil), l.Sites...),
		})
	}
	return out
}

func pickActiveCoverListID(lists []CoverList, want string) string {
	if len(lists) == 0 {
		return ""
	}
	if want != "" {
		for _, l := range lists {
			if l.ID == want {
				return want
			}
		}
	}
	return lists[0].ID
}

func activeCoverList(lists []CoverList, activeID string) CoverList {
	if len(lists) == 0 {
		return CoverList{ID: "none", Name: "Custom", Sites: []string{}}
	}
	for _, l := range lists {
		if l.ID == activeID {
			return l
		}
	}
	return lists[0]
}

func cloneServerTemplate(srv Server) Server {
	return Server{
		Name:            srv.Name,
		VPSPort:         srv.VPSPort,
		VPSUser:         srv.VPSUser,
		DecoyDomains:    append([]string(nil), srv.DecoyDomains...),
		HotSwapEnabled:  srv.HotSwapEnabled,
		HotSwapInterval: srv.HotSwapInterval,
	}
}

func cloneClientTemplate(c Client) Client {
	return Client{
		Name:                c.Name,
		AWGExePath:          c.AWGExePath,
		AWGInterface:        c.AWGInterface,
		LMStudioModel:       c.LMStudioModel,
		AISystemPrompt:      c.AISystemPrompt,
		CoverLists:          normalizeCoverLists(c.CoverLists, c.CoverSites, c.BehaviorProfile),
		ActiveCoverListID:   c.ActiveCoverListID,
		CoverSites:          append([]string(nil), c.CoverSites...),
		BehaviorProfile:     c.BehaviorProfile,
		Intensity:           c.Intensity,
		SessionMinutes:      c.SessionMinutes,
		MemoryTimeoutPolicy: c.MemoryTimeoutPolicy,
		AdaptiveControl:     c.AdaptiveControl,
		AdaptiveMode:        c.AdaptiveMode,
	}
}

func (s *Store) activeServerLocked() (Server, bool) {
	idx := s.findServerIndexLocked(s.data.ActiveServerID)
	if idx < 0 {
		return Server{}, false
	}
	return s.data.Servers[idx], true
}

func (s *Store) activeServerClientLocked() (Server, Client, bool) {
	srv, ok := s.activeServerLocked()
	if !ok {
		return Server{}, Client{}, false
	}
	cidx := findClientIndex(srv.Clients, srv.ActiveClientID)
	if cidx < 0 {
		return Server{}, Client{}, false
	}
	return srv, srv.Clients[cidx], true
}

func (s *Store) setServerClientLocked(srv Server, cli Client) {
	cidx := findClientIndex(srv.Clients, cli.ID)
	if cidx >= 0 {
		srv.Clients[cidx] = cli
	}
	s.setActiveServerLocked(srv)
}

func (s *Store) setActiveServerLocked(srv Server) {
	sidx := s.findServerIndexLocked(srv.ID)
	if sidx >= 0 {
		s.data.Servers[sidx] = srv
	}
}

func (s *Store) findServerIndexLocked(id string) int {
	return findServerIndex(s.data.Servers, id)
}

func findServerIndex(servers []Server, id string) int {
	for i := range servers {
		if servers[i].ID == id {
			return i
		}
	}
	return -1
}

func findClientIndex(clients []Client, id string) int {
	for i := range clients {
		if clients[i].ID == id {
			return i
		}
	}
	return -1
}

func choose(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func newServerID() string { return fmt.Sprintf("s-%d", time.Now().UnixNano()) }
func newClientID() string { return fmt.Sprintf("c-%d", time.Now().UnixNano()) }

func profileFilePath() string {
	exe, err := os.Executable()
	if err != nil || strings.TrimSpace(exe) == "" {
		return profileFileName
	}
	return filepath.Join(filepath.Dir(exe), profileFileName)
}
