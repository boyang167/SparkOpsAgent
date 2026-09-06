import paramiko

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect("192.168.5.24", username="root", password="123456", timeout=30, allow_agent=False, look_for_keys=False)
cmd = """
echo "=== v2 headers ==="
curl -skI https://192.168.5.24/v2/ | tr -d '\r'
echo "=== token request ==="
curl -sk -u admin:Harbor12345! 'https://192.168.5.24/service/token?account=admin&service=harbor-registry&scope=repository:test-workspace/test-repo:pull,push' ; echo
echo "=== core ping ==="
curl -sk -u admin:Harbor12345! https://192.168.5.24/api/v2.0/users/current ; echo
"""
_, o, _ = c.exec_command(cmd, get_pty=True, timeout=60)
print(o.read().decode("utf-8", "replace"))
c.close()
