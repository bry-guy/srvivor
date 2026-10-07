{ // own block: page scripts share one global scope
// Spell It Out: tap a tray tile to put it in the next empty slot, tap a slot to send its tile back, or drag
// a tile onto any slot (swapping with what's there) or back to the tray. Sort and Shuffle only reorder the
// tray. Check sends the arrangement; the server only says right or wrong. The
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

  // Dragging: pointer events cover mouse and touch. A press that moves more than a few pixels becomes a
  // drag with a floating copy of the tile; anything shorter stays a tap (handled by the click listeners).
  let drag = null;
  let dragged = false; // swallow the click that follows a drag
  const sourceOf = (el) => {
    const tile = el.closest(".scramble-tile");
    if (tile && !placed.includes(tiles.indexOf(tile))) return { tile: tiles.indexOf(tile), from: null };
    const slot = slots.indexOf(el.closest(".scramble-slot"));
    if (slot >= 0 && placed[slot] !== null) return { tile: placed[slot], from: slot };
    return null;
  };
  form.addEventListener("pointerdown", (event) => {
    const source = sourceOf(event.target);
    if (!source || event.button > 0) return;
    drag = { ...source, x: event.clientX, y: event.clientY, ghost: null, origin: event.target.closest(".scramble-tile, .scramble-slot") };
    drag.origin.setPointerCapture(event.pointerId);
  });
  form.addEventListener("pointermove", (event) => {
    if (!drag) return;
    if (!drag.ghost) {
      if (Math.hypot(event.clientX - drag.x, event.clientY - drag.y) < 6) return;
      drag.ghost = tiles[drag.tile].cloneNode(true);
      drag.ghost.classList.add("scramble-ghost");
      drag.ghost.classList.remove("used");
      document.body.append(drag.ghost);
      drag.origin.classList.add("dragging");
    }
    drag.ghost.style.left = `${event.clientX}px`;
    drag.ghost.style.top = `${event.clientY}px`;
    for (const slot of slots) slot.classList.remove("drop");
    const over = document.elementsFromPoint(event.clientX, event.clientY).find((el) => el.classList.contains("scramble-slot"));
    if (over) over.classList.add("drop");
  });
  const endDrag = (event) => {
    if (!drag) return;
    const { tile, from, ghost, origin } = drag;
    drag = null;
    origin.classList.remove("dragging");
    for (const slot of slots) slot.classList.remove("drop");
    if (!ghost) return; // a tap
    ghost.remove();
    dragged = true;
    setTimeout(() => (dragged = false), 0);
    if (event.type === "pointercancel") return;
    const under = document.elementsFromPoint(event.clientX, event.clientY);
    const target = slots.indexOf(under.find((el) => el.classList.contains("scramble-slot")));
    if (target >= 0) {
      if (from !== null) placed[from] = placed[target]; // swap (or move into an empty slot)
      placed[target] = tile;
    } else if (from !== null && under.some((el) => el === tray || tray.contains(el))) {
      placed[from] = null; // back to the tray
    }
    status.textContent = "";
    render();
  };
  form.addEventListener("pointerup", endDrag);
  form.addEventListener("pointercancel", endDrag);
  form.addEventListener("click", (event) => {
    if (dragged) {
      event.stopPropagation();
      event.preventDefault();
    }
  }, true);

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
}
