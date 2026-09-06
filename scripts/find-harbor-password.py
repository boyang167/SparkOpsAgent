import paramiko
import base64

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect("192.168.5.24", username="root", password="123456", timeout=30, allow_agent=False, look_for_keys=False)
cmd = r"""
grep -E 'harbor_admin_password|password' /home/softs/harbor/harbor.yml 2>/dev/null | head -5
grep -E 'harbor_admin_password|password' /opt/harbor/harbor.yml 2>/dev/null | head -5
echo "=== test passwords ==="
for pwd in 'Harbor12345' 'Harbor12345!' 'admin' '123456'; do
  code=$(curl -sk -o /dev/null -w '%{http_code}' -u "admin:${pwd}" https://192.168.5.24/api/v2.0/users/current)
  echo "admin:${pwd} -> HTTP $code"
done
"""
_, o, _ = c.exec_command(cmd, get_pty=True, timeout=60)
print(o.read().decode("utf-8", "replace"))
c.close()

print("decoded auth:", base64.b64decode("YWRtaW46SGFyYm9yMTIzNDU=").decode())
