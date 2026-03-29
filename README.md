# Furor Davidis

> *«Ira sine ratione — bestia est. Ira cum ratione — gladius.»*
> Rage without reason is a beast. Rage with reason — is a sword.

**Furor Davidis** is a Windows application that deploys an AmneziaWG VPN tunnel and simultaneously runs a local AI model that generates realistic cover traffic — making your VPN connection invisible to Deep Packet Inspection.

One click sets up a VPS. One click connects. Then a 1.7 billion parameter mind begins its quiet work.

[Русская версия](README.ru.md)

---

## The Name. The Idea.

**David and Goliath** — a small shepherd with a sling against an armored giant.

The modern equivalent: your ISP's AI cluster — petabytes of training data, thousands of GPUs, billions of parameters — classifies every packet you send in milliseconds. That is Goliath.

David is a 1.7B parameter model that fits in 1.1 GB of RAM, running on your CPU.

The outcome seems predetermined. It is not.

**Furor** — this is not anger. Not frustration. Not impulse. It is a specific state: the mind becomes absolutely clear, free of emotion, operating as pure intellect — but powered by rage as fuel. A warrior in this state does not swing wildly. He thinks faster. Sees more precisely. Strikes where it matters.

The small model does not try to match Goliath in scale. It fights asymmetrically — it does not need to be bigger, it needs to be *specific*. It generates exactly the kind of browsing pattern that neutralizes your particular DPI, in your particular network, at this particular hour of the day. And it learns. Every session it gets a little better at fighting your Goliath specifically.

*Scrutator frustra laborat.* The inspector labors in vain.

---

## From the Author

WireGuard and AmneziaWG are my daily working tools. I build communication channels between people and infrastructure scattered across geographies. I know how networks work — not as a programmer, but as a systems engineer who has stood up servers from bare metal more times than he can count.

The problem of DPI is not theoretical for me. It is the difference between a connection that works and one that does not.

The idea behind Furor Davidis came from a simple observation: DPI systems are increasingly AI-driven. They learn patterns. The classical answer — obfuscation — is an arms race where the defender always moves second. I wanted to flip that. Instead of hiding the tunnel, confuse the classifier. Flood it with plausible signal. Make the "suspicious" traffic disappear into a sea of legitimate-looking HTTPS that follows real human browsing patterns.

A local model on the client machine means zero external dependencies, zero API keys, zero data leaving your machine. The AI runs on your hardware, generates patterns for your network, learns from your results.

This is vibe-coding — the architecture is mine, the implementation was built in pair with AI. Fully open source.

---

## Architecture

```
┌──────────────────────────────────────────────────────┐
│  FUROR DAVIDIS (Windows client)                      │
│                                                      │
│  [Monitor] ─→ AWG interface RTT / bytes / session    │
│      ↓                                               │
│  [Rules Engine] (deterministic Go)                   │
│      RTT +20% → trigger cover batch                  │
│      session > 30min → HotSwap decoy domain          │
│      ↓                                               │
│  [AI Engine] ─→ LM Studio API (HTTP:1234)            │
│      input:  JSON metrics + RAG memory context       │
│      output: JSON sequence [{url, read_sec}, ...]    │
│      model:  any instruct model loaded in LM Studio  │
│      ↓                                               │
│  [Cover Executor]                                    │
│      bind → physical adapter IP (bypass AWG)         │
│      real TLS/HTTPS to real domains                  │
│      Chrome TLS fingerprint (utls JA3/JA4)          │
│      DNS → 1.1.1.1 via physical adapter             │
│      session cookies + Referer chain                 │
│      timing: read_sec + jitter ±20%                  │
│      ↓                                               │
│  [HotSwap Controller]                                │
│      SSH → VPS → docker kill --signal HUP xray      │
│      rotation: microsoft.com / youtube.com / ...     │
│      post-verify: TCP:443 check after swap           │
│                                                      │
└──────────────────────────────────────────────────────┘
         ↓ parallel, independent ↓
┌──────────────────────────────────────────────────────┐
│  AWG TUNNEL (amneziawg.exe)                          │
│  UDP → VPS : random port 40000–65000                 │
│  junk obfuscation (Jc/Jmin/Jmax/S1/S2/H1–H4)       │
└──────────────────────────────────────────────────────┘
         ↓ on the server ↓
┌──────────────────────────────────────────────────────┐
│  VPS (Ubuntu)                                        │
│  AWG server endpoint (UDP)                           │
│  xray DECOY (TCP:443, dokodemo-door)                 │
│      observer probe → TLS → forwards to DecoyDomain  │
│      NOT a tunnel — camouflage only                  │
│  dnsmasq — DNS through the tunnel                    │
│  fail2ban — SSH brute-force protection               │
└──────────────────────────────────────────────────────┘
```

### What the ISP sees

```
Physical adapter:
  → UDP : 4XXXX      → VPS              (AWG, unknown protocol)
  → TCP : 443        → github.com       (cover, real TLS)
  → TCP : 443        → wikipedia.org    (cover, real TLS)
  → TCP : 443        → docs.python.org  (cover, real TLS)

DPI classifier: 60% normal web browsing + 40% unknown UDP → "acceptable"
```

### What makes the cover traffic convincing

- **Real TLS connections** to real domains — not simulated, not replayed. Actual HTTPS GET requests with full TLS handshake.
- **Chrome fingerprint** — Go's built-in TLS has a different JA3/JA4 signature than a browser. `utls` (refraction-networking/utls) impersonates Chrome 120 exactly.
- **Correct DNS path** — DNS queries for cover domains go through the physical adapter to 1.1.1.1, not through the AWG tunnel. ISP sees DNS query followed by matching HTTPS — exactly like a browser.
- **Session cookies** — all requests in a session share one cookie jar. Navigation from `github.com` to `github.com/search` carries cookies and Referer. The request chain looks like a single user session.
- **Human timing** — the AI sets `read_sec` per URL based on content type and estimated reading time. Jitter ±20% is applied. No machine-gun pattern.

---

## The AI Engine

The model does one specific job: given a behavior profile, current network state, and time of day — generate a realistic sequence of URLs with reading times.

```json
Input:
{
  "profile": "developer",
  "session_min": 20,
  "sites": ["github.com", "wikipedia.org", "docs.python.org"],
  "rtt_trend": "rising",
  "hour": 14
}

Output:
[
  {"url": "github.com/golang/go/blob/master/src/net/http/transport.go", "read_sec": 67},
  {"url": "en.wikipedia.org/wiki/Transport_Layer_Security", "read_sec": 94},
  {"url": "docs.python.org/3/library/asyncio.html", "read_sec": 41}
]
```

The model does not make real-time decisions about the tunnel. The Rules Engine (deterministic Go code) decides *when* to act. The model decides *what* the browsing session looks like. This division of responsibility is deliberate: LLMs are excellent at generating plausible human-like sequences; they are unreliable as real-time control logic.

### Editable Cover Lists

| Item | Meaning |
|---|---|
| Named lists | Create/rename/delete your own domain lists |
| Active list per client | Every client profile stores its own active list |
| HotSwap sync | Decoy domain pool is synced from active AI Cover list |
| Capacity | UI allows long lists; AI takes an adaptive subset per session |

---

## RAG Memory — The Learning Component

After each cover session, the application measures what happened to the AWG tunnel RTT over the next 5 minutes. If RTT stabilized or dropped — the session was effective. If RTT increased or the connection dropped — it was not.

This outcome is scored and stored in `furor_memory.json`.

```
Record(context, action)
       ↓
  [5 min later]
       ↓
Evaluate(RTT delta)
  RTT dropped  → score +0.3  → outcome: success
  RTT rose     → score −0.3  → outcome: failure
  no change    → outcome: neutral

Score decays 1%/day → old data fades, recent wins count more
Cap at 500 entries → trim by lowest score
```

Before generating a new session, the AI receives the top-scoring historical entries that match the current context (same time of day, same day type, same RTT trend). It learns that `github.com` browsing at 14:00 on a weekday tends to work well in your network. It learns that `youtube.com` at 23:00 with a rising RTT trend worked before.

This is not fine-tuning. It is in-context learning — injecting what worked into the system prompt. The model gets smarter about your specific network without touching its weights.

`furor_memory.json` is portable. Copy it between machines.

---

## Features

| Feature | Description |
|---|---|
| **Deploy** | Automated AWG + xray decoy deployment to Ubuntu VPS over SSH |
| **Multi-VPS / Multi-client profiles** | One app can manage multiple servers, each with multiple client profiles |
| **Connect / Disconnect** | Connection state button uses `Libertad` / `Libero`; disconnect also stops AI Cover |
| **AI Cover** | Start/stop the AI orchestrator — cover traffic generation loop |
| **Adaptive live control** | Dynamic tuning of session length, URL count, chain depth, RAG weight and timeout policy |
| **Prompt safety editor** | View-only by default, double warning before edit, validation, reset to recommended |
| **HotSwap** | Rotate the server's decoy domain via SSH without dropping the AWG tunnel (~100ms reload) |
| **RAG Memory** | Persistent learning — what worked in your network is remembered and reused |
| **Memory Export/Import** | `furor_memory.json` — portable, shareable between machines |
| **Diagnostics** | Check presence of all required local files and server service status |
| **Editable cover lists** | Named lists per client profile with active-list sync into HotSwap decoy pool |
| **Chrome TLS fingerprint** | `utls` HelloChrome_Auto — JA3/JA4 identical to Chrome 120 |
| **DNS leak protection** | Cover traffic DNS also goes through physical adapter (1.1.1.1), not AWG |
| **dnsmasq on VPS** | DNS for tunnel traffic routes through VPS, installed automatically |
| **fail2ban** | SSH brute-force protection on VPS (maxretry=5, ban 1 hour) |
| **Post-HotSwap verify** | TCP:443 check after every decoy domain rotation |

---

## Requirements

### Client (Windows)

- Windows 10/11, **Administrator rights** (required for route injection)
- Place the following next to `FurorDavidis.exe`:

```
FurorDavidis.exe
awg\
    amneziawg.exe
    wintun.dll
```

- Install **LM Studio** and run local server on `http://localhost:1234`

### Server

- Ubuntu **22.04** or **24.04**
- SSH access (root, password)
- One open UDP port (random, 40000–65000 range) — the app handles this
- Docker is not required — deployment installs everything automatically

---

## Quick Start

### 1. Download Dependencies

Go to the **Diagnostics** tab — the download links are there.

| File | Size | Source |
|---|---|---|
| `amneziawg.exe` + `wintun.dll` | ~5 MB | [github.com/amnezia-vpn](https://github.com/amnezia-vpn/amneziawg-windows) |
| `LM Studio` | app | [lmstudio.ai](https://lmstudio.ai/) |
| `Qwen3-1.7B-Q4_K_M.gguf` | ~1.1 GB | Search **huggingface.co** → `Qwen3-1.7B GGUF Q4_K_M` |
| `Qwen2.5-1.5B-Instruct-Q4_K_M.gguf` | ~1.0 GB | Stable alternative — search `Qwen2.5-1.5B Instruct GGUF` |
| `Qwen3-0.6B-Q4_K_M.gguf` | ~400 MB | Search `Qwen3-0.6B GGUF Q4_K_M` — if RAM < 4 GB |

> GGUF repositories change often — search by model name directly on [huggingface.co/models](https://huggingface.co/models), filter by `GGUF`. The file name matters for the Settings path, not the source repo.

In LM Studio: load your model, open **Local Server**, start the API server on port `1234`.

### 2. Unblock

Windows blocks downloaded executables via Zone.Identifier. Right-click each `.exe` → **Properties** → **Unblock** → OK. Or in PowerShell:
```powershell
Get-ChildItem -Recurse *.exe | Unblock-File
```

### 3. Deploy the Server

**Deploy** tab:
- Enter VPS host, SSH port, user, password
- Set decoy domains (defaults: microsoft.com, youtube.com, github.com...)
- Click **¡Viva la libertad** — takes 3–7 minutes

The log streams all stages: AWG install, xray decoy configuration, UFW, dnsmasq, fail2ban, final verification. Deploy starts with `Launching Supremo...`.

### 4. Start AI Cover

**AI Cover** tab:
- Check/edit your cover list
- Click **Start**

### 5. Connect

**Connect** tab → **Libertad**.
When connected, label changes to **Libero**. Disconnect also stops AI Cover.

The AI orchestrator begins its loop. Every 60 seconds it checks tunnel metrics. If RTT trends upward — it generates a cover session and fires it out through your physical adapter, parallel to the AWG tunnel.

---

## HotSwap — Decoy Domain Rotation

The xray process on the server runs as a `dokodemo-door` that forwards all TCP:443 traffic to a legitimate domain (the decoy). When a DPI system or an observer probes your VPS on port 443, they get a valid TLS handshake with `microsoft.com`'s certificate — because they are actually talking to microsoft.com through your server.

**HotSwap** changes this decoy domain without restarting xray:

```
SSH → sed config.json → docker kill --signal HUP xray
```

xray reloads its config in ~100ms. No connection drop. The AWG tunnel is untouched.

After each swap, the application checks that TCP:443 on the VPS still responds. If not — a warning appears in the log. The tunnel keeps working.

---

## Settings

**Settings** tab:

| Setting | Description |
|---|---|
| `Model override (optional)` | Exact model name loaded in LM Studio |
| `amneziawg.exe path` | Path to AWG client executable |
| `Interface name` | AWG interface name (default: `furor`) |
| `HotSwap enabled` | Automatic decoy rotation |
| `HotSwap interval` | Minutes between automatic rotations (default: 30) |
| `Adaptive control` | Enable live auto-tuning for AI session parameters |
| `Adaptive mode` | `conservative` / `balanced` / `aggressive` |
| `RAG timeout penalty` | `low/base/high` timeout score penalty |

---

## Building from Source

```bash
# Install Wails CLI (no Node.js required — frontend is vanilla JS)
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Production build (stripped, no console window)
cd FurorDavidis
wails build -ldflags="-s -w -H windowsgui"

# Output: build\bin\FurorDavidis.exe
```

> After building — **unblock the executable** (Zone.Identifier).
> Run as **Administrator**.

---

## Acknowledgements

**[AmneziaWG / Amnezia Team](https://github.com/amnezia-vpn/amneziawg-go)**
The only tunnel in this stack. Header junk obfuscation, open source, actively maintained. You made WireGuard actually work in places where it gets blocked. This project exists because yours does.

**[Qwen / Alibaba Cloud](https://github.com/QwenLM/Qwen3)**
A 1.7B model that generates coherent, structured JSON fast enough to fit into a 60-second control loop on a consumer CPU. That is not a small achievement. The 0.6B variant runs on hardware that has no business running a language model at all.

**[LM Studio](https://lmstudio.ai/)**
Local OpenAI-compatible runtime used by Furor Davidis. Keeps model execution on the client machine.

**[utls / refraction-networking](https://github.com/refraction-networking/utls)**
Go's built-in TLS stack is immediately identifiable by its JA3 fingerprint. utls impersonates real browser TLS handshakes. Without it, the cover traffic would be as suspicious as the tunnel.

**[XTLS / Xray-core](https://github.com/XTLS/Xray-core)**
Used here only as a decoy — a `dokodemo-door` that makes a VPS look like a legitimate website to observers. A small part of what xray can do, but exactly the right part.

**[AmneziaWG Windows Client](https://github.com/amnezia-vpn/amneziawg-windows)**
The Windows-native AWG client that handles kernel interface management cleanly.

**[WireGuard / Jason Donenfeld](https://github.com/WireGuard/wireguard-go)**
The foundation everything else stands on.

**[Wails](https://github.com/wailsapp/wails)**
A Go framework for building native Windows applications with a web frontend. No Node.js required for a vanilla JS project. Made this possible without leaving the Go ecosystem.

**[teddysun/xray Docker image](https://hub.docker.com/r/teddysun/xray)**
Clean, maintained Docker image for xray on the server side.

**[Linux / Linus Torvalds and the kernel community](https://kernel.org)**
Every VPS in this stack runs Linux. An operating system that runs the entire internet, started by one person in 1991 and built by thousands since. I work with it every day. I never stop being grateful for it.

---

## Manifesto

*«Ira sine ratione — bestia est. Ira cum ratione — gladius.»*

The DPI systems deployed to surveil and restrict internet access are large, expensive, and sophisticated. That is the point: they are designed to win through scale. An arms race on those terms — bigger obfuscation vs bigger classifiers — is a race the defender cannot win.

But scale has a weakness. It has to be general. It classifies traffic across millions of users with patterns trained on averages. A local AI fighting for one specific connection, in one specific network, at one specific hour — is not an average. It is a specific anomaly that the general model was not trained to catch.

David did not need to be stronger than Goliath. He needed to be faster, more precise, and to hit exactly the right point.

Furor Davidis is that sling. A small, focused, learning system — running on your machine, for your network, carrying your traffic — against a system that does not know you exist.

*Scrutator frustra laborat.*

---

## License

Released under the **[MIT License](LICENSE)** — take it, use it, modify it, share it.

Built on the shoulders of open source: AmneziaWG, Xray-core, Qwen, utls, Wails, and dozens of other projects. Closing off something grown from open code would be dishonest.

All dependencies carry their own licenses — see the respective repositories.

Use responsibly.

---

<p align="center">
<em>«Furor Davidis»</em><br>
The fury of David.<br>
<br>
<em>Not the fury that blinds — the fury that focuses.</em>
</p>
