// Bean browser sidecar: newline-delimited JSON RPC over stdio.
// Request:  {"id":N,"method":"<name>","params":{...}}
// Response: {"id":N,"result":{...}} | {"id":N,"error":{"code":"...","message":"..."}}
// One process hosts one browser session (context/cookies/tabs persist for
// the process lifetime). Protocol methods mirror internal/browserapi.
import { chromium } from "playwright";
import { mkdtempSync, readFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

let browser = null;
let context = null;
let page = null;
let generation = "";
let launchOptions = { headless: true };
let tracing = false;
// Host-configured egress boundary: non-empty = only requests whose host is
// an allowed domain or a subdomain of one are routed; everything else is
// aborted client-side and reported as a request_blocked event.
let allowedDomains = [];

const MAX_NODES = 4096;
const MAX_NAME = 512;
const MAX_VALUE = 4096;
const MAX_ATTRS = 16;
const MAX_EXTRACT = 65536;
const MAX_EVENT_VALUE = 4096;

function newGeneration() {
  return "snap-" + Math.random().toString(36).slice(2, 12);
}

// Page observations flow to the host as unsolicited {"event":...} lines;
// responses always carry the request's `id`, so the reader can demultiplex.
function pushEvent(kind, data) {
  write({ event: { kind, time: new Date().toISOString(), data } });
}

const clip = (value) => String(value ?? "").slice(0, MAX_EVENT_VALUE);

async function ensurePage() {
  if (page && !page.isClosed()) {
    return page;
  }
  if (!browser) {
    browser = await chromium.launch(launchOptions);
  }
  if (!context) {
    context = await browser.newContext();
    if (allowedDomains.length) {
      await context.route("**/*", (route) => {
        const host = new URL(route.request().url()).hostname.toLowerCase();
        if (allowedDomains.some((domain) => host === domain || host.endsWith("." + domain))) {
          route.continue();
          return;
        }
        pushEvent("request_blocked", { url: clip(route.request().url()), host });
        route.abort("blockedbyclient");
      });
    }
    context.on("console", (message) =>
      pushEvent("console", { type: message.type(), text: clip(message.text()) }),
    );
    context.on("pageerror", (error) => pushEvent("page_error", { message: clip(error) }));
    context.on("request", (request) =>
      pushEvent("request", { method: request.method(), url: clip(request.url()), resource_type: request.resourceType() }),
    );
    context.on("response", (response) =>
      pushEvent("response", { status: response.status(), url: clip(response.url()) }),
    );
    context.on("requestfailed", (request) =>
      pushEvent("request_failed", { method: request.method(), url: clip(request.url()), error: clip(request.failure()?.errorText) }),
    );
    await context.tracing.start({ snapshots: true });
    tracing = true;
  }
  page = await context.newPage();
  page.setDefaultTimeout(30_000);
  return page;
}

function asError(code, error) {
  const message = error instanceof Error ? error.message : String(error);
  if (/timed? ?out|Timeout/.test(message) && code === "internal") {
    return { code: "timeout", message };
  }
  return { code, message };
}

function invalid(message) {
  return Object.assign(new Error(message), { code: "invalid" });
}

function unknownRef(id) {
  return Object.assign(new Error(`ref ${id} not present in snapshot ${generation}`), { code: "unknown_ref" });
}

function refLocator(params) {
  if (!generation) {
    throw Object.assign(new Error("no snapshot taken"), { code: "stale_ref" });
  }
  if (params.snapshot !== generation) {
    throw Object.assign(
      new Error(`ref minted by ${params.snapshot || "no snapshot"}, current is ${generation}`),
      { code: "stale_ref" },
    );
  }
  return page.locator(`[data-bean-ref="${params.id}"]`);
}

const SNAPSHOT_SCRIPT = `(() => {
  const MAX_NODES = ${MAX_NODES}, MAX_NAME = ${MAX_NAME}, MAX_VALUE = ${MAX_VALUE}, MAX_ATTRS = ${MAX_ATTRS};
  for (const el of document.querySelectorAll("[data-bean-ref]")) {
    el.removeAttribute("data-bean-ref");
  }
  const TAG_ROLES = { A: "link", BUTTON: "button", SELECT: "combobox", TEXTAREA: "textbox", OPTION: "option", SUMMARY: "button", DATALIST: "listbox" };
  const INPUT_ROLES = { checkbox: "checkbox", radio: "radio", range: "slider", search: "searchbox", number: "spinbutton", file: "button", submit: "button", reset: "button", button: "button", image: "button" };
  const LANDMARK = { MAIN: "main", NAV: "navigation", ASIDE: "complementary", HEADER: "banner", FOOTER: "contentinfo", FORM: "form", SECTION: "region", UL: "list", OL: "list", LI: "listitem", TABLE: "table", TR: "row", TH: "columnheader", TD: "cell", FIGURE: "figure", IMG: "img", VIDEO: "video", AUDIO: "audio", DIALOG: "dialog", DETAILS: "group", P: "paragraph" };
  const roleOf = (el) => {
    const explicit = el.getAttribute("role");
    if (explicit) return explicit.trim().split(/\\s+/)[0];
    if (el.tagName === "INPUT") return INPUT_ROLES[(el.getAttribute("type") || "text").toLowerCase()] || "textbox";
    const heading = /^H([1-6])$/.exec(el.tagName);
    if (heading) return "heading";
    return TAG_ROLES[el.tagName] || LANDMARK[el.tagName] || null;
  };
  const visible = (el) => {
    if (el.getClientRects().length === 0) return false;
    const style = getComputedStyle(el);
    return style.visibility !== "hidden" && style.display !== "none";
  };
  const nameOf = (el) => {
    const labelledby = el.getAttribute("aria-labelledby");
    if (labelledby) {
      const text = labelledby.split(/\\s+/).map((id) => document.getElementById(id)).filter(Boolean).map((n) => n.innerText || n.textContent || "").join(" ").trim();
      if (text) return text;
    }
    const aria = el.getAttribute("aria-label");
    if (aria) return aria;
    if (el.labels && el.labels.length) return Array.from(el.labels).map((n) => n.innerText || n.textContent || "").join(" ").trim();
    const alt = el.getAttribute("alt");
    if (alt) return alt;
    const placeholder = el.getAttribute("placeholder");
    if (placeholder) return placeholder;
    const title = el.getAttribute("title");
    if (title) return title;
    return (el.innerText || el.textContent || "").trim().replace(/\\s+/g, " ");
  };
  const nodes = [];
  const candidates = document.querySelectorAll("a[href], button, input, select, textarea, option, summary, [role], [aria-label], [aria-labelledby], [contenteditable=''], [contenteditable='true'], h1, h2, h3, h4, h5, h6, main, nav, aside, header, footer, form, label, img[alt], dialog");
  for (const el of candidates) {
    if (!visible(el)) continue;
    const role = roleOf(el);
    if (!role) continue;
    let depth = 0;
    for (let n = el.parentElement; n; n = n.parentElement) depth++;
    const node = { role, name: nameOf(el).slice(0, MAX_NAME), depth, attrs: {} };
    if (el.disabled ?? el.getAttribute("aria-disabled") === "true") node.disabled = true;
    if (el.tagName === "INPUT" && (el.type === "checkbox" || el.type === "radio")) node.checked = !!el.checked;
    if (el.tagName === "INPUT" || el.tagName === "TEXTAREA" || el.tagName === "SELECT") node.value = String(el.value ?? "").slice(0, MAX_VALUE);
    for (const key of ["id", "name", "type", "href", "placeholder", "data-testid"]) {
      const value = el.getAttribute(key);
      if (value) {
        node.attrs[key] = String(value).slice(0, 512);
        if (Object.keys(node.attrs).length >= MAX_ATTRS) break;
      }
    }
    el.setAttribute("data-bean-ref", "e" + (nodes.length + 1));
    nodes.push(node);
    if (nodes.length >= MAX_NODES) break;
  }
  return { url: location.href, title: document.title, nodes };
})()`;

const handlers = {
  async ping() {
    return { ok: true };
  },
  async configure(params) {
    allowedDomains = (params.allowed_domains || [])
      .map((domain) => String(domain).trim().toLowerCase())
      .filter(Boolean);
    if (context) {
      throw invalid("configure must precede the first page");
    }
    return { ok: true };
  },
  async open(params) {
    const p = await ensurePage();
    if (!params.url || params.url.length > 65536) {
      throw invalid("open requires a bounded url");
    }
    const before = p.url();
    await p.goto(params.url, { waitUntil: "load" });
    generation = "";
    return { url: p.url(), navigated: p.url() !== before };
  },
  async snapshot() {
    const p = await ensurePage();
    const raw = await p.evaluate(SNAPSHOT_SCRIPT);
    generation = newGeneration();
    return { id: generation, url: raw.url, title: raw.title, nodes: raw.nodes };
  },
  async click(params) {
    const p = await ensurePage();
    const locator = refLocator(params);
    if ((await locator.count()) === 0) {
      throw unknownRef(params.id);
    }
    const before = p.url();
    await Promise.all([
      p.waitForLoadState("load", { timeout: 5_000 }).catch(() => {}),
      locator.first().click(),
    ]);
    generation = "";
    return { url: p.url(), navigated: p.url() !== before };
  },
  async fill(params) {
    const p = await ensurePage();
    const locator = refLocator(params);
    if ((await locator.count()) === 0) {
      throw unknownRef(params.id);
    }
    await locator.first().fill(String(params.text ?? ""));
    return { url: p.url() };
  },
  async select(params) {
    const p = await ensurePage();
    const locator = refLocator(params);
    if ((await locator.count()) === 0) {
      throw unknownRef(params.id);
    }
    await locator.first().selectOption(String(params.value ?? ""));
    return { url: p.url() };
  },
  async press(params) {
    const p = await ensurePage();
    const before = p.url();
    await Promise.all([
      p.waitForLoadState("load", { timeout: 5_000 }).catch(() => {}),
      p.keyboard.press(String(params.key ?? "")),
    ]);
    generation = "";
    return { url: p.url(), navigated: p.url() !== before };
  },
  async wait(params) {
    const p = await ensurePage();
    const timeout = params.timeout_millis > 0 ? params.timeout_millis : 30_000;
    const deadline = (work) =>
      Promise.race([
        work,
        new Promise((_, reject) =>
          setTimeout(
            () => reject(Object.assign(new Error("condition not met before timeout"), { code: "timeout" })),
            timeout,
          ),
        ),
      ]);
    switch (params.kind) {
      case "navigation":
        await deadline(p.waitForLoadState("load", { timeout }));
        break;
      case "network_idle":
        await deadline(p.waitForLoadState("networkidle", { timeout }));
        break;
      case "ref_visible": {
        const locator = refLocator(params);
        await deadline(locator.first().waitFor({ state: "visible", timeout }));
        break;
      }
      case "ref_hidden": {
        const locator = refLocator(params);
        await deadline(locator.first().waitFor({ state: "hidden", timeout }));
        break;
      }
      case "text_present":
        await deadline(p.waitForFunction(
          (text) => (document.body?.innerText || "").includes(text),
          String(params.text ?? ""),
          { timeout },
        ));
        break;
      case "url_equals":
        await deadline(p.waitForFunction(
          (text) => location.href === text,
          String(params.text ?? ""),
          { timeout },
        ));
        break;
      case "url_contains":
        await deadline(p.waitForFunction(
          (text) => location.href.includes(text),
          String(params.text ?? ""),
          { timeout },
        ));
        break;
      default:
        throw invalid(`invalid condition kind ${params.kind}`);
    }
    return { met: true, url: p.url() };
  },
  async extract(params) {
    const locator = refLocator(params);
    if ((await locator.count()) === 0) {
      throw unknownRef(params.id);
    }
    const el = locator.first();
    let value;
    switch (params.as) {
      case "text":
        value = await el.innerText();
        break;
      case "value":
        value = await el.inputValue().catch(() => el.getAttribute("value"));
        break;
      case "attribute":
        if (!params.attribute) {
          throw invalid("attribute extraction requires attribute name");
        }
        value = (await el.getAttribute(params.attribute)) ?? "";
        break;
      default:
        throw invalid(`invalid extraction kind ${params.as}`);
    }
    return { value: String(value ?? "").slice(0, MAX_EXTRACT) };
  },
  async screenshot() {
    const p = await ensurePage();
    const bytes = await p.screenshot({ type: "png" });
    return { content_type: "image/png", bytes_base64: bytes.toString("base64") };
  },
  async trace() {
    if (!context) {
      throw invalid("no session");
    }
    if (!tracing) {
      throw invalid("trace already captured");
    }
    const dir = mkdtempSync(join(tmpdir(), "bean-trace-"));
    const path = join(dir, "trace.zip");
    await context.tracing.stop({ path });
    tracing = false;
    return { content_type: "application/zip", bytes_base64: readFileSync(path).toString("base64") };
  },
  async close() {
    try {
      if (tracing) {
        await context?.tracing.stop().catch(() => {});
        tracing = false;
      }
      await context?.close();
      await browser?.close();
    } finally {
      context = null;
      browser = null;
      page = null;
      generation = "";
    }
    return {};
  },
};

let inputBuffer = "";

function write(message) {
  process.stdout.write(JSON.stringify(message) + "\n");
}

async function dispatch(request) {
  const handler = handlers[request.method];
  if (!handler) {
    write({ id: request.id ?? null, error: { code: "invalid", message: `unknown method ${request.method}` } });
    return;
  }
  try {
    const result = await handler(request.params || {});
    write({ id: request.id, result });
  } catch (error) {
    write({ id: request.id, error: asError(error.code || "internal", error) });
  }
}

// Requests run strictly in arrival order: a snapshot never races a navigation.
let queue = Promise.resolve();

process.stdin.on("data", (chunk) => {
  inputBuffer += chunk;
  for (;;) {
    const newline = inputBuffer.indexOf("\n");
    if (newline < 0) {
      break;
    }
    const line = inputBuffer.slice(0, newline);
    inputBuffer = inputBuffer.slice(newline + 1);
    if (!line.trim()) {
      continue;
    }
    let request;
    try {
      request = JSON.parse(line);
    } catch {
      write({ id: null, error: { code: "invalid", message: "malformed request" } });
      continue;
    }
    queue = queue.then(() => dispatch(request));
  }
});

process.stdin.on("end", async () => {
  await handlers.close();
  process.exit(0);
});

process.on("uncaughtException", async (error) => {
  write({ id: null, error: { code: "crash", message: String(error?.message || error) } });
  await handlers.close().catch(() => {});
  process.exit(1);
});
