// internal/connect/manager.go
// AWG Connect/Disconnect — без tun2socks, без матрёшки.
// Запускаем amneziawg.exe с конфигом → маршруты → подключено.
package connect

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/yourorg/furor-davidis/internal/logger"
	"github.com/yourorg/furor-davidis/internal/routing"
	"golang.org/x/text/encoding/charmap"
)

const awgConfFile = "furor.conf"

// Manager управляет AWG тунелем.
type Manager struct {
	log    *logger.Logger
	proc   *exec.Cmd
	vpsIP  string
	awgExe string
}

func NewManager(log *logger.Logger, awgExe string) *Manager {
	return &Manager{log: log, awgExe: awgExe}
}

// Connect запускает AWG тунель.
// clientConf — содержимое .conf файла (из деплоя).
// vpsIP — IP VPS (нужен для anti-loop маршрута).
func (m *Manager) Connect(clientConf, vpsIP string) error {
	if m.proc != nil {
		return fmt.Errorf("already connected")
	}

	// Записываем конфиг
	confPath, err := filepath.Abs(awgConfFile)
	if err != nil {
		return err
	}
	clientConf = normalizeAllowedIPs(clientConf)
	clientConf = normalizeDNS(clientConf)
	if err := os.WriteFile(confPath, []byte(clientConf), 0600); err != nil {
		return fmt.Errorf("write conf: %w", err)
	}

	// Определяем физический шлюз ДО поднятия AWG маршрутов
	gw, err := routing.ParseGateway()
	if err != nil {
		return fmt.Errorf("gateway: %w", err)
	}
	m.log.Infof("[Connect] physIP=%s gw=%s", gw.LocalIP, gw.Gateway)

	// Маршрут к VPS через физический шлюз (anti-loop)
	// Удаляем перед добавлением — маршрут мог остаться с прошлой сессии
	delRoute(vpsIP + "/32")
	if err := addRoute(vpsIP+"/32", gw.Gateway); err != nil {
		m.log.Errorf("[Connect] add VPS route: %v", err)
	} else {
		m.log.Infof("[Connect] anti-loop route %s/32 via %s ✓", vpsIP, gw.Gateway)
	}

	// Запуск amneziawg.exe
	m.vpsIP = vpsIP
	cmd := exec.Command(m.awgExe, "/installtunnelservice", confPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x08000000,
		HideWindow:    true,
	}
	if err := cmd.Run(); err != nil {
		// Fallback: запуск без сервиса (для тестов)
		m.log.Debugf("[Connect] installtunnelservice failed, trying direct start: %v", err)
		m.proc = exec.Command(m.awgExe, confPath)
		m.proc.SysProcAttr = &syscall.SysProcAttr{
			CreationFlags: 0x08000000,
			HideWindow:    true,
		}
		if err2 := m.proc.Start(); err2 != nil {
			return fmt.Errorf("amneziawg start: %w", err2)
		}
	}

	// Ждём поднятия интерфейса
	time.Sleep(3 * time.Second)

	// Маршруты через AWG (весь трафик)
	if err := m.addAWGRoutes(); err != nil {
		m.log.Errorf("[Connect] routes: %v", err)
	}
	// Принудительно уводим системный DNS в AWG.
	// Иначе Windows может продолжать использовать DNS роутера (192.168.0.1),
	// что выглядит как DNS leak даже при корректном туннеле.
	if err := m.applyDNSPolicy("10.8.0.1"); err != nil {
		m.log.Errorf("[Connect] DNS policy: %v", err)
	}

	m.log.Info("[Connect] AWG connected ✓")
	return nil
}

// normalizeAllowedIPs принудительно выставляет split-default IPv4 маршруты,
// чтобы избежать kill-switch поведения при 0.0.0.0/0 в некоторых AWG/WFP сценариях.
func normalizeAllowedIPs(conf string) string {
	re := regexp.MustCompile(`(?mi)^AllowedIPs\s*=.*$`)
	const want = "AllowedIPs = 0.0.0.0/1, 128.0.0.0/1"
	if re.MatchString(conf) {
		return re.ReplaceAllString(conf, want)
	}
	return conf + "\n" + want + "\n"
}

// normalizeDNS принудительно выставляет DNS на сервер AWG внутри туннеля,
// чтобы исключить утечки обычного DNS на локальный роутер/провайдера.
func normalizeDNS(conf string) string {
	re := regexp.MustCompile(`(?mi)^DNS\s*=.*$`)
	const want = "DNS = 10.8.0.1"
	if re.MatchString(conf) {
		return re.ReplaceAllString(conf, want)
	}
	return conf + "\n" + want + "\n"
}

// Disconnect останавливает AWG и убирает маршруты.
func (m *Manager) Disconnect() {
	// Убираем маршруты
	delRoute(m.vpsIP + "/32")
	delRoute("0.0.0.0/1")
	delRoute("128.0.0.0/1")

	// Останавливаем AWG сервис
	awgExe := m.awgExe
	if awgExe == "" {
		awgExe = `awg\amneziawg.exe`
	}
	cmd := exec.Command(awgExe, "/uninstalltunnelservice", "furor")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x08000000,
		HideWindow:    true,
	}
	_ = cmd.Run()

	if m.proc != nil && m.proc.Process != nil {
		_ = m.proc.Process.Kill()
		m.proc = nil
	}
	if err := m.clearDNSPolicy(); err != nil {
		m.log.Infof("WARN: [Disconnect] clear DNS policy: %v", err)
	}

	_ = os.Remove(awgConfFile)
	m.log.Info("[Disconnect] AWG disconnected")
}

// IsConnected проверяет что AWG интерфейс существует.
func (m *Manager) IsConnected() bool {
	ifaces, _ := net.Interfaces()
	for _, i := range ifaces {
		if i.Name == "furor" {
			return true
		}
	}
	return false
}

func (m *Manager) addAWGRoutes() error {
	// Весь трафик через AWG интерфейс
	// Используем PowerShell — надёжнее во время инициализации WinTun
	script := `
try {
    $awg = Get-NetAdapter | Where-Object { $_.Name -eq "furor" } | Select-Object -First 1
    if ($awg) {
        New-NetRoute -InterfaceIndex $awg.InterfaceIndex -DestinationPrefix "0.0.0.0/1"   -NextHop "0.0.0.0" -RouteMetric 5 -ErrorAction SilentlyContinue
        New-NetRoute -InterfaceIndex $awg.InterfaceIndex -DestinationPrefix "128.0.0.0/1" -NextHop "0.0.0.0" -RouteMetric 5 -ErrorAction SilentlyContinue
        Write-Host "Routes added idx=$($awg.InterfaceIndex)"
    } else { Write-Host "ERROR: adapter furor not found"; exit 1 }
} catch { Write-Host "ERROR: $_"; exit 1 }
`
	out, err := runPS(script)
	m.log.Debugf("[Connect] routes: %s", out)
	return err
}

func addRoute(cidr, gw string) error {
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		return err
	}
	mask := fmt.Sprintf("%d.%d.%d.%d", n.Mask[0], n.Mask[1], n.Mask[2], n.Mask[3])
	_, err = runHidden("route", "add", n.IP.String(), "mask", mask, gw)
	return err
}

func delRoute(cidr string) {
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		return
	}
	mask := fmt.Sprintf("%d.%d.%d.%d", n.Mask[0], n.Mask[1], n.Mask[2], n.Mask[3])
	runHidden("route", "delete", n.IP.String(), "mask", mask) //nolint:errcheck
}

func runHidden(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x08000000,
		HideWindow:    true,
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func runPS(script string) (string, error) {
	wrapped := "$OutputEncoding = [Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)\n" + script
	out, err := exec.Command(
		"powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", wrapped,
	).CombinedOutput()
	return decodePSOutput(out), err
}

func decodePSOutput(out []byte) string {
	if len(out) == 0 {
		return ""
	}
	if utf8.Valid(out) {
		return strings.TrimSpace(string(out))
	}
	if s, err := charmap.CodePage866.NewDecoder().String(string(out)); err == nil {
		return strings.TrimSpace(s)
	}
	if s, err := charmap.Windows1251.NewDecoder().String(string(out)); err == nil {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(string(out))
}

func (m *Manager) applyDNSPolicy(dnsIP string) error {
	// 1) DNS сервер на AWG-интерфейсе.
	// 2) NRPT правило "." -> dnsIP, чтобы резолвинг не уходил на физический адаптер.
	// 3) Физические адаптеры -> 127.0.0.1 (жесткая защита от DNS leak).
	script := fmt.Sprintf(`
try {
    $awg = Get-NetAdapter | Where-Object { $_.Name -eq "furor" } | Select-Object -First 1
    if (-not $awg) { throw "adapter furor not found" }

    Set-DnsClientServerAddress -InterfaceIndex $awg.InterfaceIndex -ServerAddresses @("%s")

    $existing = Get-DnsClientNrptRule -ErrorAction SilentlyContinue | Where-Object {
        $_.Comment -eq "Furor AWG DNS force"
    }
    if ($existing) {
        $existing | Remove-DnsClientNrptRule -Force -ErrorAction SilentlyContinue
    }
    Add-DnsClientNrptRule -Namespace "." -NameServers "%s" -Comment "Furor AWG DNS force"

    # Hard anti-leak: physical adapters DNS -> localhost blackhole
    Get-NetAdapter | Where-Object {
        $_.Status -eq "Up" -and $_.Name -notlike "furor*"
    } | ForEach-Object {
        Set-DnsClientServerAddress -InterfaceAlias $_.Name -ServerAddresses @("127.0.0.1")
    }

    Clear-DnsClientCache
    Write-Host "DNS policy applied -> %s"
} catch {
    Write-Host "ERROR: $_"
    exit 1
}
`, dnsIP, dnsIP, dnsIP)

	out, err := runPS(script)
	m.log.Debugf("[Connect] DNS policy: %s", out)
	return err
}

func (m *Manager) clearDNSPolicy() error {
	script := `
try {
    $rules = Get-DnsClientNrptRule -ErrorAction SilentlyContinue | Where-Object {
        $_.Comment -eq "Furor AWG DNS force"
    }
    if ($rules) {
        $rules | Remove-DnsClientNrptRule -Force -ErrorAction SilentlyContinue
    }

    # Restore DNS on physical adapters
    Get-NetAdapter | Where-Object {
        $_.Name -notlike "furor*"
    } | ForEach-Object {
        Set-DnsClientServerAddress -InterfaceAlias $_.Name -ResetServerAddresses
    }

    $awg = Get-NetAdapter | Where-Object { $_.Name -eq "furor" } | Select-Object -First 1
    if ($awg) {
        Set-DnsClientServerAddress -InterfaceIndex $awg.InterfaceIndex -ResetServerAddresses
    }

    Clear-DnsClientCache
    Write-Host "DNS policy cleared"
} catch {
    Write-Host "ERROR: $_"
    exit 1
}
`
	out, err := runPS(script)
	m.log.Debugf("[Disconnect] DNS policy: %s", out)
	return err
}
