// scripts/e2e-helpers.js
//
// Playwright-style terminal locators for ttyd E2E tests.
// Pass the Playwright `page` object to each function.
// Does NOT import Playwright — relies on the caller's page instance.

/**
 * Extract all text from the xterm.js buffer.
 * @param {import('playwright').Page} page
 * @returns {Promise<string>}
 */
async function getTerminalContent(page) {
  return page.evaluate(() => {
    const term = window.term;
    if (!term || !term.buffer) return '';
    const buf = term.buffer.active;
    const lines = [];
    for (let i = 0; i < buf.length; i++) {
      const line = buf.getLine(i);
      if (line) lines.push(line.translateToString(true));
    }
    return lines.join('\n');
  });
}

/**
 * Find text in the terminal buffer.
 * @param {import('playwright').Page} page
 * @param {string|RegExp} text
 * @param {{ exact?: boolean, regex?: boolean, timeout?: number, pollInterval?: number }} [options]
 * @returns {Promise<{ found: boolean, line: number, content: string }>}
 */
async function getByText(page, text, options = {}) {
  const { exact = false, regex = false } = options;

  const content = await getTerminalContent(page);
  const lines = content.split('\n');

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    let matched = false;

    if (regex) {
      const re = text instanceof RegExp ? text : new RegExp(text);
      matched = re.test(line);
    } else if (exact) {
      matched = line === text;
    } else {
      matched = line.includes(text instanceof RegExp ? text.source : String(text));
    }

    if (matched) {
      return { found: true, line: i, content: line };
    }
  }

  return { found: false, line: -1, content: '' };
}

/**
 * Wait until text appears in the terminal buffer.
 * @param {import('playwright').Page} page
 * @param {string|RegExp} text
 * @param {{ exact?: boolean, regex?: boolean, timeout?: number, pollInterval?: number }} [options]
 */
async function waitForText(page, text, options = {}) {
  const { timeout = 10000, pollInterval = 200 } = options;
  const deadline = Date.now() + timeout;

  while (Date.now() < deadline) {
    const result = await getByText(page, text, options);
    if (result.found) return result;
    await sleep(pollInterval);
  }

  const content = await getTerminalContent(page);
  const last20 = content.split('\n').slice(-20).join('\n');
  throw new Error(
    `waitForText: "${text}" not found within ${timeout}ms.\nLast 20 lines:\n${last20}`
  );
}

/**
 * Wait until terminal output stops changing.
 * @param {import('playwright').Page} page
 * @param {{ stableDuration?: number, timeout?: number, pollInterval?: number }} [options]
 */
async function waitForStable(page, options = {}) {
  const { stableDuration = 500, timeout = 10000, pollInterval = 200 } = options;
  const deadline = Date.now() + timeout;

  let previous = await getTerminalContent(page);
  let stableSince = Date.now();

  while (Date.now() < deadline) {
    await sleep(pollInterval);
    const current = await getTerminalContent(page);

    if (current !== previous) {
      previous = current;
      stableSince = Date.now();
    } else if (Date.now() - stableSince >= stableDuration) {
      return;
    }
  }

  const content = await getTerminalContent(page);
  const last20 = content.split('\n').slice(-20).join('\n');
  throw new Error(
    `waitForStable: terminal did not stabilize within ${timeout}ms.\nLast 20 lines:\n${last20}`
  );
}

/**
 * Assert that text IS visible in the terminal.
 * @param {import('playwright').Page} page
 * @param {string|RegExp} text
 * @param {{ exact?: boolean, regex?: boolean, timeout?: number, pollInterval?: number }} [options]
 */
async function expectVisible(page, text, options = {}) {
  const result = await getByText(page, text, options);
  if (!result.found) {
    const content = await getTerminalContent(page);
    const last20 = content.split('\n').slice(-20).join('\n');
    throw new Error(
      `expectVisible: "${text}" not found in terminal.\nLast 20 lines:\n${last20}`
    );
  }
}

/**
 * Assert that text is NOT visible in the terminal.
 * @param {import('playwright').Page} page
 * @param {string|RegExp} text
 * @param {{ exact?: boolean, regex?: boolean, timeout?: number, pollInterval?: number }} [options]
 */
async function expectNotVisible(page, text, options = {}) {
  const result = await getByText(page, text, options);
  if (result.found) {
    throw new Error(
      `expectNotVisible: "${text}" was found in terminal at line ${result.line}: ${result.content}`
    );
  }
}

/**
 * Send a command to the terminal (appends Enter) and wait for output to stabilize.
 * @param {import('playwright').Page} page
 * @param {string} command
 * @param {{ stableDuration?: number, timeout?: number }} [options]
 */
async function sendCommand(page, command, options = {}) {
  const { stableDuration = 500, timeout = 10000 } = options;

  await page.evaluate((cmd) => {
    if (window.term) window.term.input(cmd + '\r');
  }, command);

  await waitForStable(page, { stableDuration, timeout });
}

/**
 * Simple promise-based delay.
 * @param {number} ms
 * @returns {Promise<void>}
 */
function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

module.exports = {
  getTerminalContent,
  getByText,
  waitForText,
  waitForStable,
  expectVisible,
  expectNotVisible,
  sendCommand,
  sleep,
};
