import paramiko

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect("192.168.5.24", username="root", password="123456", timeout=30, allow_agent=False, look_for_keys=False)
cmd = r"""
echo "=== harbor auth config ==="
docker exec harbor-core env | grep -E 'AUTH|REGISTRY|EXT' | sort | head -20
echo "=== try admin harbor API ==="
curl -sk -u 'admin:Harbor12345!' https://192.168.5.24/api/v2.0/users/current ; echo
curl -sk -u 'admin:Harbor12345!' https://dockerhub.kubekey.local/api/v2.0/users/current ; echo
echo "=== v2 with bearer token ==="
TOKEN=$(curl -sk -u 'admin:Harbor12345!' 'https://dockerhub.kubekey.local/service/token?account=admin&service=harbor-registry&scope=registry:catalog:*' | python3 -c 'import sys,json; print(json.load(sys.stdin)["token"])')
curl -sk -H "Authorization: Bearer $TOKEN" https://192.168.5.24/v2/ ; echo
echo "=== docker version client ==="
docker version
echo "=== config.json before login ==="
cat ~/.docker/config.json 2>/dev/null || echo none
"""
_, o, _ = c.exec_command(cmd, get_pty=True, timeout=90)
text = o.read().decode("utf-8", "replace")
print(text.encode("ascii", "replace").decode())
c.close()
