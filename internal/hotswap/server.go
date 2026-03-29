// internal/hotswap/server.go
// HotSwap декой-домена на VPS через SSH.
// Механизм: sed DecoyDomain в xray конфиге → docker kill --signal HUP xray
// HUP = xray перечитывает конфиг за ~100ms без разрыва сессий.
// После HotSwap: проверяем TCP:443 на VPS — если декой не поднялся, пишем WARN.
package hotswap

import (
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"time"

	"github.com/yourorg/furor-davidis/internal/logger"
	"github.com/yourorg/furor-davidis/internal/payload"
	"github.com/yourorg/furor-davidis/internal/ssh"
)

// Swapper меняет декой-домен xray на VPS.
type Swapper struct {
	log           *logger.Logger
	currentDomain string
	domainIdx     int
}

func NewSwapper(log *logger.Logger) *Swapper {
	return &Swapper{log: log}
}

// Swap меняет декой-домен на следующий из списка.
// SSH → sed xray конфиг → docker kill HUP → verify TCP:443.
func (s *Swapper) Swap(vpsHost string, vpsPort int, vpsUser, vpsPass string, domains []string) error {
	if len(domains) == 0 {
		return fmt.Errorf("список доменов пуст")
	}

	// Следующий домен (round-robin, избегаем повторения текущего)
	next := s.nextDomain(domains)

	s.log.Infof("[HotSwap] %s → %s", s.currentDomain, next)

	portStr := strconv.Itoa(vpsPort)
	if portStr == "0" {
		portStr = "22"
	}

	cl, err := ssh.Dial(vpsHost, portStr, vpsUser, vpsPass)
	if err != nil {
		return fmt.Errorf("ssh: %w", err)
	}
	defer cl.Close()

	script := payload.HotSwapScript(next)
	out, err := cl.RunScript(script, func(line string) {
		s.log.Debugf("[HotSwap SSH] %s", line)
	})
	if err != nil {
		return fmt.Errorf("hotswap script: %w (out: %s)", err, out)
	}

	s.currentDomain = next

	// Пост-проверка: TCP:443 должен отвечать после HUP (~100-300ms для xray reload)
	if verifyErr := verifyDecoy(vpsHost, 3*time.Second); verifyErr != nil {
		// Не фатально — AWG продолжает работать, но предупреждаем UI
		s.log.Infof("[HotSwap] WARN декой %s не отвечает на TCP:443: %v", next, verifyErr)
	} else {
		s.log.Infof("[HotSwap] декой %s OK (TCP:443 отвечает)", next)
	}

	return nil
}

// SwapTo меняет на конкретный домен (вызов из UI).
func (s *Swapper) SwapTo(domain, vpsHost string, vpsPort int, vpsUser, vpsPass string) error {
	if domain == "" {
		return fmt.Errorf("домен не указан")
	}

	s.log.Infof("[HotSwap] явная смена → %s", domain)

	portStr := strconv.Itoa(vpsPort)
	if portStr == "0" {
		portStr = "22"
	}

	cl, err := ssh.Dial(vpsHost, portStr, vpsUser, vpsPass)
	if err != nil {
		return fmt.Errorf("ssh: %w", err)
	}
	defer cl.Close()

	script := payload.HotSwapScript(domain)
	out, err := cl.RunScript(script, func(line string) {
		s.log.Debugf("[HotSwap SSH] %s", line)
	})
	if err != nil {
		return fmt.Errorf("hotswap script: %w (out: %s)", err, out)
	}

	s.currentDomain = domain

	if verifyErr := verifyDecoy(vpsHost, 3*time.Second); verifyErr != nil {
		s.log.Infof("[HotSwap] WARN декой %s не отвечает на TCP:443: %v", domain, verifyErr)
	} else {
		s.log.Infof("[HotSwap] декой %s OK (TCP:443 отвечает)", domain)
	}

	return nil
}

// CurrentDomain возвращает текущий активный декой-домен.
func (s *Swapper) CurrentDomain() string {
	return s.currentDomain
}

// SetCurrentDomain устанавливает начальный домен (после деплоя).
func (s *Swapper) SetCurrentDomain(d string) {
	s.currentDomain = d
}

// nextDomain выбирает следующий домен из списка (round-robin с рандомизацией).
func (s *Swapper) nextDomain(domains []string) string {
	if len(domains) == 1 {
		return domains[0]
	}
	// Случайный шаг 1..n-1 чтобы не было предсказуемого паттерна
	step := 1 + rand.Intn(len(domains)-1)
	s.domainIdx = (s.domainIdx + step) % len(domains)
	next := domains[s.domainIdx]
	// Если совпало с текущим — берём следующий
	if next == s.currentDomain {
		s.domainIdx = (s.domainIdx + 1) % len(domains)
		next = domains[s.domainIdx]
	}
	return next
}

// verifyDecoy проверяет доступность TCP:443 на VPS после HotSwap.
// Короткий таймаут — xray reload ~100ms, даём 3 сек на старт.
func verifyDecoy(host string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", host+":443", 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("TCP:443 timeout %s", timeout)
}
