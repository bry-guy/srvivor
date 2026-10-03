// Count presses without reloading; the form still works without script.
document.querySelector(".button-stage")?.addEventListener("submit", (event) => {
  event.preventDefault();
  const count = event.currentTarget.querySelector(".press-count");
  fetch(event.currentTarget.action, { method: "POST", headers: { "X-Press": "1" }, credentials: "same-origin" })
    .then((response) => {
      const n = response.headers.get("X-Press-Count");
      if (n && count) count.textContent = `count: ${n}`;
    });
});
