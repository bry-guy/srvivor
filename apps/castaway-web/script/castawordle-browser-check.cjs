const assert = require('node:assert/strict');
const { chromium } = require('playwright');
const base = process.argv[2];
(async () => {
  const browser = await chromium.launch({ headless: true });
  try {
    for (const mobile of [true, false]) {
      const context = await browser.newContext({ viewport: mobile ? { width: 320, height: 800 } : { width: 1280, height: 900 }, isMobile: mobile, hasTouch: mobile, colorScheme: 'dark' });
      const page = await context.newPage();
      await page.goto(`${base}/castawordle`);
      await page.waitForSelector('#create-game');
      assert.equal(new URL(page.url()).pathname, '/castawordle');
      const dark = await page.evaluate(() => getComputedStyle(document.body).backgroundColor);
      await page.selectOption('#theme', 'light');
      const light = await page.evaluate(() => getComputedStyle(document.body).backgroundColor);
      assert.notEqual(light, dark);
      await page.locator('input[name="name"]').fill(mobile ? 'Phone trial' : 'Desktop trial');
      await page.locator('input[name="answer"]').fill('SURVIVOR');
      await page.locator('#create-game button').click();
      await page.waitForFunction(() => document.querySelector('castawordle-game')?.state);
      const gameURL = page.url();
      assert.match(new URL(gameURL).pathname, /^\/castawordle\/[0-9a-f-]+$/);
      assert.equal(await page.locator('.board-row').count(), 6);
      assert.equal(await page.locator('.board-row').first().locator('.tile').count(), 8);
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
      const key = value => page.locator(`.key[data-key="${value}"]`).click();
      let postedGuesses = 0;
      page.on('request', request => { if (request.method() === 'POST' && request.url().endsWith('/play/guesses')) postedGuesses++; });
      for (const letter of 'ZZZZZZZ') await key(letter);
      assert.equal(postedGuesses, 0);
      await key('Z');
      await page.waitForFunction(() => !document.querySelector('castawordle-game').pending);
      assert.equal(await page.evaluate(() => document.querySelector('castawordle-game').state.guesses.length), 0);
      assert.match(await page.locator('.game-message').innerText(), /dictionary/);
      for (let i = 0; i < 8; i++) await key('Backspace');
      await page.route('**/play/guesses', async route => { await route.fetch(); await route.abort(); }, { times: 1 });
      for (const letter of 'CAMPFIRE') await key(letter);
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
      console.log(`${mobile ? '320px phone' : 'desktop'}: OAuth return, themes, 8-column layout, touch/physical input, lost-response retry, resume, solve passed; ${screenshot}`);
      await context.close();
    }
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
