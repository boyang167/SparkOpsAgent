import paramiko
import json

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect("192.168.5.24", username="root", password="123456", timeout=30, allow_agent=False, look_for_keys=False)
cmd = """
set -e
CA_SRC=/etc/ssl/registry/ssl/ca.crt
for host in 192.168.5.24 dockerhub.kubekey.local; do
  mkdir -p /etc/docker/certs.d/${host}
  cp "$CA_SRC" /etc/docker/certs.d/${host}/ca.crt
  chmod 644 /etc/docker/certs.d/${host}/ca.crt
done

python3 - <<'PY'
import json
p = "/etc/docker/daemon.json"
with open(p) as f:
    d = json.load(f)
# Remove insecure-registries: Harbor only serves HTTPS; HTTP causes 502 on login
d.pop("insecure-registries", None)
with open(p, "w") as f:
    json.dump(d, f, indent=2)
    f.write("\\n")
print(json.dumps(d, indent=2))
PY

grep -q 'dockerhub.kubekey.local' /etc/hosts || echo '192.168.5.24 dockerhub.kubekey.local' >> /etc/hosts

systemctl restart docker
sleep 4

echo "=== login hostname (HTTPS+CA) ==="
echo 'Harbor12345!' | docker login dockerhub.kubekey.local -u admin --password-stdin 2>&1
echo "hostname_exit=$?"

echo "=== login IP (HTTPS+CA) ==="
echo 'Harbor12345!' | docker login 192.168.5.24 -u admin --password-stdin 2>&1
echo "ip_exit=$?"
"""
_, o, _ = c.exec_command(cmd, get_pty=True, timeout=120)
print(o.read().decode("utf-8", "replace"))
print("code", o.channel.recv_exit_status())
c.close()
