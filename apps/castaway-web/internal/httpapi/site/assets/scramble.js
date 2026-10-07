// Spell It Out: tap a tray tile to put it in the next empty slot, tap a slot to send its tile back. Sort and
// Shuffle only reorder the tray. Check sends the arrangement; the server only says right or wrong. The
// arrangement is kept in localStorage so a reload doesn't lose it.
const form = document.querySelector("form.scramble");
if (form) {
  const slots = [...form.querySelectorAll(".scramble-slot")];
  const tray = form.querySelector(".scramble-tray");
  const tiles = [...tray.querySelectorAll(".scramble-tile")];
  const status = form.querySelector(".scramble-status");
  const key = `scramble:${form.action}`;
  let placed = JSON.parse(localStorage.getItem(key) || "null") || slots.map(() => null); // slot → tile index

  const render = () => {
    slots.forEach((slot, i) => {
      slot.textContent = placed[i] === null ? "" : tiles[placed[i]].textContent;
      slot.classList.toggle("filled", placed[i] !== null);
    });
    tiles.forEach((tile, i) => tile.classList.toggle("used", placed.includes(i)));
    localStorage.setItem(key, JSON.stringify(placed));
  };
  if (placed.length !== slots.length || placed.some((t) => t !== null && !tiles[t])) placed = slots.map(() => null);

  tray.addEventListener("click", (event) => {
    const tile = event.target.closest(".scramble-tile");
    const i = tiles.indexOf(tile);
    const empty = placed.indexOf(null);
    if (i < 0 || placed.includes(i) || empty < 0) return;
    placed[empty] = i;
    status.textContent = "";
    render();
  });
  form.querySelector(".scramble-board").addEventListener("click", (event) => {
    const i = slots.indexOf(event.target.closest(".scramble-slot"));
    if (i >= 0 && placed[i] !== null) {
      placed[i] = null;
      render();
    }
  });
  form.querySelector(".scramble-tools").addEventListener("click", (event) => {
    const action = event.target.dataset.action;
    if (action === "clear") placed = slots.map(() => null);
    if (action === "sort") tiles.slice().sort((a, b) => a.textContent.localeCompare(b.textContent)).forEach((t) => tray.append(t));
    if (action === "shuffle") {
      const order = tiles.slice();
      for (let i = order.length - 1; i > 0; i--) {
        const j = Math.floor(Math.random() * (i + 1));
        [order[i], order[j]] = [order[j], order[i]];
      }
      order.forEach((t) => tray.append(t));
    }
    if (action) render();
  });

  const misses = ["Not quite. The tribe waits.", "Not it. Dig deeper.", "No fire yet.", "Keep working."];
  form.addEventListener("submit", (event) => {
    event.preventDefault();
    if (placed.includes(null)) {
      status.textContent = "Fill every slot first.";
      return;
    }
    const guess = placed.map((t) => tiles[t].textContent).join("");
    fetch(form.action, { method: "POST", credentials: "same-origin", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ guess }) })
      .then((response) => response.json().then((body) => ({ ok: response.ok, body })))
      .then(({ ok, body }) => {
        if (!ok) {
          status.textContent = body.error || "Something went wrong. Try again.";
          return;
        }
        if (body.solved) {
          localStorage.removeItem(key);
          window.location.reload();
          return;
        }
        form.querySelector(".scramble-wrong").textContent = body.wrong_checks;
        status.textContent = misses[(body.wrong_checks - 1) % misses.length];
      });
  });

  const clock = form.querySelector(".scramble-clock");
  const started = Number(form.dataset.started);
  const tick = () => {
    const s = Math.max(0, Math.floor((Date.now() - started) / 1000));
    const h = Math.floor(s / 3600), m = Math.floor(s / 60) % 60, sec = String(s % 60).padStart(2, "0");
    clock.textContent = h ? `${h}:${String(m).padStart(2, "0")}:${sec}` : `${m}:${sec}`;
  };
  tick();
  setInterval(tick, 1000);
  render();
}
