// Count presses without reloading (the form still works without script), update tribe totals, and flash
// the server's hint, if any, for ~12 seconds.
const stage = document.querySelector(".button-stage");
if (stage) {
  const count = stage.querySelector(".press-count");
  const hint = stage.querySelector(".button-hint");
  let timer;
  stage.addEventListener("submit", (event) => {
    event.preventDefault();
    fetch(stage.action, { method: "POST", headers: { "X-Press": "1" }, credentials: "same-origin" })
      .then((response) => (response.ok ? response.json() : null))
      .then((body) => {
        if (!body) return;
        count.textContent = `count: ${body.count}`;
        for (const tribe of body.tribes || []) {
          const total = stage.querySelector(`[data-tribe="${CSS.escape(tribe.name)}"] b`);
          if (total) total.textContent = tribe.presses;
        }
        if (body.hint) {
          hint.textContent = body.hint;
          hint.classList.add("shown");
          clearTimeout(timer);
          timer = setTimeout(() => hint.classList.remove("shown"), 12000);
        }
      });
  });
}
