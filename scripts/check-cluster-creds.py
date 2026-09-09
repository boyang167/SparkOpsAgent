#!/usr/bin/env python3
import paramiko

HOST = "192.168.5.24"
USER = "root"
PASS = "123456"

SQL = (
    "SELECT id, cluster_name, server, "
    "length(coalesce(bearer_token,'')) AS token_len, "
    "length(coalesce(client_certificate_data,'')) AS cert_len, "
    "length(coalesce(client_key_data,'')) AS key_len, "
    "length(coalesce(certificate_authority_data,'')) AS ca_len "
    "FROM platform_admin.cluster_info ORDER BY cluster_name;"
)

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect(HOST, username=USER, password=PASS, timeout=30, allow_agent=False, look_for_keys=False)
cmd = f"docker exec postgres_db psql -U admin -d sparkops_agent -c \"{SQL}\""
_, stdout, stderr = client.exec_command(cmd, timeout=30)
print(stdout.read().decode())
err = stderr.read().decode()
if err.strip():
    print("ERR:", err)
client.close()
