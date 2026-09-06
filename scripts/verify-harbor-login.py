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
echo "=== curl v2 ==="
curl -sk https://192.168.5.24/v2/ ; echo
echo "=== docker login ==="
echo 'Harbor12345!' | docker login 192.168.5.24 -u admin --password-stdin 2>&1
echo "login_exit=$?"
echo "=== images ==="
docker images | grep 'test-workspace/test-repo' | head -5 || true
"""
_, o, _ = c.exec_command(cmd, get_pty=True, timeout=60)
print(o.read().decode("utf-8", "replace"))
print("code", o.channel.recv_exit_status())
c.close()
