// internal/payload/scripts.go
package payload

import (
	"bytes"
	"fmt"
	"text/template"
)

// DeployParams stores variables required for deploy templates.
type DeployParams struct {
	AWGListenPort    string
	AWGServerPrivKey string
	AWGServerPubKey  string
	AWGClientPrivKey string
	AWGClientPubKey  string
	AWGClientIP      string
	AWGServerIP      string
	MTU              int
	DecoyDomain      string

	// AmneziaWG junk obfuscation
	Jc, Jmin, Jmax int
	S1, S2         int
	H1, H2, H3, H4 int
}

func DeployScript(p DeployParams) (string, error) {
	return render(deployTmpl, p)
}

func HotSwapScript(newDomain string) string {
	return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
CONF=/opt/furor/xray/config.json
sed -i 's|"address":.*|"address": "%s",|' "$CONF"
docker kill --signal HUP xray 2>/dev/null || docker restart xray
echo "HotSwap OK -> %s"
`, newDomain, newDomain)
}

func VerifyScript() string {
	return `#!/usr/bin/env bash
echo "=== AWG ==="
systemctl is-active awg-quick@furor && echo "AWG: OK" || echo "AWG: FAIL"
echo "=== xray ==="
docker ps --filter name=xray --format "xray: {{.Status}}" 2>/dev/null || echo "xray: not running"
echo "=== UFW ==="
ufw status | head -20
echo "=== fail2ban ==="
systemctl is-active fail2ban && echo "fail2ban: OK" || echo "fail2ban: FAIL"
echo "=== dnsmasq ==="
systemctl is-active dnsmasq && echo "dnsmasq: OK" || echo "dnsmasq: FAIL"
echo "=== Ports ==="
ss -tulnp | grep -E ':(53|443) '
`
}

var deployTmpl = `#!/usr/bin/env bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

echo "----------------------------------------"
echo "  Furor Davidis - Deploy VPS"
echo "  <<Scrutator frustra laborat>>"
echo "----------------------------------------"

echo "-- [1] apt update..."
apt-get update -qq
apt-get install -y -qq ca-certificates curl gnupg lsb-release iptables dnsmasq

echo "-- [2] AmneziaWG..."
if ! command -v awg &>/dev/null; then
    apt-get install -y -qq software-properties-common python3-launchpadlib gnupg2 \
        linux-headers-$(uname -r) dkms

    OS_ID=$(grep '^ID=' /etc/os-release | cut -d= -f2 | tr -d '"')
    if [ "$OS_ID" = "ubuntu" ] || [ "$OS_ID" = "linuxmint" ]; then
        grep -q "^deb-src" /etc/apt/sources.list || \
            sed -n 's/^deb /deb-src /p' /etc/apt/sources.list | head -1 \
            >> /etc/apt/sources.list
        add-apt-repository -y ppa:amnezia/ppa
        apt-get update -qq
        apt-get install -y -qq amneziawg
    else
        apt-key adv --keyserver keyserver.ubuntu.com --recv-keys 57290828
        grep -q "ppa.launchpadcontent.net/amnezia" /etc/apt/sources.list || {
            echo "deb https://ppa.launchpadcontent.net/amnezia/ppa/ubuntu focal main" \
                >> /etc/apt/sources.list
            echo "deb-src https://ppa.launchpadcontent.net/amnezia/ppa/ubuntu focal main" \
                >> /etc/apt/sources.list
        }
        apt-get update -qq
        apt-get install -y -qq amneziawg
    fi
fi
echo "AWG: $(awg --version 2>&1 | head -1)"

echo "-- [3] AWG config..."
mkdir -p /etc/amnezia/amneziawg

cat > /etc/amnezia/amneziawg/furor.conf << 'AWGEOF'
[Interface]
PrivateKey = {{.AWGServerPrivKey}}
Address = {{.AWGServerIP}}/24
ListenPort = {{.AWGListenPort}}
Jc = {{.Jc}}
Jmin = {{.Jmin}}
Jmax = {{.Jmax}}
S1 = {{.S1}}
S2 = {{.S2}}
H1 = {{.H1}}
H2 = {{.H2}}
H3 = {{.H3}}
H4 = {{.H4}}
PostUp = iptables -A FORWARD -i furor -j ACCEPT; iptables -A FORWARD -o furor -j ACCEPT; iptables -t nat -A POSTROUTING -o $(ip route | awk '/default/{print $5}' | head -1) -j MASQUERADE
PostDown = iptables -D FORWARD -i furor -j ACCEPT; iptables -D FORWARD -o furor -j ACCEPT; iptables -t nat -D POSTROUTING -o $(ip route | awk '/default/{print $5}' | head -1) -j MASQUERADE

[Peer]
PublicKey = {{.AWGClientPubKey}}
AllowedIPs = {{.AWGClientIP}}/32
AWGEOF

echo 1 > /proc/sys/net/ipv4/ip_forward
grep -q '^net.ipv4.ip_forward' /etc/sysctl.conf \
    && sed -i 's/^net.ipv4.ip_forward.*/net.ipv4.ip_forward=1/' /etc/sysctl.conf \
    || echo "net.ipv4.ip_forward=1" >> /etc/sysctl.conf

systemctl enable awg-quick@furor
systemctl restart awg-quick@furor
echo "AWG: started on UDP:{{.AWGListenPort}}"

echo "-- [4] dnsmasq..."
cat > /etc/dnsmasq.d/furor.conf << 'DNSEOF'
port=53
bind-interfaces
listen-address={{.AWGServerIP}},127.0.0.1
server=1.1.1.1
server=9.9.9.9
cache-size=2000
no-resolv
DNSEOF
systemctl enable dnsmasq
systemctl restart dnsmasq
echo "dnsmasq: started on {{.AWGServerIP}}:53"

echo "-- [5] Docker..."
if ! command -v docker &>/dev/null; then
    curl -fsSL https://get.docker.com | sh
fi
systemctl enable docker
systemctl start docker
echo "Docker: $(docker --version)"

echo "-- [6] xray decoy -> {{.DecoyDomain}}..."
mkdir -p /opt/furor/xray

python3 - << 'PYEOF'
import json, os

cfg = {
    "log": {"loglevel": "warning"},
    "inbounds": [{
        "port": 443,
        "protocol": "dokodemo-door",
        "settings": {
            "address": "{{.DecoyDomain}}",
            "port": 443,
            "network": "tcp",
            "followRedirect": False
        },
        "tag": "decoy"
    }],
    "outbounds": [
        {"protocol": "freedom", "tag": "direct"}
    ]
}

os.makedirs("/opt/furor/xray", exist_ok=True)
with open("/opt/furor/xray/config.json", "w") as f:
    json.dump(cfg, f, indent=2)
print("xray config saved")
PYEOF

docker rm -f xray 2>/dev/null || true
docker run -d \
    --name xray \
    --restart unless-stopped \
    --network host \
    -v /opt/furor/xray:/etc/xray \
    teddysun/xray:latest \
    xray run -config /etc/xray/config.json

sleep 2
docker ps | grep xray && echo "xray: OK" || echo "xray: WARN (check docker logs xray)"

echo "-- [7] UFW..."
apt-get install -y -qq ufw
ufw --force reset
ufw default deny incoming
ufw default allow outgoing
sed -i 's/DEFAULT_FORWARD_POLICY="DROP"/DEFAULT_FORWARD_POLICY="ACCEPT"/' /etc/default/ufw
sed -i 's|#net/ipv4/ip_forward=1|net/ipv4/ip_forward=1|' /etc/ufw/sysctl.conf
ufw allow {{.AWGListenPort}}/udp comment "AWG"
ufw allow in on furor to any port 53 proto udp comment "DNS via AWG"
ufw allow in on furor to any port 53 proto tcp comment "DNS via AWG"
ufw allow 443/tcp comment "xray decoy"
ufw allow 22/tcp comment "SSH"
ufw --force enable
echo 1 > /proc/sys/net/ipv4/ip_forward
echo "UFW: OK (forward=ACCEPT)"

echo "-- [8] fail2ban..."
apt-get install -y -qq fail2ban || true
cat > /etc/fail2ban/jail.d/furor.conf << 'F2BEOF'
[sshd]
enabled  = true
port     = 22
filter   = sshd
logpath  = /var/log/auth.log
maxretry = 5
bantime  = 3600
findtime = 600
F2BEOF
systemctl enable fail2ban
systemctl restart fail2ban
echo "fail2ban: OK (maxretry=5, bantime=1h)"

echo ""
echo "----------------------------------------"
echo "  DEPLOY COMPLETE"
echo "  AWG UDP : {{.AWGListenPort}}"
echo "  DNS     : {{.AWGServerIP}}:53 (dnsmasq)"
echo "  xray    : TCP:443 -> {{.DecoyDomain}}"
echo "  fail2ban: active"
echo "----------------------------------------"
`

// ClientConfig returns AWG config for the Windows client (amneziawg.exe).
func ClientConfig(p DeployParams, vpsIP string) string {
	return fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s/24
DNS = %s
Jc = %d
Jmin = %d
Jmax = %d
S1 = %d
S2 = %d
H1 = %d
H2 = %d
H3 = %d
H4 = %d
MTU = %d

[Peer]
PublicKey = %s
Endpoint = %s:%s
AllowedIPs = 0.0.0.0/1, 128.0.0.0/1
PersistentKeepalive = 25
`,
		p.AWGClientPrivKey,
		p.AWGClientIP,
		p.AWGServerIP,
		p.Jc, p.Jmin, p.Jmax,
		p.S1, p.S2,
		p.H1, p.H2, p.H3, p.H4,
		p.MTU,
		p.AWGServerPubKey,
		vpsIP, p.AWGListenPort,
	)
}

func render(src string, p DeployParams) (string, error) {
	t, err := template.New("").Parse(src)
	if err != nil {
		return "", fmt.Errorf("template parse: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, p); err != nil {
		return "", fmt.Errorf("template execute: %w", err)
	}
	return buf.String(), nil
}
