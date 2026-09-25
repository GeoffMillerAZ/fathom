const OPERATION_TIMEOUT_MS = 15_000;
const EXIT_TIMEOUT_MS = 5_000;

function lifecycleError(code) {
  return new Error(code);
}

export async function bounded(operation, milliseconds, code) {
  let timer;
  try {
    return await Promise.race([
      Promise.resolve(operation),
      new Promise((_, reject) => {
        timer = setTimeout(() => reject(lifecycleError(code)), milliseconds);
      }),
    ]);
  } catch {
    throw lifecycleError(code);
  } finally {
    clearTimeout(timer);
  }
}

function running(child) {
  return child && child.exitCode === null && child.signalCode === null;
}

async function waitForExit(child) {
  if (!running(child)) return;
  await bounded(new Promise(resolve => child.once('exit', resolve)), EXIT_TIMEOUT_MS,
    'browser-lifecycle-process-exit-failed');
}

async function forceServerExit(server, child) {
  if (!child || running(child)) {
    try {
      await bounded(server.kill(), EXIT_TIMEOUT_MS, 'browser-lifecycle-server-kill-failed');
    } catch {
      if (child) {
        try { child.kill('SIGKILL'); } catch {}
      }
    }
  }
  if (running(child)) {
    try { child.kill('SIGKILL'); } catch {}
  }
  await waitForExit(child);
}

async function disposeAfterConnectFailure(server, child) {
  try {
    await bounded(server.close(), EXIT_TIMEOUT_MS, 'browser-lifecycle-server-close-failed');
    await waitForExit(child);
  } catch {
    await forceServerExit(server, child);
  }
}

async function disposeLateLaunch(server) {
  let child;
  try { child = server.process(); } catch {}
  await disposeAfterConnectFailure(server, child);
}

export async function closeContext(context) {
  if (!context || typeof context.close !== 'function') {
    throw lifecycleError('browser-context-cleanup-failed');
  }
  await bounded(context.close(), OPERATION_TIMEOUT_MS, 'browser-context-cleanup-failed');
}

export async function launchManaged(chromium, options) {
  let server;
  let launch;
  try {
    // Keep the original promise: if our independent deadline wins before
    // Playwright's own deadline settles, its late BrowserServer still has an
    // owner and is disposed below instead of becoming an orphan.
    launch = Promise.resolve().then(() =>
      chromium.launchServer({...options, timeout: OPERATION_TIMEOUT_MS}));
    server = await bounded(
      launch,
      OPERATION_TIMEOUT_MS,
      'browser-lifecycle-launch-failed',
    );
  } catch {
    launch?.then(late => disposeLateLaunch(late).catch(() => {}), () => {});
    throw lifecycleError('browser-lifecycle-launch-failed');
  }

  const child = server.process();
  let browser;
  try {
    browser = await bounded(
      chromium.connect(server.wsEndpoint(), {timeout: OPERATION_TIMEOUT_MS}),
      OPERATION_TIMEOUT_MS,
      'browser-lifecycle-connect-failed',
    );
  } catch {
    try {
      await disposeAfterConnectFailure(server, child);
    } catch {
      throw lifecycleError('browser-lifecycle-connect-cleanup-failed');
    }
    throw lifecycleError('browser-lifecycle-connect-failed');
  }

  let closePromise;
  const close = () => {
    if (closePromise) return closePromise;
    closePromise = (async () => {
      let graceful = true;
      try {
        await bounded(browser.close(), OPERATION_TIMEOUT_MS, 'browser-lifecycle-browser-close-failed');
      } catch {
        graceful = false;
      }
      try {
        await bounded(server.close(), EXIT_TIMEOUT_MS, 'browser-lifecycle-server-close-failed');
        await waitForExit(child);
      } catch {
        graceful = false;
      }
      if (!graceful) {
        try {
          await forceServerExit(server, child);
        } catch {
          throw lifecycleError('browser-lifecycle-forced-cleanup-failed');
        }
        throw lifecycleError('browser-lifecycle-graceful-cleanup-failed');
      }
    })();
    return closePromise;
  };

  return {browser, close};
}
