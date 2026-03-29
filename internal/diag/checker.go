// internal/diag/checker.go
// Диагностика локальных файлов и состояния сервера.
package diag

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/yourorg/furor-davidis/internal/ai"
)

// Item — один пункт диагностики.
type Item struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// Report — полный отчёт диагностики.
type Report struct {
	Local  []Item `json:"local"`
	Server []Item `json:"server"`
}

// CheckLocal проверяет AWG файлы и состояние LM Studio.
func CheckLocal(preferredModel string) []Item {
	if preferredModel == "" {
		preferredModel = ai.DefaultModel
	}
	return []Item{
		checkFile(`awg\amneziawg.exe`, "AWG бинарь"),
		checkFile(`awg\wintun.dll`, "AWG WinTun драйвер"),
		checkLMStudio(preferredModel),
	}
}

// CheckServer проверяет доступность сервера.
func CheckServer(vpsHost string, awgPort string) []Item {
	return []Item{
		checkTCP(vpsHost, "443", "xray декой TCP:443"),
		checkUDPPort(vpsHost, awgPort),
		checkPing(vpsHost),
	}
}

func checkFile(path, name string) Item {
	if _, err := os.Stat(path); err == nil {
		return Item{Name: name, OK: true, Detail: path}
	}
	return Item{Name: name, OK: false, Detail: "не найден: " + path}
}

// checkLMStudio проверяет что LM Studio сервер запущен и модель загружена.
func checkLMStudio(preferredModel string) Item {
	probe := &http.Client{Timeout: 3 * time.Second}
	resp, err := probe.Get("http://localhost:1234/v1/models")
	if err != nil {
		return Item{
			Name:   "LM Studio",
			OK:     false,
			Detail: "сервер не запущен — открой LM Studio → Local Server → Start Server",
		}
	}
	defer resp.Body.Close()

	var modelsResp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil || len(modelsResp.Data) == 0 {
		return Item{
			Name:   "LM Studio",
			OK:     false,
			Detail: fmt.Sprintf("сервер запущен, модель не загружена — нажми «Запустить» (подгрузится %s)", preferredModel),
		}
	}

	return Item{
		Name:   "LM Studio",
		OK:     true,
		Detail: fmt.Sprintf("запущен · модель: %s", modelsResp.Data[0].ID),
	}
}

func checkTCP(host, port, name string) Item {
	addr := net.JoinHostPort(host, port)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return Item{Name: name, OK: false, Detail: fmt.Sprintf("%s недоступен", addr)}
	}
	conn.Close()
	return Item{Name: name, OK: true, Detail: addr + " → доступен"}
}

func checkUDPPort(host, port string) Item {
	name := "AWG UDP:" + port
	if port == "" {
		return Item{Name: "AWG UDP", OK: false, Detail: "порт не задан (деплой не выполнен?)"}
	}
	return Item{Name: name, OK: true, Detail: "порт задан (UDP проверка — только ping)"}
}

func checkPing(host string) Item {
	if host == "" {
		return Item{Name: "Ping VPS", OK: false, Detail: "VPS не задан"}
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, "22"), 5*time.Second)
	if err != nil {
		conn2, err2 := net.DialTimeout("tcp", net.JoinHostPort(host, "443"), 5*time.Second)
		if err2 != nil {
			return Item{Name: "VPS доступность", OK: false, Detail: host + " — недоступен"}
		}
		conn2.Close()
	} else {
		conn.Close()
	}
	return Item{Name: "VPS доступность", OK: true, Detail: host + " → доступен"}
}
