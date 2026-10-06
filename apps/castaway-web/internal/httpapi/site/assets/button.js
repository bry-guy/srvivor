// Count presses without reloading (the form still works without script), update tribe totals, and flash a
// hint for ~12 seconds when the count reaches a threshold.
const stage = document.querySelector(".button-stage");
if (stage) {
  const hints = JSON.parse(stage.dataset.hints || "[]");
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
        const reached = hints.find((h) => h.At === body.count);
        if (reached) {
          hint.textContent = reached.Text;
          hint.classList.add("shown");
          clearTimeout(timer);
          timer = setTimeout(() => hint.classList.remove("shown"), 12000);
        }
      });
  });
}
