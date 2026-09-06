#!/usr/bin/env python3
"""Configure Docker on remote host to trust Harbor registry."""
import paramiko
import sys

HOST = "192.168.5.24"
USER = "root"
PASS = "123456"

REMOTE_SCRIPT = r"""
set -e
echo "=== Host ==="
hostname

echo "=== Find Harbor CA ==="
CA=""
for p in \
  /etc/ssl/registry/ca.crt \
  /etc/harbor/ssl/ca.crt \
  /opt/harbor/common/config/registry/cert/ca.crt \
  /data/secret/cert/ca.crt; do
  if [ -s "$p" ]; then
    echo "FOUND: $p"
    openssl x509 -in "$p" -noout -subject -issuer 2>/dev/null || true
    CA="$p"
    break
  fi
done

if [ -z "$CA" ]; then
  echo "Searching ca.crt..."
  CA=$(find /etc /opt /data /root -name 'ca.crt' 2>/dev/null | grep -Ei 'registry|harbor|ssl' | head -1 || true)
  if [ -n "$CA" ] && [ -s "$CA" ]; then
    echo "FOUND via find: $CA"
    openssl x509 -in "$CA" -noout -subject -issuer 2>/dev/null || true
  fi
fi

if [ -z "$CA" ] || [ ! -s "$CA" ]; then
  for c in harbor-core registry nginx; do
    if docker ps --format '{{.Names}}' 2>/dev/null | grep -q "^${c}$"; then
      if docker exec "$c" test -s /etc/harbor/ssl/ca.crt 2>/dev/null; then
        CA="/tmp/harbor-ca-from-${c}.crt"
        docker exec "$c" cat /etc/harbor/ssl/ca.crt > "$CA"
        echo "FOUND in container $c -> $CA"
        openssl x509 -in "$CA" -noout -subject -issuer 2>/dev/null || true
        break
      fi
    fi
  done
fi

REG=192.168.5.24
mkdir -p /etc/docker/certs.d/${REG}

if [ -n "$CA" ] && [ -s "$CA" ]; then
  cp "$CA" /etc/docker/certs.d/${REG}/ca.crt
  chmod 644 /etc/docker/certs.d/${REG}/ca.crt
  echo "Installed CA from $CA"
  ls -la /etc/docker/certs.d/${REG}/ca.crt
  openssl x509 -in /etc/docker/certs.d/${REG}/ca.crt -noout -subject -issuer
else
  echo "WARN: CA file not found, will rely on insecure-registries"
fi

DAEMON=/etc/docker/daemon.json
if [ -f "$DAEMON" ]; then
  python3 - <<'PY'
import json
p = "/etc/docker/daemon.json"
with open(p) as f:
    d = json.load(f)
regs = set(d.get("insecure-registries") or [])
regs.update(["192.168.5.24", "dockerhub.kubekey.local"])
d["insecure-registries"] = sorted(regs)
with open(p, "w") as f:
    json.dump(d, f, indent=2)
    f.write("\n")
print("Updated", p)
print(json.dumps(d, indent=2))
PY
else
  cat > "$DAEMON" <<'JSON'
{
  "insecure-registries": ["192.168.5.24", "dockerhub.kubekey.local"]
}
JSON
  echo "Created $DAEMON"
  cat "$DAEMON"
fi

grep -q 'dockerhub.kubekey.local' /etc/hosts || echo '192.168.5.24 dockerhub.kubekey.local' >> /etc/hosts
grep dockerhub.kubekey.local /etc/hosts || true

echo "=== Restart docker ==="
if command -v systemctl >/dev/null 2>&1; then
  systemctl restart docker
  sleep 3
  systemctl is-active docker
else
  service docker restart
  sleep 3
fi

echo "=== Test docker login ==="
echo 'Harbor12345!' | docker login 192.168.5.24 -u admin --password-stdin
echo "=== Done ==="
"""


def main() -> int:
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    try:
        print(f"Connecting to {USER}@{HOST}...")
        client.connect(
            HOST,
            username=USER,
            password=PASS,
            timeout=30,
            allow_agent=False,
            look_for_keys=False,
        )
        stdin, stdout, stderr = client.exec_command(REMOTE_SCRIPT, get_pty=True, timeout=180)
        out = stdout.read().decode("utf-8", errors="replace")
        err = stderr.read().decode("utf-8", errors="replace")
        code = stdout.channel.recv_exit_status()
        print(out)
        if err.strip():
            print("STDERR:", err)
        print(f"Exit code: {code}")
        return code
    except Exception as exc:
        print(f"ERROR: {exc}")
        return 1
    finally:
        client.close()


if __name__ == "__main__":
    sys.exit(main())
