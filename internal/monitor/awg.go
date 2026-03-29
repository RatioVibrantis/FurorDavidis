// internal/monitor/awg.go
// Сбор метрик AWG интерфейса: RTT, байты/сек, статус.
package monitor

import (
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Metrics — текущие метрики тунеля.
type Metrics struct {
	RTTms       int
	RTTTrend    string // "rising" / "falling" / "stable"
	BytesPerSec float64
	Connected   bool
	IfaceName   string
}

// Monitor собирает метрики AWG интерфейса.
type Monitor struct {
	ifaceName  string
	rttHistory []int
	lastBytes  uint64
	lastTime   time.Time
}

func NewMonitor(ifaceName string) *Monitor {
	return &Monitor{
		ifaceName: ifaceName,
		lastTime:  time.Now(),
	}
}

// Collect собирает текущие метрики.
func (m *Monitor) Collect(vpsIP string) Metrics {
	metrics := Metrics{IfaceName: m.ifaceName}

	// Connected = адаптер AWG существует (ping может блокироваться VPS firewall)
	metrics.Connected = adapterExists(m.ifaceName)

	// RTT через ping к VPS (для тренда; 0 если ICMP заблокирован)
	rtt := pingRTT(vpsIP)
	metrics.RTTms = rtt

	// RTT trend — сравниваем с историей (последние 5 измерений)
	m.rttHistory = append(m.rttHistory, rtt)
	if len(m.rttHistory) > 5 {
		m.rttHistory = m.rttHistory[len(m.rttHistory)-5:]
	}
	metrics.RTTTrend = calcTrend(m.rttHistory)

	return metrics
}

// MeasureRTTMedian делает несколько ping-замеров RTT и возвращает медиану.
// 0 означает, что валидных замеров не удалось получить.
func MeasureRTTMedian(host string, samples int, pause time.Duration) int {
	if samples <= 0 {
		samples = 1
	}
	if pause < 0 {
		pause = 0
	}
	vals := make([]int, 0, samples)
	for i := 0; i < samples; i++ {
		rtt := pingRTT(host)
		if rtt > 0 {
			vals = append(vals, rtt)
		}
		if i < samples-1 && pause > 0 {
			time.Sleep(pause)
		}
	}
	if len(vals) == 0 {
		return 0
	}
	sort.Ints(vals)
	mid := len(vals) / 2
	if len(vals)%2 == 1 {
		return vals[mid]
	}
	return (vals[mid-1] + vals[mid]) / 2
}

// RTTTrend возвращает последний известный тренд.
func (m *Monitor) RTTTrend() string {
	return calcTrend(m.rttHistory)
}

// calcTrend считает тренд RTT по последним N значениям.
func calcTrend(history []int) string {
	if len(history) < 3 {
		return "stable"
	}
	first := avg(history[:len(history)/2])
	last := avg(history[len(history)/2:])
	delta := float64(last-first) / float64(first+1)
	switch {
	case delta > 0.15:
		return "rising"
	case delta < -0.15:
		return "falling"
	default:
		return "stable"
	}
}

func avg(vals []int) int {
	if len(vals) == 0 {
		return 0
	}
	sum := 0
	for _, v := range vals {
		sum += v
	}
	return sum / len(vals)
}

// pingRTT пингует хост и возвращает RTT в мс. 0 = недоступен.
func pingRTT(host string) int {
	if host == "" {
		return 0
	}
	out, err := runHidden("ping", "-n", "1", "-w", "2000", host)
	if err != nil {
		return 0
	}
	// Парсим "Average = 45ms" или "time=45ms"
	re := regexp.MustCompile(`(?:Average\s*=\s*|time[<=])(\d+)ms`)
	m := re.FindStringSubmatch(out)
	if len(m) == 2 {
		v, _ := strconv.Atoi(m[1])
		return v
	}
	// Fallback: "Среднее = 45 мс"
	re2 := regexp.MustCompile(`(?:Среднее|Average)\s*[=:]\s*(\d+)`)
	m2 := re2.FindStringSubmatch(out)
	if len(m2) == 2 {
		v, _ := strconv.Atoi(m2[1])
		return v
	}
	if strings.Contains(out, "TTL=") || strings.Contains(out, "ttl=") {
		return 1 // доступен, RTT не распарсили
	}
	return 0
}

// adapterExists проверяет что сетевой адаптер с данным именем существует.
func adapterExists(name string) bool {
	ifaces, err := net.Interfaces()
	if err != nil {
		return false
	}
	for _, i := range ifaces {
		if i.Name == name {
			return true
		}
	}
	return false
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

// FormatMetrics форматирует метрики для лога.
func FormatMetrics(m Metrics) string {
	if !m.Connected {
		return fmt.Sprintf("[Monitor] AWG: нет соединения")
	}
	return fmt.Sprintf("[Monitor] RTT=%dms trend=%s", m.RTTms, m.RTTTrend)
}
