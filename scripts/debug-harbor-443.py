import paramiko

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect("192.168.5.24", username="root", password="123456", timeout=30, allow_agent=False, look_for_keys=False)
cmd = """
echo "=== hosts ==="
grep kubekey /etc/hosts
echo "=== curl IP 443 ==="
curl -skI https://192.168.5.24/v2/ | head -3
echo "=== curl hostname 443 ==="
curl -skI https://dockerhub.kubekey.local/v2/ | head -3
echo "=== ss 443 ==="
ss -lntp | grep ':443' || netstat -lntp | grep ':443' || true
echo "=== openssl hostname ==="
echo | openssl s_client -connect dockerhub.kubekey.local:443 -servername dockerhub.kubekey.local 2>/dev/null | openssl x509 -noout -subject 2>/dev/null || echo fail
"""
_, o, _ = c.exec_command(cmd, get_pty=True, timeout=60)
print(o.read().decode("utf-8", "replace"))
c.close()
