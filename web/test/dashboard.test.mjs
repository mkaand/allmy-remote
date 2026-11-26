import { test } from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { spawn } from "node:child_process";
import { once } from "node:events";
import { chromium } from "playwright";

test("dashboard authentication, themes, enrollment and revocation", { timeout: 60000 }, async () => {
  const binary = process.env.ALLMY_TEST_BINARY;
  assert.ok(binary, "ALLMY_TEST_BINARY required");
  const dir = await mkdtemp(join(tmpdir(), "allmy-ui-"));
  const origin = "http://127.0.0.1:18888";
  const env = { ...process.env, ALLMY_DATA_DIR: join(dir, "data"), ALLMY_LISTEN: "127.0.0.1:18888", ALLMY_ORIGIN: origin };
  const password = "test-only-password-long";
  let server, browser;
  try {
    const init = spawn(binary, ["init-admin"], { env, stdio: ["pipe", "ignore", "ignore"] });
    const initialized = once(init, "exit");
    init.stdin.end(password + "\n");
    assert.equal((await initialized)[0], 0);
    server = spawn(binary, [], { env, stdio: "ignore" });
    let ready = false;
    for (let i = 0; i < 100; i++) {
      try { if ((await fetch(origin + "/ready")).ok) { ready = true; break; } } catch {}
      await new Promise(resolve => setTimeout(resolve, 100));
    }
    assert.ok(ready, "server did not start");
    browser = await chromium.launch({ headless: true });
    const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
    const errors = [];
    page.on("pageerror", error => errors.push(error.message));
    await page.goto(origin);
    await page.getByRole("heading", { name: "Sign in", exact: true }).waitFor();
    await page.getByLabel("Password", { exact: true }).fill(password);
    await page.getByRole("button", { name: "Sign in", exact: true }).click();
    await page.getByRole("heading", { name: "Dashboard", exact: true }).waitFor();
    await page.getByText("No devices enrolled", { exact: true }).waitFor();
    await page.emulateMedia({ colorScheme: "light" });
    await page.waitForFunction(() => document.documentElement.dataset.theme === "light" && document.documentElement.dataset.themeMode === "system");
    await page.getByRole("switch", { name: "Switch to dark theme" }).click();
    await page.waitForFunction(() => document.documentElement.dataset.theme === "dark" && document.documentElement.dataset.themeMode === "dark");
    await page.reload();
    await page.getByRole("heading", { name: "Dashboard", exact: true }).waitFor();
    await page.waitForFunction(() => document.documentElement.dataset.theme === "dark");
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
    await page.getByLabel("Device name", { exact: true }).fill("Mock acceptance device");
    await page.getByRole("button", { name: "Generate grant" }).click();
    const grantInput = page.getByLabel("Enrollment grant", { exact: true });
    await grantInput.waitFor();
    const grant = await grantInput.inputValue();
    assert.equal(grant.length, 43);
    const enrolled = await fetch(origin + "/api/v1/enrollment/exchange", {
      method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ grant })
    });
    assert.equal(enrolled.status, 201);
    await enrolled.json();
    await page.getByRole("button", { name: "Hide grant" }).click();
    await page.getByRole("button", { name: "Refresh", exact: true }).click();
    const card = page.getByRole("article", { name: "Mock acceptance device" });
    await card.waitFor();
    await card.getByText("offline", { exact: true }).waitFor();
    assert.equal(await card.getByRole("button", { name: "Desktop" }).isDisabled(), true);
    await card.getByRole("button", { name: "Revoke device" }).click();
    await card.getByRole("button", { name: "Confirm revoke" }).click();
    await card.getByText("revoked", { exact: true }).waitFor();
    assert.ok(await page.evaluate(() => Object.keys(localStorage).every(key => key === "allmy-theme")));
    await page.getByRole("button", { name: "Sign out" }).click();
    await page.getByRole("heading", { name: "Sign in", exact: true }).waitFor();
    assert.equal(await page.getByRole("article", { name: "Mock acceptance device" }).count(), 0);
    const status = await page.evaluate(async () => (await fetch("/api/v1/devices")).status);
    assert.equal(status, 401);
    assert.deepEqual(errors, []);
  } finally {
    if (browser) await browser.close();
    if (server && server.exitCode === null) {
      const exited = once(server, "exit");
      server.kill("SIGTERM");
      await exited;
    }
    await rm(dir, { recursive: true, force: true });
  }
});
