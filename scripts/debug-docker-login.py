import paramiko

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect("192.168.5.24", username="root", password="123456", timeout=30, allow_agent=False, look_for_keys=False)
cmd = r"""
echo "=== token with creds ==="
curl -sk -u 'admin:Harbor12345!' 'https://dockerhub.kubekey.local/service/token?account=admin&service=harbor-registry&scope=registry:catalog:*' | head -c 120 ; echo
echo "=== token wrong pwd ==="
curl -sk -u 'admin:wrong' 'https://dockerhub.kubekey.local/service/token?account=admin&service=harbor-registry' | head -c 120 ; echo
echo "=== login hostname ==="
echo 'Harbor12345!' | docker login dockerhub.kubekey.local -u admin --password-stdin 2>&1
echo "host=$?"
echo "=== login IP ==="
echo 'Harbor12345!' | docker login 192.168.5.24 -u admin --password-stdin 2>&1
echo "ip=$?"
echo "=== certs.d ==="
ls -la /etc/docker/certs.d/*/ca.crt 2>/dev/null
echo "=== daemon.json ==="
cat /etc/docker/daemon.json
"""
_, o, _ = c.exec_command(cmd, get_pty=True, timeout=90)
text = o.read().decode("utf-8", "replace")
open(r"a:\21-Agent\SparkOpsAgent\scripts\harbor-login-debug.log", "w", encoding="utf-8").write(text)
print(text)
print("code", o.channel.recv_exit_status())
c.close()
