// internal/cover/executor.go
// Cover traffic executor.
// Р”РµР»Р°РµС‚ СЂРµР°Р»СЊРЅС‹Рµ HTTPS Р·Р°РїСЂРѕСЃС‹ Рє Р»РµРіРёС‚РёРјРЅС‹Рј СЃР°Р№С‚Р°Рј РЎРќРђР РЈР–Р AWG С‚СѓРЅРµР»СЏ.
// РњРµС…Р°РЅРёР·Рј bypass AWG: route add <targetIP>/32 <physGW> вЂ” Р±РѕР»РµРµ СЃРїРµС†РёС„РёС‡РЅС‹Р№ РјР°СЂС€СЂСѓС‚
// С‡РµРј AWG 0.0.0.0/1, РїРѕСЌС‚РѕРјСѓ СЏРґСЂРѕ СЂРѕСѓС‚РёС‚ TCP С‡РµСЂРµР· С„РёР·Р°РґР°РїС‚РµСЂ Р±РµР· bind(physIP).
// WFP Р±Р»РѕРєРёСЂСѓРµС‚ bind(physIP) РЅРѕ РќР• Р±Р»РѕРєРёСЂСѓРµС‚ connect Р±РµР· bind в†’ /32 route = СЂРµС€РµРЅРёРµ.
// TLS fingerprint = Chrome (utls) вЂ” Go crypto/tls РІС‹РіР»СЏРґРёС‚ РєР°Рє Р±РѕС‚.
package cover

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os/exec"
	"strings"
	"syscall"
	"time"

	utls "github.com/refraction-networking/utls"
	"github.com/yourorg/furor-davidis/internal/ai"
	"github.com/yourorg/furor-davidis/internal/logger"
	"github.com/yourorg/furor-davidis/internal/routing"
	"golang.org/x/net/http2"
)

// Executor вЂ” РёСЃРїРѕР»РЅРёС‚РµР»СЊ cover traffic.
type Executor struct {
	log      *logger.Logger
	awgIface string
}

type coverTransport struct {
	h1  http.RoundTripper
	h2  http.RoundTripper
	log *logger.Logger
}

func (t *coverTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL != nil && strings.EqualFold(req.URL.Scheme, "https") && t.h2 != nil {
		resp, err := t.h2.RoundTrip(req)
		if err == nil {
			return resp, nil
		}
		// Fallback for origins that negotiate http/1.1 or reject h2 for this fingerprint/path.
		if strings.Contains(err.Error(), "unexpected ALPN protocol") ||
			strings.Contains(err.Error(), "server does not support HTTP/2") {
			if t.log != nil {
				t.log.Debugf("[Cover] h2 fallback to h1 for %s: %v", req.URL.Host, err)
			}
			return t.h1.RoundTrip(req)
		}
		return nil, err
	}
	return t.h1.RoundTrip(req)
}

func NewExecutor(log *logger.Logger) *Executor {
	return &Executor{log: log}
}

// SetAWGInterface Р·Р°РґР°РµС‚ РёРјСЏ AWG РёРЅС‚РµСЂС„РµР№СЃР°, РєРѕС‚РѕСЂС‹Р№ РЅСѓР¶РЅРѕ РёСЃРєР»СЋС‡Р°С‚СЊ
// РїСЂРё РїРѕРёСЃРєРµ С„РёР·РёС‡РµСЃРєРѕРіРѕ default gateway.
func (e *Executor) SetAWGInterface(iface string) {
	e.awgIface = strings.TrimSpace(iface)
}

// Run РІС‹РїРѕР»РЅСЏРµС‚ РїРѕСЃР»РµРґРѕРІР°С‚РµР»СЊРЅРѕСЃС‚СЊ URL РёР· AI.
// РћРґРёРЅ CookieJar РЅР° РІСЃСЋ СЃРµСЃСЃРёСЋ вЂ” РєСѓРєРё Рё Referer СЃРѕР·РґР°СЋС‚ СЂРµР°Р»РёСЃС‚РёС‡РЅС‹Р№ РїР°С‚С‚РµСЂРЅ.
func (e *Executor) Run(ctx context.Context, items []ai.CoverItem) {
	jar, _ := cookiejar.New(nil)
	var prevURL string

	for i := 0; i < len(items); {
		host := extractHost(items[i].URL)
		if host == "" {
			host = extractHost(normalizeURL(items[i].URL))
		}
		j := i + 1
		for j < len(items) && extractHost(items[j].URL) == host {
			j++
		}
		prevURL = e.runHostSession(ctx, host, items[i:j], jar, prevURL)
		i = j
	}
}

func (e *Executor) runHostSession(
	ctx context.Context,
	host string,
	items []ai.CoverItem,
	jar http.CookieJar,
	prevURL string,
) string {
	var physGW string
	if gw, err := routing.ParseGatewayExclude(e.awgIface); err == nil {
		physGW = gw.Gateway
		e.log.Debugf("[Cover] gateway: iface=%s local=%s gw=%s", gw.Interface, gw.LocalIP, gw.Gateway)
	} else {
		e.log.Infof("[Cover] WARN: phys gateway not found (skipIface=%q): %v", e.awgIface, err)
	}

	client := e.buildClient(physGW, host, jar)

	for _, item := range items {
		select {
		case <-ctx.Done():
			return prevURL
		default:
		}

		ref := strings.TrimSpace(item.Referer)
		if ref == "" {
			ref = prevURL
		}

		targetURL := normalizeURL(item.URL)
		req, err := e.buildRequest(ctx, targetURL, ref)
		if err != nil {
			e.log.Debugf("[Cover] %s -> error: %v", item.URL, err)
		} else if err := e.doRequest(client, req); err != nil {
			e.log.Debugf("[Cover] %s -> error: %v", item.URL, err)
		} else {
			e.log.Debugf("[Cover] %s -> OK", item.URL)
			prevURL = targetURL
		}

		jitter := 1.0 + (rand.Float64()*0.4 - 0.2)
		pause := time.Duration(float64(item.ReadSec)*jitter) * time.Second
		select {
		case <-ctx.Done():
			return prevURL
		case <-time.After(pause):
		}
	}
	return prevURL
}

// fetch РґРµР»Р°РµС‚ СЂРµР°Р»СЊРЅС‹Р№ HTTPS GET СЃ Chrome TLS fingerprint СЃРЅР°СЂСѓР¶Рё AWG С‚СѓРЅРµР»СЏ.
// РџРµСЂРµРґ Р·Р°РїСЂРѕСЃРѕРј: СЂРµР·РѕР»РІРёРј hostname в†’ РґРѕР±Р°РІР»СЏРµРј /32 РјР°СЂС€СЂСѓС‚ С‡РµСЂРµР· physGW.
// /32 РјР°СЂС€СЂСѓС‚ РїСЂРёРѕСЂРёС‚РµС‚РЅРµРµ AWG 0.0.0.0/1 в†’ TCP РёРґС‘С‚ С‡РµСЂРµР· С„РёР·Р°РґР°РїС‚РµСЂ Р±РµР· bind.
func (e *Executor) fetch(ctx context.Context, rawURL string, jar http.CookieJar, referer string) error {
	url := normalizeURL(rawURL)
	host := extractHost(url)

	// РЁР»СЋР· С„РёР·Р°РґР°РїС‚РµСЂР° вЂ” РЅСѓР¶РµРЅ РґР»СЏ РґРѕР±Р°РІР»РµРЅРёСЏ /32 РјР°СЂС€СЂСѓС‚РѕРІ
	var physGW string
	if gw, err := routing.ParseGatewayExclude(e.awgIface); err == nil {
		physGW = gw.Gateway
		e.log.Debugf("[Cover] gateway: iface=%s local=%s gw=%s", gw.Interface, gw.LocalIP, gw.Gateway)
	} else {
		e.log.Infof("[Cover] WARN: С„РёР·РёС‡РµСЃРєРёР№ gateway РЅРµ РЅР°Р№РґРµРЅ (skipIface=%q): %v", e.awgIface, err)
	}

	client := e.buildClient(physGW, host, jar)

	req, err := e.buildRequest(ctx, url, referer)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("do request (utls only): %w", err)
	}
	defer resp.Body.Close()

	if _, err := io.Copy(io.Discard, io.LimitReader(resp.Body, 512*1024)); err != nil {
		return fmt.Errorf("read body: %w", err)
	}
	return nil
}

func (e *Executor) doRequest(client *http.Client, req *http.Request) error {
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("do request (utls only): %w", err)
	}
	defer resp.Body.Close()

	if _, err := io.Copy(io.Discard, io.LimitReader(resp.Body, 512*1024)); err != nil {
		return fmt.Errorf("read body: %w", err)
	}
	return nil
}

var utlsProfiles = []utls.ClientHelloID{
	utls.HelloChrome_102,
	utls.HelloFirefox_105,
	utls.HelloChrome_120,
	utls.HelloFirefox_120,
}

// buildClient СЃС‚СЂРѕРёС‚ http.Client СЃ Chrome fingerprint Рё route-bypass Р»РѕРіРёРєРѕР№.
// physGW вЂ” С€Р»СЋР· С„РёР·Р°РґР°РїС‚РµСЂР° (РїСѓСЃС‚Р°СЏ СЃС‚СЂРѕРєР° = Р±РµР· bypass, С‚СЂР°С„РёРє С‡РµСЂРµР· AWG).
// Р’ DialTLSContext: СЂРµР·РѕР»РІРёРј IP в†’ route add /32 в†’ dial Рє РєРѕРЅРєСЂРµС‚РЅРѕРјСѓ IP.
func (e *Executor) buildClient(physGW, sniHint string, jar http.CookieJar) *http.Client {
	dialer := &net.Dialer{Timeout: 15 * time.Second}

	// dialWithRoute: СЂРµР·РѕР»РІРёС‚ hostname в†’ РґРѕР±Р°РІР»СЏРµС‚ /32 РјР°СЂС€СЂСѓС‚ в†’ РґРёР°Р»РёС‚ Рє РєРѕРЅРєСЂРµС‚РЅРѕРјСѓ IP.
	// РСЃРїРѕР»СЊР·СѓРµС‚СЃСЏ Рё РґР»СЏ plain DialContext Рё РІРЅСѓС‚СЂРё DialTLSContext.
	dialWithRoute := func(tCtx context.Context, network, addr string) (net.Conn, error) {
		host, port, _ := net.SplitHostPort(addr)
		ips := e.lookupHost(tCtx, host, physGW)
		if len(ips) == 0 {
			return dialer.DialContext(tCtx, network, addr)
		}
		// Р”РѕР±Р°РІР»СЏРµРј /32 РјР°СЂС€СЂСѓС‚С‹ РґР»СЏ РІСЃРµС… IP С…РѕСЃС‚Р° С‡РµСЂРµР· С„РёР·Р°РґР°РїС‚РµСЂ
		for _, ip := range ips {
			addCoverRoute(ip, physGW, e.log)
		}
		// Р”Р°С‘Рј СЏРґСЂСѓ РїСЂРёРјРµРЅРёС‚СЊ /32 РјР°СЂС€СЂСѓС‚ РїРµСЂРµРґ РїРѕРїС‹С‚РєРѕР№ dial.
		time.Sleep(500 * time.Millisecond)
		// Р”РёР°Р»РёРј Рє РїРµСЂРІРѕРјСѓ IPv4
		ip := pickIPv4(ips)
		if ip == "" {
			ip = ips[0]
		}
		return dialer.DialContext(tCtx, network, net.JoinHostPort(ip, port))
	}

	// DialTLSContext: uTLS fingerprint rotation + route-bypass
	dialTLSWithUTLS := func(tCtx context.Context, network, addr string) (net.Conn, error) {
			host, port, _ := net.SplitHostPort(addr)
			ips := e.lookupHost(tCtx, host, physGW)
			if len(ips) == 0 || host == "" || port == "" {
				return nil, fmt.Errorf("doh resolve failed for %s", host)
			}
			for _, ip := range ips {
				addCoverRoute(ip, physGW, e.log)
			}
			// Р”Р°С‘Рј СЏРґСЂСѓ РїСЂРёРјРµРЅРёС‚СЊ /32 РјР°СЂС€СЂСѓС‚ РїРµСЂРµРґ РїРѕРїС‹С‚РєРѕР№ dial/handshake.
			time.Sleep(500 * time.Millisecond)
			ip := pickIPv4(ips)
			if ip == "" {
				ip = ips[0]
			}
			sni := host
			if sniHint != "" {
				sni = sniHint
			}

			target := net.JoinHostPort(ip, port)
			var lastErr error
			for _, profile := range utlsProfiles {
				rawConn, err := dialer.DialContext(tCtx, network, target)
				if err != nil {
					lastErr = err
					continue
				}
				tlsConn := utls.UClient(rawConn, &utls.Config{
					ServerName: sni,
					MinVersion: tls.VersionTLS12,
					MaxVersion: tls.VersionTLS13,
					// Browser-consistent ALPN: РјРЅРѕРіРёРµ СЃРѕРІСЂРµРјРµРЅРЅС‹Рµ edge/CDN РѕР¶РёРґР°СЋС‚ h2.
					NextProtos: []string{"h2", "http/1.1"},
				}, profile)
				if err := tlsConn.Handshake(); err != nil {
					_ = rawConn.Close()
					lastErr = err
					e.log.Debugf("[Cover] uTLS profile failed for host=%s sni=%s profile=%v: %v", host, sni, profile, err)
					time.Sleep(500 * time.Millisecond)
					continue
				}
				return tlsConn, nil
			}
			if lastErr == nil {
				lastErr = fmt.Errorf("all uTLS profiles failed")
			}
			return nil, lastErr
	}

	h1Transport := &http.Transport{
		DialTLSContext:       dialTLSWithUTLS,
		DialContext:           dialWithRoute,
		ForceAttemptHTTP2:     false,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		DisableKeepAlives:     false,
		TLSNextProto:          map[string]func(string, *tls.Conn) http.RoundTripper{},
	}

	h2Transport := &http2.Transport{
		DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
			return dialTLSWithUTLS(ctx, network, addr)
		},
	}

	return &http.Client{
		Transport: &coverTransport{
			h1:  h1Transport,
			h2:  h2Transport,
			log: e.log,
		},
		Jar:       jar,
		Timeout:   30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > 5 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
}

// lookupHost СЂРµР·РѕР»РІРёС‚ host С‚РѕР»СЊРєРѕ С‡РµСЂРµР· DoH (Р±РµР· СЃРёСЃС‚РµРјРЅРѕРіРѕ resolver),
// С‡С‚РѕР±С‹ РёСЃРєР»СЋС‡РёС‚СЊ plain DNS leak РЅР° Р»РѕРєР°Р»СЊРЅС‹Р№ СЂРѕСѓС‚РµСЂ/РїСЂРѕРІР°Р№РґРµСЂР°.
func (e *Executor) lookupHost(ctx context.Context, host, physGW string) []string {
	ips, err := e.lookupHostDoH(ctx, host, physGW)
	if err == nil && len(ips) > 0 {
		return ips
	}
	if err != nil {
		e.log.Debugf("[Cover] DoH resolve failed for %s: %v", host, err)
	}
	return nil
}

type dohEndpoint struct {
	name       string
	ip         string
	serverName string
	urlTmpl    string
}

var dohResolvers = []dohEndpoint{
	{
		name:       "cloudflare",
		ip:         "1.1.1.1",
		serverName: "cloudflare-dns.com",
		urlTmpl:    "https://cloudflare-dns.com/dns-query?name=%s&type=A",
	},
	{
		name:       "google",
		ip:         "8.8.8.8",
		serverName: "dns.google",
		urlTmpl:    "https://dns.google/resolve?name=%s&type=A",
	},
	{
		name:       "quad9",
		ip:         "9.9.9.9",
		serverName: "dns.quad9.net",
		urlTmpl:    "https://dns.quad9.net/dns-query?name=%s&type=A",
	},
}

// lookupHostDoH СЂРµР·РѕР»РІРёС‚ A-Р·Р°РїРёСЃРё С‡РµСЂРµР· РЅР°Р±РѕСЂ DoH РїСЂРѕРІР°Р№РґРµСЂРѕРІ (HTTPS),
// С‡С‚РѕР±С‹ СѓР±СЂР°С‚СЊ plain DNS query Рє Р»РѕРєР°Р»СЊРЅРѕРјСѓ СЂРѕСѓС‚РµСЂСѓ/РїСЂРѕРІР°Р№РґРµСЂСѓ.
func (e *Executor) lookupHostDoH(ctx context.Context, host, physGW string) ([]string, error) {
	if host == "" {
		return nil, fmt.Errorf("empty host")
	}
	var lastErr error

	for _, r := range dohResolvers {
		dialer := &net.Dialer{Timeout: 7 * time.Second}
		tr := &http.Transport{
			DialContext: func(c context.Context, _, _ string) (net.Conn, error) {
				return dialer.DialContext(c, "tcp", net.JoinHostPort(r.ip, "443"))
			},
			TLSClientConfig: &tls.Config{
				ServerName: r.serverName,
				MinVersion: tls.VersionTLS12,
			},
			TLSHandshakeTimeout: 7 * time.Second,
		}
		client := &http.Client{Transport: tr, Timeout: 10 * time.Second}

		u := fmt.Sprintf(r.urlTmpl, url.QueryEscape(host))
		req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Accept", "application/dns-json")
		req.Header.Set("User-Agent", chromeUA)

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		var out []string
		func() {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				lastErr = fmt.Errorf("%s doh status: %d", r.name, resp.StatusCode)
				return
			}
			var doh struct {
				Status int `json:"Status"`
				Answer []struct {
					Type int    `json:"type"`
					Data string `json:"data"`
				} `json:"Answer"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&doh); err != nil {
				lastErr = err
				return
			}
			if doh.Status != 0 {
				lastErr = fmt.Errorf("%s doh dns status: %d", r.name, doh.Status)
				return
			}
			out = make([]string, 0, len(doh.Answer))
			for _, a := range doh.Answer {
				if a.Type != 1 {
					continue
				}
				ip := strings.TrimSpace(a.Data)
				if parsed := net.ParseIP(ip); parsed != nil && parsed.To4() != nil {
					out = append(out, ip)
				}
			}
		}()

		if len(out) > 0 {
			return out, nil
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("all doh resolvers failed")
	}
	return nil, lastErr
}

// buildFallbackClient РёСЃРїРѕР»СЊР·СѓРµС‚ СЃС‚Р°РЅРґР°СЂС‚РЅС‹Р№ TLS stack РєР°Рє СЂРµР·РµСЂРІРЅС‹Р№ РІР°СЂРёР°РЅС‚,
// РєРѕРіРґР° utls РЅРµ РјРѕР¶РµС‚ СЃС‚Р°Р±РёР»СЊРЅРѕ Р·Р°РІРµСЂС€РёС‚СЊ handshake РІ С‚РµРєСѓС‰РµР№ СЃРёСЃС‚РµРјРµ.
// addCoverRoute РґРѕР±Р°РІР»СЏРµС‚ /32 РјР°СЂС€СЂСѓС‚ РґР»СЏ IP cover С…РѕСЃС‚Р° С‡РµСЂРµР· С„РёР·РёС‡РµСЃРєРёР№ С€Р»СЋР·.
// /32 РїСЂРёРѕСЂРёС‚РµС‚РЅРµРµ AWG 0.0.0.0/1 в†’ TCP Рє СЌС‚РѕРјСѓ IP РёРґС‘С‚ СЃРЅР°СЂСѓР¶Рё AWG С‚СѓРЅРµР»СЏ.
// Р‘РµР· bind(physIP) вЂ” WFP РЅРµ Р±Р»РѕРєРёСЂСѓРµС‚.
func addCoverRoute(ip, gw string, log *logger.Logger) {
	if gw == "" {
		return
	}
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.To4() == nil {
		return // С‚РѕР»СЊРєРѕ IPv4
	}
	// Р”РѕР±Р°РІР»СЏРµРј РІСЂРµРјРµРЅРЅС‹Р№ РјР°СЂС€СЂСѓС‚ (РІ Windows РѕРЅ РїСЂРѕРїР°РґРµС‚ РїРѕСЃР»Рµ РїРµСЂРµР·Р°РіСЂСѓР·РєРё РёР»Рё РµСЃР»Рё РЅРµ СѓРєР°Р·Р°РЅ -p)
	cmd := exec.Command("route", "add", ip, "mask", "255.255.255.255", gw)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000, HideWindow: true}
	if err := cmd.Run(); err != nil {
		log.Debugf("[Cover] Route add %s failed: %v (РІРѕР·РјРѕР¶РЅРѕ, СѓР¶Рµ СЃСѓС‰РµСЃС‚РІСѓРµС‚)", ip, err)
	} else {
		log.Debugf("[Cover] Route added: %s via %s", ip, gw)
	}
}

// pickIPv4 РІРѕР·РІСЂР°С‰Р°РµС‚ РїРµСЂРІС‹Р№ IPv4 РёР· СЃРїРёСЃРєР° (РїСЂРµРґРїРѕС‡С‚РёС‚РµР»СЊРЅРµРµ IPv6 РґР»СЏ /32 routing).
func pickIPv4(ips []string) string {
	for _, ip := range ips {
		if net.ParseIP(ip).To4() != nil {
			return ip
		}
	}
	return ""
}

func normalizeURL(rawURL string) string {
	if len(rawURL) < 8 || (rawURL[:7] != "http://" && rawURL[:8] != "https://") {
		return "https://" + rawURL
	}
	return rawURL
}

func (e *Executor) buildRequest(ctx context.Context, url, referer string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", chromeUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	if referer != "" {
		fullRef := normalizeURL(referer)
		req.Header.Set("Referer", fullRef)
		if sameHost(referer, url) {
			req.Header.Set("Sec-Fetch-Site", "same-origin")
		} else {
			req.Header.Set("Sec-Fetch-Site", "cross-site")
		}
	} else {
		req.Header.Set("Sec-Fetch-Site", "none")
	}
	return req, nil
}

func sameHost(a, b string) bool {
	return extractHost(a) == extractHost(b)
}

func extractHost(rawURL string) string {
	s := normalizeURL(rawURL)
	u, err := url.Parse(s)
	if err == nil {
		h := strings.TrimSpace(strings.ToLower(u.Hostname()))
		if h != "" {
			return h
		}
	}
	// Fallback РґР»СЏ РЅРµС‚РёРїРёС‡РЅС‹С… СЃС‚СЂРѕРє.
	if i := strings.Index(s, "/"); i >= 0 {
		s = s[:i]
	}
	if h, _, err := net.SplitHostPort(s); err == nil {
		return strings.ToLower(h)
	}
	return strings.ToLower(s)
}

const chromeUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
