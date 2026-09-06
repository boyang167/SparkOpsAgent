import paramiko

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(
    "192.168.5.24",
    username="root",
    password="123456",
    timeout=30,
    allow_agent=False,
    look_for_keys=False,
)
cmd = """
echo "=== hosts ==="
grep kubekey /etc/hosts || true
echo "=== login by hostname ==="
echo 'Harbor12345!' | docker login dockerhub.kubekey.local -u admin --password-stdin 2>&1
echo "hostname_login=$?"
echo "=== login by IP insecure ==="
echo 'Harbor12345!' | docker login 192.168.5.24 -u admin --password-stdin 2>&1
echo "ip_login=$?"
echo "=== harbor API health ==="
curl -sk -u admin:Harbor12345! https://192.168.5.24/api/v2.0/health | head -c 200 ; echo
"""
_, o, _ = c.exec_command(cmd, get_pty=True, timeout=60)
print(o.read().decode("utf-8", "replace"))
print("code", o.channel.recv_exit_status())
c.close()
