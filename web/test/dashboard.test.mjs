import { test } from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { spawn, execFileSync } from "node:child_process";
import { createServer as createHTTPS } from "node:https";
import { request as requestHTTP } from "node:http";
import { readFile } from "node:fs/promises";
import { once } from "node:events";
import { chromium, webkit } from "playwright";

for (const [engine, browserType] of [["Chromium", chromium], ["WebKit", webkit]]) {
test(engine + ": dashboard authentication, themes, enrollment and revocation", { timeout: 90000 }, async () => {
  const binary = process.env.ALLMY_TEST_BINARY;
  assert.ok(binary, "ALLMY_TEST_BINARY required");
  const dir = await mkdtemp(join(tmpdir(), "allmy-ui-"));
  const backend = "http://127.0.0.1:18888";
  const origin = "https://127.0.0.1:18889";
  const env = { ...process.env, ALLMY_DATA_DIR: join(dir, "data"), ALLMY_LISTEN: "127.0.0.1:18888", ALLMY_ORIGIN: origin };
  const password = "test-only-password-long";
  let server, browser, proxy;
  try {
    const init = spawn(binary, ["init-admin"], { env, stdio: ["pipe", "ignore", "ignore"] });
    const initialized = once(init, "exit");
    init.stdin.end(password + "\n");
    assert.equal((await initialized)[0], 0);
    server = spawn(binary, [], { env, stdio: "ignore" });
    let ready = false;
    for (let i = 0; i < 100; i++) {
      try { if ((await fetch(backend + "/ready")).ok) { ready = true; break; } } catch {}
      await new Promise(resolve => setTimeout(resolve, 100));
    }
    assert.ok(ready, "server did not start");
    // Ephemeral loopback HTTPS only for the browser test. Never ship this certificate.
    const key = join(dir, "test.key");
    const cert = join(dir, "test.pem");
    execFileSync("openssl", ["req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "1",
      "-subj", "/CN=localhost", "-keyout", key, "-out", cert], { stdio: "ignore" });
    proxy = createHTTPS({ key: await readFile(key), cert: await readFile(cert) }, (req, res) => {
      const upstream = requestHTTP(backend + req.url, { method: req.method, headers: req.headers }, response => {
        res.writeHead(response.statusCode, response.headers);
        response.pipe(res);
      });
      upstream.on("error", () => { if (!res.headersSent) res.writeHead(502); res.end(); });
      req.pipe(upstream);
    });
    proxy.listen(18889, "127.0.0.1");
    await once(proxy, "listening");
    browser = await browserType.launch({ headless: true });
    const page = await browser.newPage({ viewport: { width: 1280, height: 800 }, ignoreHTTPSErrors: true });
    const errors = [];
    page.on("pageerror", error => errors.push(error.message));
    await page.goto(origin);
    await page.getByRole("heading", { name: "Sign in", exact: true }).waitFor();
    const logo = page.getByRole("img", { name: "AllMy Remote", exact: true });
    await logo.waitFor();
    await page.waitForFunction(() => { const img = document.querySelector(".brand-logo"); return img?.complete && img.naturalWidth === 480; });
    for (const [rel, path] of [["icon", "/favicon.png"], ["apple-touch-icon", "/apple-touch-icon.png"]]) {
      assert.equal(await page.locator(`link[rel="${rel}"]`).getAttribute("href"), path);
      const response = await page.request.get(origin + path);
      assert.equal(response.status(), 200);
      assert.equal(response.headers()["content-type"], "image/png");
    }
    await page.getByLabel("Password", { exact: true }).fill(password);
    await page.getByRole("button", { name: "Sign in", exact: true }).click();
    await page.getByRole("heading", { name: "Dashboard", exact: true }).waitFor();
    assert.equal(await logo.isVisible(), true);
    await page.getByText("No devices enrolled", { exact: true }).waitFor();
    await page.emulateMedia({ colorScheme: "light" });
    await page.waitForFunction(() => document.documentElement.dataset.theme === "light" && document.documentElement.dataset.themeMode === "system");
    await page.getByRole("switch", { name: "Switch to dark theme" }).click();
    await page.waitForFunction(() => document.documentElement.dataset.theme === "dark" && document.documentElement.dataset.themeMode === "dark");
    await page.reload();
    await page.getByRole("heading", { name: "Dashboard", exact: true }).waitFor();
    await page.waitForFunction(() => document.documentElement.dataset.theme === "dark");
    await page.waitForFunction(() => { const img = document.querySelector(".brand-logo"); return img?.complete && img.naturalWidth === 480; });
    await page.waitForFunction(() => { const img = document.querySelector(".brand-logo"); return img?.complete && img.naturalWidth === 480 && img.getAttribute("src") === "/logo-dark.png"; });
    assert.equal(await logo.evaluate(img => getComputedStyle(img).backgroundColor), "rgba(0, 0, 0, 0)");
    await page.getByRole("switch", { name: "Switch to light theme" }).click();
    await page.waitForFunction(() => document.documentElement.dataset.theme === "light");
    await page.emulateMedia({ colorScheme: "dark" });
    assert.equal(await page.evaluate(() => document.documentElement.dataset.theme), "light");
    await page.getByRole("button", { name: "Use automatic system theme" }).click();
    await page.waitForFunction(() => document.documentElement.dataset.theme === "dark" && document.documentElement.dataset.themeMode === "system");
    await page.emulateMedia({ colorScheme: "light" });
    await page.waitForFunction(() => document.documentElement.dataset.theme === "light");
    assert.equal(await page.evaluate(() => localStorage.getItem("allmy-theme")), null);
    await page.evaluate(() => localStorage.setItem("allmy.theme", "dark"));
    await page.reload();
    await page.getByRole("heading", { name: "Dashboard", exact: true }).waitFor();
    await page.waitForFunction(() => document.documentElement.dataset.theme === "dark" && localStorage.getItem("allmy-theme") === "dark" && localStorage.getItem("allmy.theme") === null);
    await page.getByRole("button", { name: "Use automatic system theme" }).click();
    await page.waitForFunction(() => document.documentElement.dataset.theme === "light" && document.documentElement.dataset.themeMode === "system");
    await page.setViewportSize({ width: 390, height: 844 });
    assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), "mobile overflow");
    assert.equal(await logo.isVisible(), true);
    await page.getByLabel("Device name", { exact: true }).fill("Mock acceptance device");
    await page.getByRole("button", { name: "Generate grant" }).click();
    const grantInput = page.getByLabel("Enrollment grant", { exact: true });
    await grantInput.waitFor();
    const grant = await grantInput.inputValue();
    assert.equal(grant.length, 43);
    const enrolled = await fetch(backend + "/api/v1/enrollment/exchange", {
      method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ grant })
    });
    assert.equal(enrolled.status, 201);
    await enrolled.json();
    await page.getByRole("button", { name: "Hide grant" }).click();
    await page.getByRole("button", { name: "Refresh", exact: true }).click();
    let card = page.getByRole("article", { name: "Mock acceptance device" });
    await card.waitFor();
    await card.getByText("offline", { exact: true }).waitFor();
    assert.equal(await card.getByRole("button", { name: "Desktop" }).isDisabled(), true);
    await card.getByRole("button", { name: "Edit device" }).click();
    await card.getByLabel("Device name", { exact: true }).fill("Renamed acceptance device");
    await card.locator(".icon-option").filter({hasText: "Mini PC"}).click();
    assert.equal(await card.getByRole("radio", { name: "Mini PC", exact: true }).isChecked(), true);
    await card.getByRole("button", { name: "Save changes" }).click();
    card = page.getByRole("article", { name: "Renamed acceptance device", exact: true });
    await card.waitFor();
    await card.locator('.device-icon [data-device-icon="mini-pc"]').waitFor();
    await page.reload();
    await card.waitFor();
    await card.locator('.device-icon [data-device-icon="mini-pc"]').waitFor();
    await card.getByRole("button", { name: "Edit device" }).click();
    await card.getByLabel("Device name", { exact: true }).fill("Unsaved name");
    await card.getByRole("button", { name: "Cancel edit" }).click();
    await card.waitFor();
    await card.getByRole("button", { name: "Revoke device" }).click();
    await card.getByRole("button", { name: "Confirm revoke" }).click();
    await card.getByText("revoked", { exact: true }).waitFor();
    await card.getByRole("button", { name: "Delete device", exact: true }).click();
    await card.getByRole("button", { name: "Cancel", exact: true }).click();
    await card.getByText("revoked", { exact: true }).waitFor();
    await card.getByRole("button", { name: "Delete device", exact: true }).click();
    await card.getByRole("button", { name: "Confirm delete", exact: true }).click();
    await card.waitFor({state: "detached"});
    await page.getByText("No devices enrolled", { exact: true }).waitFor();
    assert.ok(await page.evaluate(() => Object.keys(localStorage).every(key => key === "allmy-theme")));
    await page.getByRole("button", { name: "Sign out" }).click();
    await page.getByRole("heading", { name: "Sign in", exact: true }).waitFor();
    assert.equal(await page.getByRole("article", { name: "Renamed acceptance device" }).count(), 0);
    const status = await page.evaluate(async () => (await fetch("/api/v1/devices")).status);
    assert.equal(status, 401);
    assert.deepEqual(errors, []);
  } finally {
    if (browser) await browser.close();
    if (proxy) {
      proxy.closeAllConnections();
      await new Promise(resolve => proxy.close(resolve));
    }
    if (server && server.exitCode === null) {
      const exited = once(server, "exit");
      server.kill("SIGTERM");
      await exited;
    }
    await rm(dir, { recursive: true, force: true });
  }
});
}
