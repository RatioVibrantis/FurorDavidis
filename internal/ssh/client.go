// internal/ssh/client.go
// SSH клиент — адаптировано из Vanus Scrutator (проверено в бою).
// Password + KeyboardInteractive (OpenSSH 8+, Ubuntu 24.04).
// Keepalive 30s — AWG DKMS сборка занимает до 5 мин тишины.
package ssh

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	gossh "golang.org/x/crypto/ssh"
)

// Client — SSH соединение со стримингом вывода.
type Client struct {
	host    string
	port    string
	user    string
	pass    string
	conn    *gossh.Client
	LogLine chan string // канал для UI стриминга
}

// Dial создаёт клиент и сразу подключается.
func Dial(host, port, user, pass string) (*Client, error) {
	c := &Client{
		host:    host,
		port:    port,
		user:    user,
		pass:    pass,
		LogLine: make(chan string, 512),
	}
	return c, c.connect()
}

func (c *Client) connect() error {
	if c.port == "" {
		c.port = "22"
	}
	addr := net.JoinHostPort(c.host, c.port)
	pass := c.pass

	kbi := gossh.KeyboardInteractive(func(
		_, _ string, questions []string, _ []bool,
	) ([]string, error) {
		ans := make([]string, len(questions))
		for i := range questions {
			ans[i] = pass
		}
		return ans, nil
	})

	cfg := &gossh.ClientConfig{
		User:            c.user,
		Auth:            []gossh.AuthMethod{gossh.Password(pass), kbi},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), //nolint:gosec
		Timeout:         30 * time.Second,
	}

	var err error
	c.conn, err = gossh.Dial("tcp", addr, cfg)
	if err != nil {
		return fmt.Errorf("ssh dial %s: %w", addr, err)
	}

	// Keepalive 30s — не даём NAT/firewall рвать соединение
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for range t.C {
			if c.conn == nil {
				return
			}
			if _, _, err := c.conn.SendRequest("keepalive@openssh.com", true, nil); err != nil {
				return
			}
		}
	}()
	return nil
}

// Close закрывает соединение и канал.
func (c *Client) Close() {
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
	if c.LogLine != nil {
		close(c.LogLine)
		c.LogLine = nil
	}
}

// RunScript выполняет bash скрипт через stdin, стримит вывод построчно в logFn и LogLine.
func (c *Client) RunScript(script string, logFn func(string)) (string, error) {
	if c.conn == nil {
		return "", fmt.Errorf("не подключен")
	}
	sess, err := c.conn.NewSession()
	if err != nil {
		return "", fmt.Errorf("new session: %w", err)
	}
	defer sess.Close()

	outPR, outPW := io.Pipe()
	errPR, errPW := io.Pipe()
	sess.Stdout = outPW
	sess.Stderr = errPW

	stdin, err := sess.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("stdin pipe: %w", err)
	}

	if err := sess.Start("bash -s"); err != nil {
		return "", fmt.Errorf("start bash: %w", err)
	}

	go func() {
		io.WriteString(stdin, script) //nolint:errcheck
		stdin.Close()
	}()

	var mu sync.Mutex
	var sb strings.Builder

	drain := func(r io.Reader) {
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			line := sc.Text()
			mu.Lock()
			sb.WriteString(line + "\n")
			mu.Unlock()
			if logFn != nil {
				logFn(line)
			}
			c.emit(line)
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); drain(outPR) }()
	go func() { defer wg.Done(); drain(errPR) }()

	waitErr := sess.Wait()
	outPW.Close()
	errPW.Close()
	wg.Wait()

	return sb.String(), waitErr
}

// Run выполняет одну команду, возвращает вывод.
func (c *Client) Run(cmd string) (string, error) {
	if c.conn == nil {
		return "", fmt.Errorf("не подключен")
	}
	sess, err := c.conn.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()
	var sb strings.Builder
	sess.Stdout = &sb
	sess.Stderr = &sb
	err = sess.Run(cmd)
	return sb.String(), err
}

func (c *Client) emit(line string) {
	if c.LogLine == nil {
		return
	}
	defer func() { recover() }() //nolint:errcheck
	select {
	case c.LogLine <- line:
	default:
	}
}
