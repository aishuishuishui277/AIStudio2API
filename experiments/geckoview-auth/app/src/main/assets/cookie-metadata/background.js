"use strict";
const HOST = "cookie_metadata";
const domains = new Set(["google.com", "accounts.google.com", "aistudio.google.com"]);
// Never copy, serialize, log, or send raw Cookie objects. Only the explicit auth action below accesses c.value; metadata stays value-free.
function metadata(c) {
  return {
    name: c.name, domain: c.domain, path: c.path,
    secure: c.secure, httpOnly: c.httpOnly, sameSite: c.sameSite,
    session: c.session, hostOnly: c.hostOnly,
    hasExpiration: typeof c.expirationDate === "number" && Number.isFinite(c.expirationDate),
    hasPartitionKey: !!c.partitionKey && Object.keys(c.partitionKey).length > 0,
    hasFirstPartyDomain: typeof c.firstPartyDomain === "string" && c.firstPartyDomain.length > 0
  };
}
let attempts = 0;
let busy = false;
function connect() {
  if (++attempts > 30) return; // Bounded startup/reconnect; never read cookies on a timer.
  const port = browser.runtime.connectNative(HOST);
  port.onDisconnect.addListener(() => { setTimeout(connect, 500); });
  port.onMessage.addListener(async request => {
    if (busy || !["inspect", "auth"].includes(request?.action) || !Number.isSafeInteger(request.id)) return;
    busy = true;
    let message;
    try {
      if (typeof browser.cookies?.getAll !== "function") throw new TypeError();
      // Empty partitionKey includes both partitioned and unpartitioned storage.
      // Null firstPartyDomain includes all first-party isolation contexts.
      await browser.runtime.sendNativeMessage(HOST, {type:"cookieReadAudit", id:request.id});
      const raw = await browser.cookies.getAll({domain: "google.com", partitionKey: {}, firstPartyDomain: null});
      const scoped = raw.filter(c => domains.has(c.domain.replace(/^\./, "")));
      if (request.action === "auth") {
        // This branch runs only after the native user-click command.
        const cookies = scoped.map(c => {
          if ((c.partitionKey && Object.keys(c.partitionKey).length) || c.firstPartyDomain) throw new Error("PARTITION_UNSUPPORTED");
          const sameSite = {no_restriction:"None", lax:"Lax", strict:"Strict", unspecified:""}[c.sameSite];
          if (sameSite === undefined) throw new Error("COOKIE_SAMESITE_INVALID");
          if (!c.session && (!Number.isFinite(c.expirationDate) || c.expirationDate <= Date.now()/1000)) throw new Error("COOKIE_EXPIRED");
          if (c.hostOnly && c.domain.startsWith(".")) throw new Error("COOKIE_HOST_ONLY_CONFLICT");
          return {name:c.name,value:c.value,domain:c.domain,path:c.path,secure:c.secure,httpOnly:c.httpOnly,
            hostOnly:c.hostOnly,sameSite,session:c.session,expires:c.session ? -1 : c.expirationDate};
        });
        message = {type:"authState",id:request.id,state:{cookies}};
      } else {
        const cookies = scoped.map(metadata);
        message = {type: "metadata", id: request.id, count: cookies.length, cookies};
      }
    } catch (error) {
      // Fixed enum only: exception text could contain private browser data.
      message = {type: "error", id: request.id, code: ["PARTITION_UNSUPPORTED","COOKIE_SAMESITE_INVALID","COOKIE_EXPIRED","COOKIE_HOST_ONLY_CONFLICT"].includes(error?.message) ? error.message : (typeof browser.cookies?.getAll === "function" ? "COOKIE_QUERY_FAILED" : "COOKIE_API_UNAVAILABLE")};
    }
    try { await browser.runtime.sendNativeMessage(HOST, message); } catch (_) { /* UI request timeout reports delivery failure */ }
    busy = false;
  });
}
connect();
