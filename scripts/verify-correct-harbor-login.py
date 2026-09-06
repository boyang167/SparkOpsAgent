import paramiko

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect("192.168.5.24", username="root", password="123456", timeout=30, allow_agent=False, look_for_keys=False)
cmd = r"""
echo "=== docker login IP (correct pwd) ==="
echo 'Harbor12345' | docker login 192.168.5.24 -u admin --password-stdin 2>&1
echo "ip=$?"
echo "=== docker login hostname ==="
echo 'Harbor12345' | docker login dockerhub.kubekey.local -u admin --password-stdin 2>&1
echo "host=$?"
echo "=== test push ==="
docker push 192.168.5.24/test-workspace/test-repo:ffa162826cb5 2>&1 | tail -8
"""
_, o, _ = c.exec_command(cmd, get_pty=True, timeout=180)
text = o.read().decode("utf-8", "replace")
print(text)
print("code", o.channel.recv_exit_status())
c.close()
