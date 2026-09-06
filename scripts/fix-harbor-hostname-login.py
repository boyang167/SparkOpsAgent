import paramiko

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect("192.168.5.24", username="root", password="123456", timeout=30, allow_agent=False, look_for_keys=False)
cmd = """
set -e
CA=/etc/docker/certs.d/192.168.5.24/ca.crt
mkdir -p /etc/docker/certs.d/dockerhub.kubekey.local
cp "$CA" /etc/docker/certs.d/dockerhub.kubekey.local/ca.crt
chmod 644 /etc/docker/certs.d/dockerhub.kubekey.local/ca.crt
echo "cert installed for dockerhub.kubekey.local"
systemctl restart docker
sleep 3
echo "=== login hostname ==="
echo 'Harbor12345!' | docker login dockerhub.kubekey.local -u admin --password-stdin 2>&1
echo "hostname_exit=$?"
echo "=== login IP ==="
echo 'Harbor12345!' | docker login 192.168.5.24 -u admin --password-stdin 2>&1
echo "ip_exit=$?"
"""
_, o, _ = c.exec_command(cmd, get_pty=True, timeout=90)
print(o.read().decode("utf-8", "replace"))
print("code", o.channel.recv_exit_status())
c.close()
