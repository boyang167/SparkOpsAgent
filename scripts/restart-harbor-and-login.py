import paramiko
import time

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect("192.168.5.24", username="root", password="123456", timeout=30, allow_agent=False, look_for_keys=False)
cmd = """
set -e
cd /home/softs/harbor
echo "=== restart harbor ==="
docker-compose up -d
sleep 15
docker ps --format 'table {{.Names}}\t{{.Status}}' | grep -E 'harbor|registry|NAMES'
echo "=== wait for 443 ==="
for i in 1 2 3 4 5 6 7 8 9 10; do
  if curl -skI https://192.168.5.24/v2/ 2>/dev/null | head -1 | grep -q 401; then
    echo "Harbor ready"
    break
  fi
  echo "waiting... $i"
  sleep 3
done
curl -skI https://192.168.5.24/v2/ | head -3
echo "=== docker login IP ==="
echo 'Harbor12345!' | docker login 192.168.5.24 -u admin --password-stdin 2>&1
echo "ip_exit=$?"
echo "=== docker login hostname ==="
echo 'Harbor12345!' | docker login dockerhub.kubekey.local -u admin --password-stdin 2>&1
echo "host_exit=$?"
"""
_, o, _ = c.exec_command(cmd, get_pty=True, timeout=300)
print(o.read().decode("utf-8", "replace"))
print("code", o.channel.recv_exit_status())
c.close()
