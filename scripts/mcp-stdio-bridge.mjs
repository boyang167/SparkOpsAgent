#!/usr/bin/env node
/**
 * Stdio MCP bridge → SparkOps HTTP JSON-RPC (/api/v1/mcp/rpc).
 * For clients that only support stdio (legacy). Prefer url + /api/v1/mcp in mcp.json.
 */
import readline from "node:readline";
import { stdin as input, stdout as output, stderr as error } from "node:process";

const apiURL = (process.env.SPARKOPS_API_URL ?? "http://localhost:8080").replace(/\/$/, "");
const token = process.env.SPARKOPS_TOKEN?.trim();
const workspaceId = process.env.SPARKOPS_WORKSPACE_ID?.trim();

if (!token || !workspaceId) {
  error.write("SPARKOPS_TOKEN and SPARKOPS_WORKSPACE_ID are required\n");
  process.exit(1);
}

const rl = readline.createInterface({ input, crlfDelay: Infinity });

rl.on("line", async (line) => {
  const trimmed = line.trim();
  if (!trimmed) return;

  let req;
  try {
    req = JSON.parse(trimmed);
  } catch {
    writeRpc({ jsonrpc: "2.0", error: { code: -32700, message: "parse error" } });
    return;
  }

  try {
    const res = await fetch(`${apiURL}/api/v1/mcp/rpc`, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${token}`,
        "X-Workspace-Id": workspaceId,
        "Content-Type": "application/json",
      },
      body: JSON.stringify(req),
    });
    const text = await res.text();
    if (res.status === 401) {
      writeRpc({
        jsonrpc: "2.0",
        id: req.id,
        error: { code: -32000, message: "unauthorized (check SPARKOPS_TOKEN)" },
      });
      return;
    }
    if (res.status >= 400) {
      writeRpc({
        jsonrpc: "2.0",
        id: req.id,
        error: { code: -32000, message: `upstream HTTP ${res.status}: ${text.trim()}` },
      });
      return;
    }
    output.write(`${text.trim()}\n`);
  } catch (err) {
    writeRpc({
      jsonrpc: "2.0",
      id: req.id,
      error: { code: -32000, message: err instanceof Error ? err.message : String(err) },
    });
  }
});

rl.on("close", () => {
  process.exit(0);
});

function writeRpc(payload) {
  output.write(`${JSON.stringify(payload)}\n`);
}
