#!/usr/bin/env node
// Screenshot loop: seeds data/asterwaf-seed.db, starts a throwaway asterwaf
// instance on loopback ports, logs in, and captures every page in both themes
// at 1440/768/390 into docs/screenshots/<page>-<theme>-<width>.png. Fails on
// horizontal overflow, an unapplied theme, or console errors/warnings.
// Depends only on the repo Node and a system Chromium; no npm dependency.
import { spawn } from "node:child_process";
import { mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { createHash, randomBytes } from "node:crypto";
import { tmpdir } from "node:os";
import path from "node:path";
import net from "node:net";

const root = path.resolve(import.meta.dirname, "..");
const outDir = path.resolve(root, process.env.ASTERWAF_SHOTS_DIR ?? "docs/screenshots");
const chromium =
  process.env.ASTERWAF_CHROMIUM ?? "/home/willtanoe/.cache/ms-playwright/chromium-1248/chrome-linux64/chrome";

const proxyPort = 18180;
const adminPort = 18190;
const baseUrl = `http://127.0.0.1:${adminPort}`;

const widths = [1440, 768, 390];
const themes = ["dark", "light"];
const pages = [
  { name: "overview", route: "/" },
  { name: "logs", route: "/logs" },
  { name: "rules", route: "/rules" },
  { name: "access-lists", route: "/access-lists" },
  { name: "sites", route: "/sites" },
  { name: "login", route: "/login", anonymous: true },
];

// Generic demo sites: the shots instance must be reproducible on any clone,
// so it never reads the deployed config.yaml.
const demoConfig = `database: ${path.join(root, "data", "asterwaf-seed.db")}
auth:
  username: admin
  password_hash: ""
  secure_cookie: false
listen: 127.0.0.1:${proxyPort}
admin_listen: 127.0.0.1:${adminPort}
mode: enforce
trusted_proxies: []
sites:
  - host: app.example.com
    upstream: http://127.0.0.1:18601
    health_path: /
  - host: api.example.com
    upstream: http://127.0.0.1:18602
    mode: shadow
    health_path: /
  - host: intranet.example.com
    upstream: http://127.0.0.1:18603
    health_path: /
inspect_headers:
  - User-Agent
  - Referer
  - X-Forwarded-For
rate_limit:
  enabled: false
  requests_per_minute: 600
  burst: 100
`;

async function runBinary(args, timeoutMs = 120000) {
  const child = spawn(path.join(root, "bin", "asterwaf"), args, {
    cwd: root,
    stdio: ["ignore", "ignore", "inherit"],
  });
  const timer = setTimeout(() => child.kill("SIGTERM"), timeoutMs);
  const code = await new Promise((resolve) => child.once("exit", resolve));
  clearTimeout(timer);
  if (code !== 0) {
    throw new Error(`asterwaf ${args.join(" ")} exited with ${code}`);
  }
}

async function startServer(configPath) {
  const child = spawn(path.join(root, "bin", "asterwaf"), ["-config", configPath], {
    cwd: root,
    stdio: ["ignore", "ignore", "inherit"],
  });
  const deadline = Date.now() + 20000;
  while (Date.now() < deadline) {
    if (child.exitCode !== null) {
      throw new Error(`asterwaf server exited with ${child.exitCode}`);
    }
    try {
      const response = await fetch(`${baseUrl}/healthz`);
      if (response.ok) {
        return child;
      }
    } catch {
      // belum siap.
    }
    await new Promise((resolve) => setTimeout(resolve, 150));
  }
  child.kill("SIGTERM");
  throw new Error("asterwaf server did not become healthy");
}

async function readPassword() {
  const file = path.join(root, "data", "first-run-admin-password");
  const deadline = Date.now() + 5000;
  while (Date.now() < deadline) {
    try {
      const value = (await readFile(file, "utf8")).trim();
      if (value !== "") {
        return value;
      }
    } catch {
      // server belum menulis file.
    }
    await new Promise((resolve) => setTimeout(resolve, 200));
  }
  throw new Error("first-run password file not found after server start");
}

async function login(base, password) {
  const response = await fetch(`${base}/api/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username: "admin", password }),
  });
  if (!response.ok) {
    throw new Error(`login failed with status ${response.status}`);
  }
  const cookie = response.headers.getSetCookie?.() ?? [];
  const session = cookie.find((value) => value.startsWith("asterwaf_session="));
  if (!session) {
    throw new Error("login response did not set a session cookie");
  }
  return session.split(";")[0].split("=")[1];
}

function encodeFrame(text) {
  const payload = Buffer.from(text, "utf8");
  const mask = randomBytes(4);
  let header;
  if (payload.length < 126) {
    header = Buffer.alloc(2);
    header[1] = 0x80 | payload.length;
  } else if (payload.length < 65536) {
    header = Buffer.alloc(4);
    header[1] = 0x80 | 126;
    header.writeUInt16BE(payload.length, 2);
  } else {
    header = Buffer.alloc(10);
    header[1] = 0x80 | 127;
    header.writeBigUInt64BE(BigInt(payload.length), 2);
  }
  header[0] = 0x81;
  const masked = Buffer.alloc(payload.length);
  for (let i = 0; i < payload.length; i += 1) {
    masked[i] = payload[i] ^ mask[i % 4];
  }
  return Buffer.concat([header, mask, masked]);
}

class Cdp {
  constructor(socket) {
    this.socket = socket;
    this.buffer = Buffer.alloc(0);
    this.nextId = 1;
    this.pending = new Map();
    this.listeners = [];
    socket.on("data", (chunk) => this.onData(chunk));
  }

  onData(chunk) {
    this.buffer = Buffer.concat([this.buffer, chunk]);
    for (;;) {
      if (this.buffer.length < 2) return;
      const first = this.buffer[0];
      const second = this.buffer[1];
      const opcode = first & 0x0f;
      let length = second & 0x7f;
      let offset = 2;
      if (length === 126) {
        if (this.buffer.length < 4) return;
        length = this.buffer.readUInt16BE(2);
        offset = 4;
      } else if (length === 127) {
        if (this.buffer.length < 10) return;
        length = Number(this.buffer.readBigUInt64BE(2));
        offset = 10;
      }
      if (this.buffer.length < offset + length) return;
      const payload = this.buffer.subarray(offset, offset + length);
      this.buffer = this.buffer.subarray(offset + length);
      if (opcode === 0x8) {
        this.socket.end();
        return;
      }
      if (opcode === 0x9) {
        this.sendFrame(0x8a, payload);
        continue;
      }
      if (opcode !== 0x1) continue;
      let message;
      try {
        message = JSON.parse(payload.toString("utf8"));
      } catch {
        continue;
      }
      this.dispatch(message);
    }
  }

  sendFrame(first, payload) {
    const mask = randomBytes(4);
    let header;
    if (payload.length < 126) {
      header = Buffer.alloc(2);
      header[1] = 0x80 | payload.length;
    } else if (payload.length < 65536) {
      header = Buffer.alloc(4);
      header[1] = 0x80 | 126;
      header.writeUInt16BE(payload.length, 2);
    } else {
      header = Buffer.alloc(10);
      header[1] = 0x80 | 127;
      header.writeBigUInt64BE(BigInt(payload.length), 2);
    }
    header[0] = first;
    const masked = Buffer.alloc(payload.length);
    for (let i = 0; i < payload.length; i += 1) {
      masked[i] = payload[i] ^ mask[i % 4];
    }
    this.socket.write(Buffer.concat([header, mask, masked]));
  }

  dispatch(message) {
    if (message.id !== undefined && this.pending.has(message.id)) {
      const { resolve, reject } = this.pending.get(message.id);
      this.pending.delete(message.id);
      if (message.error) {
        reject(new Error(`${message.error.message} (${message.error.code})`));
      } else {
        resolve(message.result);
      }
      return;
    }
    for (const listener of this.listeners) {
      listener(message);
    }
  }

  send(method, params = {}, sessionId) {
    const id = this.nextId++;
    const message = { id, method, params };
    if (sessionId) message.sessionId = sessionId;
    const promise = new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
    });
    this.sendFrame(0x81, Buffer.from(JSON.stringify(message), "utf8"));
    return promise;
  }

  waitFor(predicate) {
    return new Promise((resolve) => {
      const listener = (message) => {
        if (!predicate(message)) return;
        this.listeners = this.listeners.filter((item) => item !== listener);
        resolve(message);
      };
      this.listeners.push(listener);
    });
  }

  close() {
    this.socket.end();
  }
}

function connect(wsUrl) {
  return new Promise((resolve, reject) => {
    const url = new URL(wsUrl);
    const key = randomBytes(16).toString("base64");
    const socket = net.connect({ host: url.hostname, port: Number(url.port) }, () => {
      socket.write(
        `GET ${url.pathname}${url.search} HTTP/1.1\r\n` +
          `Host: ${url.host}\r\n` +
          "Upgrade: websocket\r\n" +
          "Connection: Upgrade\r\n" +
          `Sec-WebSocket-Key: ${key}\r\n` +
          "Sec-WebSocket-Version: 13\r\n\r\n",
      );
    });
    let handshake = Buffer.alloc(0);
    const onData = (chunk) => {
      handshake = Buffer.concat([handshake, chunk]);
      const end = handshake.indexOf("\r\n\r\n");
      if (end < 0) return;
      const header = handshake.subarray(0, end).toString("latin1");
      if (!header.includes("101")) {
        reject(new Error(`websocket upgrade failed: ${header.split("\r\n")[0]}`));
        socket.destroy();
        return;
      }
      const expected = createHash("sha1")
        .update(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11")
        .digest("base64");
      if (!header.includes(expected)) {
        reject(new Error("websocket accept key mismatch"));
        socket.destroy();
        return;
      }
      socket.off("data", onData);
      const rest = handshake.subarray(end + 4);
      const cdp = new Cdp(socket);
      if (rest.length > 0) cdp.onData(rest);
      resolve(cdp);
    };
    socket.on("data", onData);
    socket.on("error", reject);
  });
}

async function waitForDebugger(port, child, timeoutMs = 15000) {
  const deadline = Date.now() + timeoutMs;
  let exited = false;
  child.once("exit", () => {
    exited = true;
  });
  while (Date.now() < deadline) {
    if (exited) {
      throw new Error("chromium exited before the debugging port opened");
    }
    try {
      const response = await fetch(`http://127.0.0.1:${port}/json/version`);
      if (response.ok) {
        return await response.json();
      }
    } catch {
      // 端口尚未就绪。
    }
    await new Promise((resolve) => setTimeout(resolve, 150));
  }
  throw new Error("timed out waiting for the chromium debugging port");
}

async function main() {
  await mkdir(path.dirname(path.join(root, "data", "asterwaf-seed.db")), { recursive: true });
  const configPath = path.join(tmpdir(), `asterwaf-shots-${Date.now()}.yaml`);
  await writeFile(configPath, demoConfig, "utf8");
  await runBinary(["seed", "-config", configPath]);

  const server = await startServer(configPath);
  const problems = [];
  try {
    const password = await readPassword();
    const session = await login(baseUrl, password);
    await mkdir(outDir, { recursive: true });
    const profile = await mkdtemp(path.join(tmpdir(), "asterwaf-chrome-"));
    const port = 9300 + Math.floor(Math.random() * 400);
    const child = spawn(
      chromium,
      [
        "--headless=new",
        "--no-sandbox",
        "--disable-gpu",
        "--hide-scrollbars",
        `--remote-debugging-port=${port}`,
        `--user-data-dir=${profile}`,
        "about:blank",
      ],
      { stdio: ["ignore", "ignore", "pipe"] },
    );
    try {
      const version = await waitForDebugger(port, child);
      const cdp = await connect(version.webSocketDebuggerUrl);
      const { targetId } = await cdp.send("Target.createTarget", { url: "about:blank" });
      const { sessionId } = await cdp.send("Target.attachToTarget", { targetId, flatten: true });
      const send = (method, params) => cdp.send(method, params, sessionId);

      await send("Page.enable");
      await send("Network.enable");
      await send("Runtime.enable");
      await send("Log.enable");
      const consoleMessages = [];
      const onConsole = (message) => {
        if (message.sessionId !== sessionId) return;
        if (message.method === "Runtime.consoleAPICalled") {
          const type = message.params.type;
          if (type === "error" || type === "warning") {
            consoleMessages.push(`${type}: ${message.params.args.map((arg) => arg.value ?? arg.description ?? "").join(" ")}`);
          }
        }
        if (message.method === "Log.entryAdded") {
          const entry = message.params.entry;
          if (entry.level === "error" || entry.level === "warning") {
            consoleMessages.push(`${entry.level}: ${entry.text}`);
          }
        }
        if (message.method === "Runtime.exceptionThrown") {
          const detail = message.params.exceptionDetails;
          consoleMessages.push(`exception: ${detail.text} ${detail.exception?.description ?? ""}`);
        }
        if (message.method === "Network.loadingFailed") {
          // Canceled requests are normal navigation churn; failures are not.
          if (message.params.errorText !== "net::ERR_ABORTED") {
            consoleMessages.push(`network: ${message.params.errorText} (${message.params.type})`);
          }
        }
      };
      cdp.listeners.push(onConsole);
      await send("Network.setCookie", {
        name: "asterwaf_session",
        value: session,
        domain: new URL(baseUrl).hostname,
        path: "/",
        httpOnly: true,
      });

      // 主题在模块加载时读取 localStorage，因此先落到同源页面再写入。
      const warmup = cdp.waitFor(
        (message) => message.sessionId === sessionId && message.method === "Page.loadEventFired",
      );
      await send("Page.navigate", { url: `${baseUrl}/login` });
      await warmup;

      for (const page of pages) {
        for (const theme of themes) {
          for (const width of widths) {
            const name = `${page.name}-${theme}-${width}`;
            consoleMessages.length = 0;
            await send("Emulation.setDeviceMetricsOverride", {
              width,
              height: 900,
              deviceScaleFactor: 1,
              mobile: false,
            });
            await send("Runtime.evaluate", {
              expression: `localStorage.setItem("asterwaf-theme", ${JSON.stringify(theme)}); true`,
            });
            const loaded = cdp.waitFor(
              (message) =>
                message.sessionId === sessionId && message.method === "Page.loadEventFired",
            );
            await send("Page.navigate", { url: baseUrl + page.route });
            await loaded;
            await new Promise((resolve) => setTimeout(resolve, 1600));

            const metrics = await send("Runtime.evaluate", {
              expression: `(() => {
                const root = document.documentElement;
                const wide = [...document.querySelectorAll("body *")]
                  .map((el) => {
                    const r = el.getBoundingClientRect();
                    return {
                      name: el.tagName.toLowerCase() + "." + (el.className || "").toString().slice(0, 30),
                      width: Math.round(r.width),
                      right: Math.round(r.right),
                      scrollWidth: el.scrollWidth,
                      overflowX: getComputedStyle(el).overflowX,
                      scrollable: el.closest(".table-scroll, .drawer, .select-list, [data-inner-scroll]") !== null,
                    };
                  })
                  .filter((item) => item.width > 0 && !item.scrollable && item.right > root.clientWidth + 1)
                  .sort((a, b) => b.right - a.right)
                  .slice(0, 8);
                return JSON.stringify({
                  scrollWidth: root.scrollWidth,
                  clientWidth: root.clientWidth,
                  theme: document.documentElement.dataset.theme,
                  title: document.querySelector("h1")?.textContent ?? "",
                  wide,
                });
              })()`,
              returnByValue: true,
            });
            const info = JSON.parse(metrics.result.value);
            if (info.scrollWidth > info.clientWidth + 1) {
              problems.push(
                `${name}: horizontal overflow scrollWidth=${info.scrollWidth} clientWidth=${info.clientWidth} ${JSON.stringify(info.wide)}`,
              );
            }
            if (info.theme !== theme) {
              problems.push(`${name}: theme did not apply (got ${info.theme})`);
            }
            for (const line of consoleMessages) {
              problems.push(`${name}: console ${line}`);
            }
            const shot = await send("Page.captureScreenshot", {
              format: "png",
              captureBeyondViewport: true,
            });
            await writeFile(path.join(outDir, `${name}.png`), Buffer.from(shot.data, "base64"));
            console.log(`captured ${name}.png (${info.title || "no h1"})`);
          }
        }
      }
      cdp.listeners = cdp.listeners.filter((item) => item !== onConsole);
      cdp.close();
    } finally {
      child.kill("SIGTERM");
      await new Promise((resolve) => {
        if (child.exitCode !== null) {
          resolve();
          return;
        }
        const timer = setTimeout(resolve, 3000);
        child.once("exit", () => {
          clearTimeout(timer);
          resolve();
        });
      });
      for (let attempt = 0; attempt < 5; attempt += 1) {
        try {
          await rm(profile, { recursive: true, force: true });
          break;
        } catch {
          await new Promise((resolve) => setTimeout(resolve, 200));
        }
      }
    }
  } finally {
    server.kill("SIGTERM");
  }
  await rm(configPath, { force: true });
  if (problems.length > 0) {
    console.error("screenshot checks failed:");
    for (const problem of problems) {
      console.error(`  - ${problem}`);
    }
    process.exitCode = 1;
    return;
  }
  console.log(`screenshots written to ${path.relative(root, outDir)}`);
}

main().catch((error) => {
  console.error(`screenshots: ${error.message}`);
  process.exit(1);
});
