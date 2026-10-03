// Count presses without reloading; the form still works without script.
document.querySelector(".button-stage")?.addEventListener("submit", (event) => {
  event.preventDefault();
  fetch(event.currentTarget.action, { method: "POST", headers: { "X-Press": "1" }, credentials: "same-origin" });
});
