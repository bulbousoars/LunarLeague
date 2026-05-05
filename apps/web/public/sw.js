// Service worker — minimal Workbox-free PWA shell.
//
// Strategy:
//   - Precache the app shell (the SVG icon + manifest) so the install banner
//     surfaces in browsers that require it.
//   - Pass-through fetch (let Next handle routing/caching).
//   - Handle Web Push events: notification + click to deep-link.

const CACHE = "lunarleague-shell-v1";
const SHELL = ["/manifest.webmanifest", "/icon.svg"];

self.addEventListener("install", (event) => {
  event.waitUntil(caches.open(CACHE).then((c) => c.addAll(SHELL)));
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))),
    ),
  );
  self.clients.claim();
});

self.addEventListener("fetch", () => {
  // pass-through; Next handles caching headers
});

self.addEventListener("push", (event) => {
  let data = { title: "Lunar League", body: "Update", url: "/" };
  try {
    if (event.data) data = { ...data, ...event.data.json() };
  } catch {
    // ignore parse errors
  }
  event.waitUntil(
    self.registration.showNotification(data.title, {
      body: data.body,
      icon: "/icon.svg",
      badge: "/icon.svg",
      data: { url: data.url },
    }),
  );
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  const url = event.notification.data?.url ?? "/";
  event.waitUntil(self.clients.openWindow(url));
});
