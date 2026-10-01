class CastawordleGame extends HTMLElement {
  connectedCallback() {
    this.draft = '';
    this.pending = false;
    this.keyHandler = event => {
      if (event.ctrlKey || event.metaKey || event.altKey || event.isComposing) return;
      if (['INPUT', 'TEXTAREA', 'SELECT'].includes(event.target.tagName)) return;
      if (document.activeElement !== document.body && !this.contains(document.activeElement)) return;
      if (/^[a-z]$/i.test(event.key) || ['Enter', 'Backspace'].includes(event.key)) {
        event.preventDefault();
        this.press(event.key);
      }
    };
    document.addEventListener('keydown', this.keyHandler);
    this.load();
  }
  disconnectedCallback() { document.removeEventListener('keydown', this.keyHandler); }
  get endpoint() { return `/api/castawordle/${encodeURIComponent(this.getAttribute('game-id'))}/play`; }
  async request(url, options) {
    const response = await fetch(url, { credentials: 'same-origin', ...options });
    const body = await response.json();
    if (!response.ok) {
      const error = new Error(body.error || 'Could not load your game.');
      error.status = response.status;
      throw error;
    }
    return body;
  }
  async load() {
    try {
      this.state = await this.request(this.endpoint);
      this.render();
    } catch (error) {
      this.textContent = '';
      const message = document.createElement('p');
      message.setAttribute('role', 'status');
      message.textContent = error.message || 'Connection lost. Reload to try again.';
      this.append(message);
      if (error.status === 401) {
        const link = document.createElement('a');
        link.href = `/auth/login?next=${encodeURIComponent(location.pathname)}`;
        link.textContent = 'Log in again';
        this.append(link);
      }
    }
  }
  playable() { return ['not_started', 'in_progress'].includes(this.state?.status); }
  press(key) {
    if (this.pending || !this.playable()) return;
    if (key === 'Enter') { this.submit(); return; }
    const previousDraft = this.draft;
    const addingLetter = /^[a-z]$/i.test(key) && this.draft.length < this.state.game.word_length;
    if (key === 'Backspace') this.draft = this.draft.slice(0, -1);
    else if (addingLetter) this.draft += key.toUpperCase();
    if (this.draft !== previousDraft) this.message.textContent = '';
    this.renderBoard();
  }
  async submit() {
    if (this.draft.length !== this.state.game.word_length) {
      this.message.textContent = `Enter ${this.state.game.word_length} letters first.`;
      return;
    }
    this.pending = true;
    this.keyboard.querySelectorAll('button').forEach(button => { button.disabled = true; });
    this.message.textContent = 'Checking your guess…';
    try {
      this.state = await this.request(`${this.endpoint}/guesses`, {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ guess: this.draft, position: this.state.guesses.length + 1 }),
      });
      this.draft = '';
      this.renderBoard();
      this.renderKeyboard();
      this.renderTorches();
      this.setMessage();
    } catch (error) {
      if (error.status === 409) {
        try {
          this.state = await this.request(this.endpoint);
          this.draft = '';
          this.renderBoard();
          this.renderKeyboard();
          this.renderTorches();
        } catch {}
      }
      this.message.textContent = error.message || 'Connection lost. Your turn is safe; try again.';
    } finally {
      this.pending = false;
      this.keyboard.querySelectorAll('button').forEach(button => { button.disabled = !this.playable(); });
    }
  }
  render() {
    this.dataset.length = String(this.state.game.word_length);
    this.innerHTML = '<div class="torches" aria-label="Torches remaining"></div><div class="board" role="grid" tabindex="0" aria-label="Castawordle guesses"></div><p class="game-message" role="status" aria-live="polite"></p><div class="keyboard" aria-label="Letter keyboard"></div>';
    this.board = this.querySelector('.board');
    this.message = this.querySelector('.game-message');
    this.keyboard = this.querySelector('.keyboard');
    this.keyboard.addEventListener('click', event => {
      const button = event.target.closest('button[data-key]');
      if (button) this.press(button.dataset.key);
    });
    this.renderBoard();
    this.renderKeyboard();
    this.renderTorches();
    this.setMessage();
  }
  renderBoard() {
    this.board.replaceChildren();
    for (let row = 0; row < this.state.game.guess_limit; row++) {
      const guessed = this.state.guesses[row];
      const word = guessed?.word || (row === this.state.guesses.length ? this.draft : '');
      const line = document.createElement('div');
      line.className = 'board-row';
      line.setAttribute('role', 'row');
      for (let col = 0; col < this.state.game.word_length; col++) {
        const tile = document.createElement('div');
        const feedback = guessed?.feedback[col];
        tile.className = `tile${word[col] ? ' filled' : ''}${feedback ? ` ${feedback}` : ''}`;
        tile.textContent = word[col] || '';
        tile.setAttribute('role', 'gridcell');
        tile.setAttribute('aria-label', `Row ${row + 1}, letter ${col + 1}: ${word[col] || 'empty'}${feedback ? `, ${feedback}` : ''}`);
        line.append(tile);
      }
      this.board.append(line);
    }
  }
  renderKeyboard() {
    const rank = { absent: 1, present: 2, correct: 3 };
    const colors = {};
    for (const guess of this.state.guesses) {
      for (let i = 0; i < guess.word.length; i++) {
        const letter = guess.word[i], feedback = guess.feedback[i];
        if ((rank[feedback] || 0) > (rank[colors[letter]] || 0)) colors[letter] = feedback;
      }
    }
    this.keyboard.replaceChildren();
    for (const keys of ['QWERTYUIOP'.split(''), 'ASDFGHJKL'.split(''), ['Enter', ...'ZXCVBNM', 'Backspace']]) {
      const row = document.createElement('div');
      row.className = 'key-row';
      for (const key of keys) {
        const button = document.createElement('button');
        button.type = 'button';
        button.dataset.key = key;
        button.className = `key${key.length > 1 ? ' wide' : ''}${colors[key] ? ` ${colors[key]}` : ''}`;
        button.textContent = key === 'Backspace' ? '⌫' : key === 'Enter' ? 'ENTER' : key;
        button.setAttribute('aria-label', `${key}${colors[key] ? `, ${colors[key]}` : ''}`);
        button.disabled = this.pending || !this.playable();
        row.append(button);
      }
      this.keyboard.append(row);
    }
  }
  renderTorches() {
    const wrong = this.state.guesses.filter(guess => !guess.feedback.every(color => color === 'correct')).length;
    const holder = this.querySelector('.torches');
    holder.replaceChildren();
    holder.setAttribute('aria-label', `${this.state.game.guess_limit - wrong} torches remaining`);
    for (let i = 0; i < this.state.game.guess_limit; i++) {
      const torch = document.createElement('span');
      torch.className = `torch${i < wrong ? ' extinguished' : ''}`;
      torch.setAttribute('aria-hidden', 'true');
      holder.append(torch);
    }
  }
  setMessage() {
    const status = this.state.status;
    if (status === 'solved') this.message.textContent = `Torch saved! ${this.state.answer} in ${this.state.guesses.length}/${this.state.game.guess_limit}.`;
    else if (status === 'exhausted') this.message.textContent = `Your torch is out. The word was ${this.state.answer}.`;
    else if (status === 'expired') this.message.textContent = 'This puzzle has closed. Your progress is saved.';
    else if (status === 'not_open') this.message.textContent = `Opens ${new Date(this.state.game.opens_at).toLocaleString()}.`;
    else this.message.textContent = 'Make your next move. Invalid words cost no torch.';
  }
}
customElements.define('castawordle-game', CastawordleGame);
