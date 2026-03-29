// internal/routing/windows.go
// Определение физического шлюза и IP адаптера.
// Адаптировано из Vanus Scrutator — проверено в бою.
package routing

import (
	"fmt"
	"math"
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

// GatewayInfo — информация об активном физическом шлюзе.
type GatewayInfo struct {
	Gateway   string // IP шлюза, напр. 192.168.1.1
	Interface string // имя интерфейса Windows
	LocalIP   string // локальный IP машины (physIP для bind)
	Metric    int    // метрика маршрута (меньше = приоритетнее)
}

// ParseGateway определяет активный физический шлюз через route print.
// При нескольких адаптерах (Wi-Fi + Ethernet) выбирает с наименьшей метрикой.
// Вызывать динамически — DHCP и смена сети меняют LocalIP.
func ParseGateway() (*GatewayInfo, error) {
	return ParseGatewayExclude("")
}

// ParseGatewayExclude — как ParseGateway, но пропускает указанный интерфейс (напр. AWG iface).
// skipIface == "" → не пропускать ничего.
func ParseGatewayExclude(skipIface string) (*GatewayInfo, error) {
	out, err := runCmd("route", "print", "0.0.0.0")
	if err != nil {
		return nil, fmt.Errorf("route print: %w", err)
	}

	// Строка вида: "   0.0.0.0   0.0.0.0   192.168.1.1   192.168.1.100   25"
	// Последняя колонка — метрика (может отсутствовать в некоторых версиях Windows).
	re := regexp.MustCompile(`\s+0\.0\.0\.0\s+0\.0\.0\.0\s+(\d+\.\d+\.\d+\.\d+)\s+(\d+\.\d+\.\d+\.\d+)(?:\s+(\d+))?`)

	best := &GatewayInfo{Metric: math.MaxInt32}
	found := false

	for _, line := range strings.Split(out, "\n") {
		m := re.FindStringSubmatch(line)
		if len(m) < 3 {
			continue
		}
		gw := strings.TrimSpace(m[1])
		localIP := strings.TrimSpace(m[2])

		metric := 9999
		if len(m) >= 4 && m[3] != "" {
			if v, err2 := strconv.Atoi(strings.TrimSpace(m[3])); err2 == nil {
				metric = v
			}
		}

		ifName, _ := ifaceByIP(localIP)

		// Пропускаем AWG / виртуальный интерфейс если указан
		if skipIface != "" && strings.EqualFold(ifName, skipIface) {
			continue
		}

		if metric < best.Metric {
			best = &GatewayInfo{
				Gateway:   gw,
				Interface: ifName,
				LocalIP:   localIP,
				Metric:    metric,
			}
			found = true
		}
	}

	if !found {
		return nil, fmt.Errorf("шлюз не найден в таблице маршрутизации")
	}
	return best, nil
}

// ifaceByIP находит имя интерфейса по его IP.
func ifaceByIP(ip string) (string, error) {
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			var s string
			switch v := addr.(type) {
			case *net.IPNet:
				s = v.IP.String()
			case *net.IPAddr:
				s = v.IP.String()
			}
			if s == ip {
				return iface.Name, nil
			}
		}
	}
	return "unknown", nil
}

func runCmd(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x08000000,
		HideWindow:    true,
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}
