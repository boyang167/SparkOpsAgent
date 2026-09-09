#!/usr/bin/env python3
"""Fix ImagePullBackOff: configure K8s node Docker to pull Harbor over HTTPS."""
import json
import sys
from typing import Tuple

import paramiko

HARBOR_HOST = "192.168.5.24"
K8S_HOST = "192.168.5.21"
USER = "root"
PASS = "123456"
REGISTRY = "192.168.5.24"
IMAGE = "192.168.5.24/test-workspace/test-repo:master"
NAMESPACE = "test-workspace"
DEPLOYMENT = "test-repo"


def run(client: paramiko.SSHClient, script: str, timeout: int = 180) -> Tuple[int, str, str]:
    stdin, stdout, stderr = client.exec_command(script, get_pty=True, timeout=timeout)
    out = stdout.read().decode("utf-8", errors="replace")
    err = stderr.read().decode("utf-8", errors="replace")
    code = stdout.channel.recv_exit_status()
    return code, out, err


def connect(host: str) -> paramiko.SSHClient:
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(host, username=USER, password=PASS, timeout=30, allow_agent=False, look_for_keys=False)
    return client


def main() -> int:
    harbor = connect(HARBOR_HOST)
    try:
        code, out, err = run(
            harbor,
            "cat /etc/docker/certs.d/192.168.5.24/ca.crt",
            timeout=30,
        )
        if code != 0 or "BEGIN CERTIFICATE" not in out:
            print("Failed to read Harbor CA:", out, err)
            return 1
        ca_pem = out.strip()
        print(f"Loaded Harbor CA ({len(ca_pem)} bytes)")
    finally:
        harbor.close()

    k8s = connect(K8S_HOST)
    try:
        remote = f"""
set -e
mkdir -p /etc/docker/certs.d/{REGISTRY}
cat > /etc/docker/certs.d/{REGISTRY}/ca.crt <<'EOF'
{ca_pem}
EOF
chmod 644 /etc/docker/certs.d/{REGISTRY}/ca.crt
if command -v python3 >/dev/null 2>&1; then
  PY=python3
elif command -v python >/dev/null 2>&1; then
  PY=python
else
  PY=""
fi
if [ -n "$PY" ]; then
  $PY - <<'PY'
import json
p = "/etc/docker/daemon.json"
with open(p) as f:
    d = json.load(f)
regs = [r for r in d.get("insecure-registries", []) if r != "{REGISTRY}"]
if regs:
    d["insecure-registries"] = regs
else:
    d.pop("insecure-registries", None)
with open(p, "w") as f:
    json.dump(d, f, indent=2)
    f.write("\\n")
print("Updated daemon.json:")
print(json.dumps(d, indent=2))
PY
else
  cp /etc/docker/daemon.json /etc/docker/daemon.json.bak
  cat > /etc/docker/daemon.json <<'JSON'
{{
  "log-opts": {{
    "max-size": "5m",
    "max-file": "3"
  }},
  "insecure-registries": ["dockerhub.kubekey.local"],
  "exec-opts": ["native.cgroupdriver=systemd"]
}}
JSON
  echo "Rewrote daemon.json without {REGISTRY} in insecure-registries"
  cat /etc/docker/daemon.json
fi
echo "=== Restart docker ==="
systemctl restart docker
sleep 5
systemctl is-active docker
echo "=== Docker login ==="
echo 'Harbor12345!' | docker login {REGISTRY} -u admin --password-stdin
echo "=== Test pull ==="
docker pull {IMAGE}
echo "=== Restart deployment ==="
kubectl rollout restart deployment/{DEPLOYMENT} -n {NAMESPACE}
kubectl rollout status deployment/{DEPLOYMENT} -n {NAMESPACE} --timeout=180s
kubectl get pods -n {NAMESPACE} -l app={DEPLOYMENT} -o wide
"""
        code, out, err = run(k8s, remote, timeout=240)
        print(out)
        if err.strip():
            print("STDERR:", err)
        print(f"Exit code: {code}")
        return code
    finally:
        k8s.close()


if __name__ == "__main__":
    sys.exit(main())
