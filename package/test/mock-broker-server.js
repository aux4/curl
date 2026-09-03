"use strict";

/*
 * Tiny mock OAuth broker used by oauth-broker-login.test.md.
 *
 * It mimics the three routes aux4/oauth-broker exposes so the curl broker-login
 * and grant:broker refresh paths can be exercised without a real broker. Zero
 * dependencies — Node built-in http only.
 *
 *   GET  /google/authorize-url  returns { url, codeVerifier, state }
 *   POST /google/exchange       returns broker token JSON (code "good" only)
 *   POST /google/refresh        returns rotated tokens (refreshToken "rt-old" only)
 */

const http = require("http");

const PORT = Number(process.env.MOCK_BROKER_PORT || 8799);

const server = http.createServer((req, res) => {
  let body = "";
  req.on("data", (chunk) => (body += chunk));
  req.on("end", () => {
    if (req.method === "GET" && req.url.startsWith("/google/authorize-url")) {
      // Record the state the client generated so a test can drive the loopback
      // callback with the matching state (mimicking the browser redirect).
      const state = new URL(req.url, "http://localhost").searchParams.get("state") || "";
      if (process.env.STATE_FILE) {
        try {
          require("fs").writeFileSync(process.env.STATE_FILE, state);
        } catch (e) {
          /* ignore */
        }
      }
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(
        JSON.stringify({
          url: "https://provider.example/auth?client_id=broker-cid&state=" + encodeURIComponent(state),
          codeVerifier: "cv-123",
          state: state
        })
      );
      return;
    }

    if (req.method === "POST" && req.url === "/google/exchange") {
      const data = JSON.parse(body || "{}");
      if (data.code !== "good") {
        res.writeHead(400, { "Content-Type": "application/json" });
        res.end(JSON.stringify({ error: "invalid_grant" }));
        return;
      }
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(
        JSON.stringify({
          accessToken: "at-1",
          refreshToken: "rt-old",
          idToken: "",
          expiresIn: 3600,
          tokenType: "Bearer",
          principal: { email: "u@example.com", provider: "google" }
        })
      );
      return;
    }

    if (req.method === "POST" && req.url === "/google/refresh") {
      const data = JSON.parse(body || "{}");
      if (data.refreshToken !== "rt-old") {
        res.writeHead(400, { "Content-Type": "application/json" });
        res.end(JSON.stringify({ error: "invalid_grant" }));
        return;
      }
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(
        JSON.stringify({
          accessToken: "at-new",
          refreshToken: "rt-new",
          idToken: "",
          expiresIn: 3600,
          tokenType: "Bearer"
        })
      );
      return;
    }

    res.writeHead(404);
    res.end("not found");
  });
});

server.listen(PORT, () => {
  process.stdout.write(`listening on ${PORT}\n`);
});
