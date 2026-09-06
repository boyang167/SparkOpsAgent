import paramiko

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect("192.168.5.24", username="root", password="123456", timeout=30, allow_agent=False, look_for_keys=False)
cmd = """
echo "=== curl v2 with basic auth ==="
curl -sk -u admin:Harbor12345! https://192.168.5.24/v2/ ; echo
curl -sk -u admin:Harbor12345! https://192.168.5.24/v2/_catalog ; echo
echo "=== docker version ==="
docker version --format '{{.Client.Version}} / {{.Server.Version}}'
echo "=== docker info registry ==="
docker info 2>/dev/null | grep -i insecure -A2 || true
echo "=== try login debug DOCKER_CLI_HINTS=false ==="
DOCKER_CLI_EXPERIMENTAL=enabled docker login 192.168.5.24 -u admin -p Harbor12345! 2>&1 || true
echo "=== check registry container ==="
docker ps --format '{{.Names}}' | grep -E 'registry|harbor' | head -10
"""
_, o, _ = c.exec_command(cmd, get_pty=True, timeout=60)
print(o.read().decode("utf-8", "replace"))
c.close()
