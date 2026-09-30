const form = document.querySelector('#create-game');
form?.addEventListener('submit', async event => {
  event.preventDefault();
  const button = form.querySelector('button');
  const message = document.querySelector('#create-game-message');
  button.disabled = true;
  message.textContent = 'Creating…';
  try {
    const fields = new FormData(form);
    const response = await fetch(`/api/instances/${encodeURIComponent(form.dataset.instanceId)}/castawordle`, {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, credentials: 'same-origin',
      body: JSON.stringify({ name: fields.get('name'), answer: fields.get('answer') }),
    });
    const body = await response.json();
    if (!response.ok) throw new Error(body.error || 'Could not create the puzzle.');
    form.reset();
    location.assign(`/castawordle/${encodeURIComponent(body.id)}`);
  } catch (error) {
    message.textContent = error.message || 'Connection lost. Try again.';
    button.disabled = false;
  }
});
