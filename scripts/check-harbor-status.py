import paramiko

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect("192.168.5.24", username="root", password="123456", timeout=30, allow_agent=False, look_for_keys=False)
cmd = """
echo "=== docker ps harbor ==="
docker ps -a --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}' | grep -E 'harbor|registry|NAMES' | head -20
echo "=== ss listeners ==="
ss -lntp | grep -E ':443|:80|:9999' || true
echo "=== try start harbor if compose exists ==="
for d in /opt/harbor /home/softs/harbor /data/harbor; do
  if [ -f "$d/docker-compose.yml" ]; then echo "compose: $d"; fi
done
ls /home/softs/jenkins2/docker-compose.yml 2>/dev/null || true
"""
_, o, _ = c.exec_command(cmd, get_pty=True, timeout=60)
print(o.read().decode("utf-8", "replace"))
c.close()
