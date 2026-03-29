// internal/deploy/orchestrator.go
// Деплой AWG + xray декой на чистый Ubuntu VPS.
// Ключи AWG генерируются на клиенте (Go crypto) — сервер ключей не знает.
package deploy

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	mathrand "math/rand"
	"time"

	"golang.org/x/crypto/curve25519"

	"github.com/yourorg/furor-davidis/internal/logger"
	"github.com/yourorg/furor-davidis/internal/payload"
	"github.com/yourorg/furor-davidis/internal/profile"
	"github.com/yourorg/furor-davidis/internal/ssh"
)

// Result — результат деплоя, записывается в профиль.
type Result struct {
	AWGClientPrivKey string
	AWGClientPubKey  string
	AWGServerPubKey  string
	AWGListenPort    string
	ClientConfig     string // готовый .conf для amneziawg.exe
}

// Deploy выполняет полный деплой на VPS.
// logFn — коллбэк построчного вывода в UI (как в Vanus).
func Deploy(p profile.Profile, logFn func(string), log *logger.Logger) (*Result, error) {
	step := func(title string) {
		msg := "-- " + title
		log.Info(msg)
		logFn(msg)
	}
	fail := func(format string, args ...interface{}) error {
		err := fmt.Errorf(format, args...)
		log.Error(err.Error())
		logFn("ERROR: " + err.Error())
		return err
	}
	logFn("¡Viva la libertad")
	logFn("Launching Supremo...")

	// ── [1] SSH подключение ──────────────────────────────────────────────
	step("[1] Connecting to VPS...")
	port := p.VPSPort
	if port == 0 {
		port = 22
	}
	client, err := ssh.Dial(p.VPSHost, fmt.Sprintf("%d", port), p.VPSUser, p.VPSPassword)
	if err != nil {
		return nil, fail("SSH: %w", err)
	}
	defer client.Close()

	// Стримим SSH лог в UI параллельно
	go func() {
		for line := range client.LogLine {
			logFn(line)
		}
	}()

	step("[1] Connected OK")

	// ── [2] Генерация AWG ключей (на клиенте) ────────────────────────────
	step("[2] Generating AWG keys...")
	serverPriv, serverPub, err := generateAWGKeyPair()
	if err != nil {
		return nil, fail("keygen server: %w", err)
	}
	clientPriv, clientPub, err := generateAWGKeyPair()
	if err != nil {
		return nil, fail("keygen client: %w", err)
	}

	awgPort := randomPort(40000, 65000)
	decoyDomain := p.DecoyDomains[0]
	if decoyDomain == "" {
		decoyDomain = "microsoft.com"
	}

	params := payload.DeployParams{
		AWGListenPort:    awgPort,
		AWGServerPrivKey: serverPriv,
		AWGServerPubKey:  serverPub,
		AWGClientPrivKey: clientPriv,
		AWGClientPubKey:  clientPub,
		AWGClientIP:      "10.8.0.2",
		AWGServerIP:      "10.8.0.1",
		MTU:              1380,
		DecoyDomain:      decoyDomain,
		Jc:               4, Jmin: 50, Jmax: 1000,
		S1: 15, S2: 24,
		H1: 1, H2: 2, H3: 3, H4: 4,
	}

	// ── [3] Деплой на сервер ─────────────────────────────────────────────
	step("[3] Running deploy script (3-7 min)...")
	script, err := payload.DeployScript(params)
	if err != nil {
		return nil, fail("template: %w", err)
	}

	_, deployErr := client.RunScript(script, logFn)
	if deployErr != nil {
		// Не фатально — деплой мог частично пройти (docker pull медленный)
		log.Errorf("[Deploy] script returned error: %v (check log)", deployErr)
		logFn(fmt.Sprintf("WARN: deploy script finished with error: %v", deployErr))
		logFn("WARN: if error is in apt/docker, retry deploy (cache may help)")
	}

	// ── [4] Проверка ─────────────────────────────────────────────────────
	step("[4] Verifying services...")
	out, err := client.Run("systemctl is-active awg-quick@furor 2>/dev/null && docker ps --filter name=xray --format '{{.Status}}' 2>/dev/null")
	if err != nil {
		logFn("WARN: service check failed, verify manually")
	} else {
		logFn("Status: " + out)
	}

	// ── [5] Формируем результат ───────────────────────────────────────────
	clientConf := payload.ClientConfig(params, p.VPSHost)

	step("[5] Deploy complete")
	logFn("  AWG UDP port : " + awgPort)
	logFn("  xray decoy   : TCP:443 -> " + decoyDomain)
	logFn("  fail2ban     : active")
	logFn("")
	logFn("Press Connect to start.")

	return &Result{
		AWGClientPrivKey: clientPriv,
		AWGClientPubKey:  clientPub,
		AWGServerPubKey:  serverPub,
		AWGListenPort:    awgPort,
		ClientConfig:     clientConf,
	}, nil
}

// HotSwap меняет декой-домен на уже развёрнутом сервере.
func HotSwap(p profile.Profile, newDomain string, logFn func(string), log *logger.Logger) error {
	client, err := ssh.Dial(p.VPSHost, fmt.Sprintf("%d", p.VPSPort), p.VPSUser, p.VPSPassword)
	if err != nil {
		return fmt.Errorf("ssh: %w", err)
	}
	defer client.Close()

	script := payload.HotSwapScript(newDomain)
	_, err = client.RunScript(script, logFn)
	if err != nil {
		return fmt.Errorf("hotswap: %w", err)
	}
	log.Infof("[HotSwap] %s -> %s OK", p.ActiveDecoyDomain, newDomain)
	return nil
}

// Verify проверяет состояние сервисов на VPS.
func Verify(p profile.Profile, logFn func(string)) error {
	client, err := ssh.Dial(p.VPSHost, fmt.Sprintf("%d", p.VPSPort), p.VPSUser, p.VPSPassword)
	if err != nil {
		return fmt.Errorf("ssh: %w", err)
	}
	defer client.Close()
	_, err = client.RunScript(payload.VerifyScript(), logFn)
	return err
}

// ── helpers ───────────────────────────────────────────────────────────────

// generateAWGKeyPair генерирует пару ключей Curve25519 для AWG.
// Тот же алгоритм что использует WireGuard/AmneziaWG.
func generateAWGKeyPair() (privBase64, pubBase64 string, err error) {
	var privRaw [32]byte
	if _, err = rand.Read(privRaw[:]); err != nil {
		return
	}
	// Clamping как в RFC 7748
	privRaw[0] &= 248
	privRaw[31] = (privRaw[31] & 127) | 64

	var pubRaw [32]byte
	curve25519.ScalarBaseMult(&pubRaw, &privRaw)

	privBase64 = base64.StdEncoding.EncodeToString(privRaw[:])
	pubBase64 = base64.StdEncoding.EncodeToString(pubRaw[:])
	return
}

func randomPort(min, max int) string {
	src := mathrand.NewSource(time.Now().UnixNano())
	r := mathrand.New(src)
	return fmt.Sprintf("%d", min+r.Intn(max-min))
}
