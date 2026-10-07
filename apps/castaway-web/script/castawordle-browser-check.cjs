const assert = require('node:assert/strict');
const { chromium } = require('playwright');
const os = require('node:os');
const path = require('node:path');
const base = process.argv[2];
(async () => {
  const browser = await chromium.launch({ headless: true });
  try {
    for (const mobile of [true, false]) {
      const context = await browser.newContext({ viewport: mobile ? { width: 320, height: 800 } : { width: 1280, height: 900 }, isMobile: mobile, hasTouch: mobile, colorScheme: 'dark' });
      const page = await context.newPage();
      page.on('pageerror', error => { throw new Error(`page error: ${error.message}`); });
      page.on('console', msg => { if (msg.type() === 'error') console.log(`console error: ${msg.text()}`); });
      await page.goto(`${base}/games`);
      await page.waitForSelector('#create-game');
      assert.equal(new URL(page.url()).pathname, '/games');
      const dark = await page.evaluate(() => getComputedStyle(document.body).backgroundColor);
      await page.selectOption('#theme', 'light');
      const light = await page.evaluate(() => getComputedStyle(document.body).backgroundColor);
      assert.notEqual(light, dark);
      await page.locator('input[name="name"]').fill(mobile ? 'Phone trial' : 'Desktop trial');
      await page.locator('input[name="answer"]').fill('SURVIVOR');
      await page.locator('#create-game button').click();
      await page.waitForFunction(() => document.querySelector('castawordle-game')?.state);
      const gameURL = page.url();
      assert.match(new URL(gameURL).pathname, /^\/games\/castawordle\/[0-9a-f-]+$/);
      assert.equal(await page.locator('.board-row').count(), 6);
      assert.equal(await page.locator('.board-row').first().locator('.tile').count(), 8);
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
      const key = value => page.locator(`.key[data-key="${value}"]`).click();
      let postedGuesses = 0;
      page.on('request', request => { if (request.method() === 'POST' && request.url().endsWith('/play/guesses')) postedGuesses++; });
      for (const letter of 'ZZZZZZZ') await key(letter);
      assert.equal(postedGuesses, 0);
      await key('Z');
      assert.equal(postedGuesses, 0, 'a full word must wait for Enter');
      await key('Enter');
      await page.waitForFunction(() => !document.querySelector('castawordle-game').pending);
      assert.equal(await page.evaluate(() => document.querySelector('castawordle-game').state.guesses.length), 0);
      assert.equal(await page.locator('.game-message').innerText(), 'invalid word, try again');
      await key('Backspace');
      assert.equal(await page.locator('.game-message').innerText(), '');
      assert.equal(await page.evaluate(() => document.querySelector('castawordle-game').draft), 'ZZZZZZZ');
      for (let i = 0; i < 7; i++) await key('Backspace');
      await page.route('**/play/guesses', async route => { await route.fetch(); await route.abort(); }, { times: 1 });
      for (const letter of 'CAMPFIRE') await key(letter);
      await key('Enter');
      await page.waitForFunction(() => !document.querySelector('castawordle-game').pending);
      assert.equal(await page.evaluate(() => document.querySelector('castawordle-game').state.guesses.length), 0);
      await key('Enter');
      await page.waitForFunction(() => document.querySelector('castawordle-game').state.guesses.length === 1);
      assert.equal(await page.locator('.torch.extinguished').count(), 1);
      await page.reload();
      await page.waitForFunction(() => document.querySelector('castawordle-game')?.state?.guesses.length === 1);
      assert.equal(await page.locator('#theme').inputValue(), 'light');
      assert.equal(await page.locator('.board-row').first().innerText().then(t => t.replace(/\s/g, '')), 'CAMPFIRE');
      for (const theme of ['light', 'dark']) {
        await page.selectOption('#theme', theme);
        const darker = await page.evaluate(() => {
          const brightness = element => getComputedStyle(element).backgroundColor.match(/\d+/g).slice(0, 3).map(Number).reduce((a, b) => a + b, 0);
          return brightness(document.querySelector('.key.absent')) < brightness(document.querySelector('.key:not(.absent):not(.present):not(.correct)'));
        });
        assert(darker, `${theme}: absent keyboard letters must be darker than unused letters`);
      }
      if (!mobile) await page.selectOption('#theme', 'light');
      const screenshot = `/tmp/castawordle-${mobile ? 'phone' : 'desktop'}.png`;
      await page.screenshot({ path: screenshot, fullPage: true });
      await page.locator('.board').focus();
      await page.keyboard.type('SURVIVO');
      assert.equal(await page.evaluate(() => document.querySelector('castawordle-game').state.guesses.length), 1);
      await page.keyboard.type('R');
      assert.equal(await page.evaluate(() => document.querySelector('castawordle-game').state.guesses.length), 1);
      await page.keyboard.press('Enter');
      await page.waitForFunction(() => document.querySelector('castawordle-game').state.status === 'solved');
      assert.equal(await page.evaluate(() => document.querySelector('castawordle-game').state.guesses.length), 2);
      assert.equal(postedGuesses, 4);
      const cookies = await context.cookies();
      const anonymous = await browser.newContext();
      const fresh = await anonymous.newPage();
      await fresh.goto(gameURL);
      await fresh.waitForFunction(() => document.querySelector('castawordle-game')?.state?.status === 'solved');
      assert.equal(new URL(fresh.url()).pathname, new URL(gameURL).pathname);
      assert(cookies.some(cookie => cookie.name === 'castaway_session' && cookie.httpOnly));
      await anonymous.close();
      await page.goto(`${base}/games`);
      await page.locator('input[name="name"]').fill('Seven-letter dictionary trial');
      await page.locator('input[name="answer"]').fill('OUTCAST');
      await page.locator('#create-game button').click();
      await page.waitForFunction(() => document.querySelector('castawordle-game')?.state);
      assert.equal(await page.locator('.board-row').first().locator('.tile').count(), 7);
      for (const letter of 'SWADDLE') await key(letter);
      await key('Enter');
      await page.waitForFunction(() => document.querySelector('castawordle-game').state.guesses.length === 1);
      assert.equal(await page.locator('.board-row').first().innerText().then(t => t.replace(/\s/g, '')), 'SWADDLE');
      await page.goto(`${base}/games`);
      const episode = mobile ? '3' : '4';
      await page.selectOption('select[name="episode_number"]', episode);
      await page.locator('input[name="name"]').fill(`Prepared episode ${episode}`);
      await page.locator('input[name="answer"]').fill('TORCH');
      await page.locator('#create-game button').click();
      await page.waitForFunction(() => document.querySelector('castawordle-game')?.state);
      const prepared = await page.evaluate(() => document.querySelector('castawordle-game').state);
      assert.equal(prepared.game.test, false);
      assert.equal(prepared.game.episode_number, Number(episode));
      assert.equal(prepared.status, 'not_open');
      assert.equal(prepared.guesses.length, 0);
      await page.goto(`${base}/games`);
      assert.equal(await page.locator(`select[name="episode_number"] option[value="${episode}"]`).count(), 0);
      await page.goto(`${base}/games/button`); // shows the open game in place
      const buttonPath = new URL(await page.locator('.button-stage').getAttribute('action'), base).pathname;
      assert.match(buttonPath, /^\/games\/button\/[0-9a-f-]+$/);
      const count = await page.locator('.press-count').innerText();
      assert.match(count, /^count: \d+$/);
      const before = Number(count.slice('count: '.length));
      if (before === 0) assert.equal(await page.locator('.button-hint').innerText(), 'Will you press the button?');
      // Press to the first noise count (10): its hint appears.
      const presses = 10 - before;
      for (let i = 0; i < presses; i++) {
        const pressed = page.waitForResponse(r => r.url().endsWith(buttonPath) && r.request().method() === 'POST');
        await page.locator('.the-button').click();
        assert.equal((await pressed).status(), 200);
      }
      await page.waitForFunction(() => document.querySelector('.press-count').textContent === 'count: 10');
      if (presses > 0) await page.waitForFunction(() => document.querySelector('.button-hint.shown')?.textContent === 'The tribe has spoken'); // the second run shares the player
      assert.match(await page.locator('.tribe-presses').innerText(), /^$|\d/);
      await page.screenshot({ path: path.join(os.tmpdir(), `button-${mobile ? 'phone' : 'desktop'}.png`) });

      // Spell It Out: a 30-letter phrase with 10 decoys fits; tiles are hidden until Start; drag fills,
      // swaps and returns tiles; a wrong check counts, a right one solves.
      const phrase = 'THE TRIBE HAS SPOKEN RESOURCEFULLY'; // 30 letters, one 13-letter word that wraps
      const answer = phrase.replaceAll(' ', '');
      await page.goto(`${base}/games`);
      await page.locator('input[name="phrase"]').fill(phrase.toLowerCase());
      await page.locator('input[name="decoys"]').fill('10');
      await page.locator('form[action="/games/scramble/tests"] button').click();
      await page.waitForSelector('.scramble-start');
      assert.equal(await page.locator('.scramble-tile').count(), 0);
      assert.equal(await page.locator('.scramble-slot').count(), 30);
      await page.locator('.scramble-start button').click();
      await page.waitForSelector('.scramble-tray');
      assert.equal(await page.locator('.scramble-tile').count(), 40);
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'no sideways scroll');
      const tileBox = await page.locator('.scramble-tile').first().boundingBox();
      assert(tileBox.width >= 30, `tiles at least 30px, got ${tileBox.width}`);
      await page.screenshot({ path: path.join(os.tmpdir(), `scramble-${mobile ? 'phone' : 'desktop'}-start.png`), fullPage: true });
      const drag = async (from, to) => {
        const a = await from.boundingBox(), b = await to.boundingBox();
        await page.mouse.move(a.x + a.width / 2, a.y + a.height / 2);
        await page.mouse.down();
        await page.mouse.move(a.x + a.width / 2 + 10, a.y + a.height / 2 + 10, { steps: 3 });
        await page.mouse.move(b.x + b.width / 2, b.y + b.height / 2, { steps: 8 });
        await page.mouse.up();
      };
      const slot = i => page.locator(`.scramble-slot[data-slot="${i}"]`);
      const freeTile = letter => page.locator('.scramble-tile:not(.used)', { hasText: new RegExp(`^${letter}$`) }).first();
      // Drag the answer in, but with slots 0 and 1 (T, H) reversed.
      const order = ['H', 'T', ...answer.slice(2)];
      for (let i = 0; i < order.length; i++) {
        await drag(freeTile(order[i]), slot(i));
        assert.equal(await slot(i).innerText(), order[i], `slot ${i} after drag`);
      }
      // Drag one back to the tray and back in again.
      await drag(slot(29), page.locator('.scramble-tray'));
      assert.equal(await slot(29).innerText(), '');
      await drag(freeTile(answer[29]), slot(29));
      await page.locator('.scramble-tools button[type="submit"]').click();
      await page.waitForFunction(() => document.querySelector('.scramble-wrong').textContent === '1');
      assert.match(await page.locator('.scramble-status').innerText(), /\S/);
      // Swap the first two by dragging one onto the other, then check.
      await drag(slot(0), slot(1));
      assert.equal(await slot(0).innerText() + await slot(1).innerText(), 'TH');
      await Promise.all([page.waitForNavigation(), page.locator('.scramble-tools button[type="submit"]').click()]);
      assert.match(await page.locator('.scramble-result').innerText(), /^Solved in \d+:\d\d · 1 wrong check/);
      await page.screenshot({ path: path.join(os.tmpdir(), `scramble-${mobile ? 'phone' : 'desktop'}-solved.png`), fullPage: true });
      console.log(`${mobile ? '320px phone' : 'desktop'}: OAuth, themes, 7/8-column layout, dictionary, input, retry/resume, solve, scheduled episode preparation passed; ${screenshot}`);
      await context.close();
    }
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
