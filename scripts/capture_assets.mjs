import { spawn } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import os from 'node:os';
import { execSync } from 'node:child_process';

const CHROME_PATH = 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe';
const APP_PORT = 9095;
const CDP_PORT = 9225;
const tempProfile = path.join(os.tmpdir(), 'chrome_cdp_stackloom_' + Date.now());

async function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function getDebuggerUrl() {
  for (let i = 0; i < 40; i++) {
    try {
      const res = await fetch(`http://127.0.0.1:${CDP_PORT}/json/new?http://localhost:${APP_PORT}/`, {
        method: 'PUT',
      });
      const data = await res.json();
      if (data && data.webSocketDebuggerUrl) {
        return data.webSocketDebuggerUrl;
      }
    } catch (e) {
      await sleep(250);
    }
  }
  throw new Error('Could not connect to Chrome debugging endpoint');
}

class CdpClient {
  constructor(wsUrl) {
    this.wsUrl = wsUrl;
    this.ws = null;
    this.id = 1;
    this.callbacks = new Map();
  }

  async connect() {
    return new Promise((resolve, reject) => {
      this.ws = new WebSocket(this.wsUrl);
      this.ws.onopen = () => resolve();
      this.ws.onerror = (err) => reject(err);
      this.ws.onmessage = (event) => {
        const msg = JSON.parse(event.data);
        if (msg.id && this.callbacks.has(msg.id)) {
          const cb = this.callbacks.get(msg.id);
          this.callbacks.delete(msg.id);
          if (msg.error) cb.reject(new Error(msg.error.message));
          else cb.resolve(msg.result);
        }
      };
    });
  }

  async send(method, params = {}) {
    const id = this.id++;
    return new Promise((resolve, reject) => {
      this.callbacks.set(id, { resolve, reject });
      this.ws.send(JSON.stringify({ id, method, params }));
    });
  }

  close() {
    if (this.ws) {
      this.ws.close();
    }
  }
}

async function run() {
  const outputDir = path.resolve('B:/workgit/stackloom/docs/images');
  if (!fs.existsSync(outputDir)) {
    fs.mkdirSync(outputDir, { recursive: true });
  }

  console.log(`Starting StackLoom on port ${APP_PORT}...`);
  const stackloomProc = spawn(
    'B:\\workgit\\stackloom\\stackloom.exe',
    ['serve', '-p', String(APP_PORT), '-f', 'testdata/microservices/docker-compose.yml'],
    {
      cwd: 'B:\\workgit\\stackloom',
      stdio: 'pipe',
    }
  );

  let serverReady = false;
  for (let i = 0; i < 30; i++) {
    try {
      const res = await fetch(`http://localhost:${APP_PORT}/api/status`);
      if (res.ok) {
        serverReady = true;
        break;
      }
    } catch (e) {
      await sleep(200);
    }
  }

  if (!serverReady) {
    stackloomProc.kill();
    throw new Error('StackLoom server failed to start within timeout');
  }
  console.log('✓ StackLoom server is live!');

  console.log('Launching headless Chrome...');
  const chromeProc = spawn(CHROME_PATH, [
    '--headless=new',
    `--remote-debugging-port=${CDP_PORT}`,
    `--user-data-dir=${tempProfile}`,
    '--disable-gpu',
    '--no-first-run',
    '--no-default-browser-check',
    '--hide-scrollbars',
  ]);

  try {
    const wsUrl = await getDebuggerUrl();
    console.log('Connected to Chrome CDP:', wsUrl);
    const cdp = new CdpClient(wsUrl);
    await cdp.connect();

    await cdp.send('Page.enable');
    await cdp.send('DOM.enable');

    // ----------------------------------------------------
    // ASSET 1: Desktop Dashboard (1440x900, 2x retina)
    // ----------------------------------------------------
    console.log('Capturing Desktop Dashboard (1440x900)...');
    await cdp.send('Emulation.setDeviceMetricsOverride', {
      width: 1440,
      height: 900,
      deviceScaleFactor: 2,
      mobile: false,
    });
    await cdp.send('Page.navigate', { url: `http://localhost:${APP_PORT}/` });
    await sleep(2000);

    const shot1 = await cdp.send('Page.captureScreenshot', { format: 'png' });
    fs.writeFileSync(path.join(outputDir, 'stackloom_dashboard.png'), Buffer.from(shot1.data, 'base64'));
    console.log('✓ Saved stackloom_dashboard.png');

    // ----------------------------------------------------
    // ASSET 2: Blast Radius Outage Simulation on postgres
    // ----------------------------------------------------
    console.log('Simulating Outage on postgres for Blast Radius...');
    await cdp.send('Runtime.evaluate', {
      expression: `
        // Find postgres node and click What-If
        const nodes = Array.from(document.querySelectorAll('.react-flow__node'));
        const pgNode = nodes.find(n => n.textContent && n.textContent.includes('postgres'));
        if (pgNode) {
          const btn = Array.from(pgNode.querySelectorAll('button')).find(b => b.textContent && b.textContent.includes('What-If'));
          if (btn) btn.click();
        }
      `,
    });
    await sleep(1500);

    // Zoom/fit view to see the outage
    await cdp.send('Runtime.evaluate', {
      expression: `
        const fitBtn = document.querySelector('.react-flow__controls-fitview');
        if (fitBtn) fitBtn.click();
      `,
    });
    await sleep(800);

    const shot2 = await cdp.send('Page.captureScreenshot', { format: 'png' });
    fs.writeFileSync(path.join(outputDir, 'stackloom_blast_radius.png'), Buffer.from(shot2.data, 'base64'));
    console.log('✓ Saved stackloom_blast_radius.png');

    // ----------------------------------------------------
    // ASSET 3: Pre-Flight Linter Drawer with 1-Click Fix
    // ----------------------------------------------------
    console.log('Clearing outage simulation and opening Pre-Flight Linter Drawer...');
    await cdp.send('Runtime.evaluate', {
      expression: `
        // Click clear outage button
        const clearBtn = Array.from(document.querySelectorAll('button')).find(b => b.title && b.title.includes('Clear Outage'));
        if (clearBtn) clearBtn.click();
      `,
    });
    await sleep(500);

    await cdp.send('Runtime.evaluate', {
      expression: `
        const linterPill = Array.from(document.querySelectorAll('button')).find(b => b.textContent && b.textContent.includes('Critical'));
        if (linterPill) linterPill.click();
      `,
    });
    await sleep(1200);

    const shot3 = await cdp.send('Page.captureScreenshot', { format: 'png' });
    fs.writeFileSync(path.join(outputDir, 'stackloom_linter.png'), Buffer.from(shot3.data, 'base64'));
    console.log('✓ Saved stackloom_linter.png');

    // ----------------------------------------------------
    // ASSET 4: Mobile Responsive View (390x844, Top-to-Bottom)
    // ----------------------------------------------------
    console.log('Capturing Mobile View (390x844)...');
    // Close drawer
    await cdp.send('Runtime.evaluate', {
      expression: `
        const closeBtn = document.querySelector('aside button');
        if (closeBtn) closeBtn.click();
      `,
    });
    await sleep(400);

    // Switch layout direction to TB for vertical phone screen!
    await cdp.send('Runtime.evaluate', {
      expression: `
        const toggleBtn = Array.from(document.querySelectorAll('button')).find(b => b.title && b.title.includes('Switch layout'));
        if (toggleBtn) toggleBtn.click();
      `,
    });
    await sleep(800);

    await cdp.send('Emulation.setDeviceMetricsOverride', {
      width: 390,
      height: 844,
      deviceScaleFactor: 3,
      mobile: true,
    });
    await sleep(800);

    // Fit view on mobile
    await cdp.send('Runtime.evaluate', {
      expression: `
        const fitBtn = document.querySelector('.react-flow__controls-fitview');
        if (fitBtn) fitBtn.click();
      `,
    });
    await sleep(800);

    const shot4 = await cdp.send('Page.captureScreenshot', { format: 'png' });
    fs.writeFileSync(path.join(outputDir, 'stackloom_mobile.png'), Buffer.from(shot4.data, 'base64'));
    console.log('✓ Saved stackloom_mobile.png');

    // ----------------------------------------------------
    // ASSET 5: Animated Demo GIF Frames
    // ----------------------------------------------------
    console.log('Generating frames for Animated Demo GIF...');
    const framesDir = path.join(os.tmpdir(), 'stackloom_frames_' + Date.now());
    if (!fs.existsSync(framesDir)) fs.mkdirSync(framesDir, { recursive: true });

    // Reset desktop resolution
    await cdp.send('Emulation.setDeviceMetricsOverride', {
      width: 1366,
      height: 768,
      deviceScaleFactor: 1.5,
      mobile: false,
    });

    // Reset page to initial state
    await cdp.send('Page.navigate', { url: `http://localhost:${APP_PORT}/` });
    await sleep(2000);
    const f1 = await cdp.send('Page.captureScreenshot', { format: 'png' });
    fs.writeFileSync(path.join(framesDir, 'frame_01.png'), Buffer.from(f1.data, 'base64'));

    // Frame 2: Select postgres node
    await cdp.send('Runtime.evaluate', {
      expression: `
        const nodes = Array.from(document.querySelectorAll('.react-flow__node'));
        const pg = nodes.find(n => n.textContent && n.textContent.includes('postgres'));
        if (pg) pg.click();
      `,
    });
    await sleep(900);
    const f2 = await cdp.send('Page.captureScreenshot', { format: 'png' });
    fs.writeFileSync(path.join(framesDir, 'frame_02.png'), Buffer.from(f2.data, 'base64'));

    // Frame 3: What-If Outage simulation on postgres
    await cdp.send('Runtime.evaluate', {
      expression: `
        const nodes = Array.from(document.querySelectorAll('.react-flow__node'));
        const pg = nodes.find(n => n.textContent && n.textContent.includes('postgres'));
        if (pg) {
          const btn = Array.from(pg.querySelectorAll('button')).find(b => b.textContent && b.textContent.includes('What-If'));
          if (btn) btn.click();
        }
      `,
    });
    await sleep(1200);
    const f3 = await cdp.send('Page.captureScreenshot', { format: 'png' });
    fs.writeFileSync(path.join(framesDir, 'frame_03.png'), Buffer.from(f3.data, 'base64'));

    // Frame 4: Open Pre-Flight Linter
    await cdp.send('Runtime.evaluate', {
      expression: `
        const clearBtn = Array.from(document.querySelectorAll('button')).find(b => b.title && b.title.includes('Clear Outage'));
        if (clearBtn) clearBtn.click();
        setTimeout(() => {
          const linterPill = Array.from(document.querySelectorAll('button')).find(b => b.textContent && b.textContent.includes('Critical'));
          if (linterPill) linterPill.click();
        }, 200);
      `,
    });
    await sleep(1200);
    const f4 = await cdp.send('Page.captureScreenshot', { format: 'png' });
    fs.writeFileSync(path.join(framesDir, 'frame_04.png'), Buffer.from(f4.data, 'base64'));

    // Frame 5: Filter Linter to Critical tab
    await cdp.send('Runtime.evaluate', {
      expression: `
        const critTab = Array.from(document.querySelectorAll('aside button')).find(b => b.textContent && b.textContent.includes('Critical'));
        if (critTab) critTab.click();
      `,
    });
    await sleep(1000);
    const f5 = await cdp.send('Page.captureScreenshot', { format: 'png' });
    fs.writeFileSync(path.join(framesDir, 'frame_05.png'), Buffer.from(f5.data, 'base64'));

    // Frame 6: Open live terminal logs
    await cdp.send('Runtime.evaluate', {
      expression: `
        const asideClose = document.querySelector('aside button');
        if (asideClose) asideClose.click();
        setTimeout(() => {
          const logBtn = Array.from(document.querySelectorAll('button')).find(b => b.title && b.title.includes('logs'));
          if (logBtn) logBtn.click();
        }, 300);
      `,
    });
    await sleep(1200);
    const f6 = await cdp.send('Page.captureScreenshot', { format: 'png' });
    fs.writeFileSync(path.join(framesDir, 'frame_06.png'), Buffer.from(f6.data, 'base64'));

    cdp.close();

    // ----------------------------------------------------
    // ASSEMBLE DEMO GIF VIA FFMPEG
    // ----------------------------------------------------
    console.log('Compiling animated GIF via ffmpeg...');
    const gifPath = path.join(outputDir, 'stackloom_demo.gif');
    
    const expandedDir = path.join(os.tmpdir(), 'stackloom_expanded_' + Date.now());
    fs.mkdirSync(expandedDir, { recursive: true });

    const frameList = ['frame_01.png', 'frame_02.png', 'frame_03.png', 'frame_04.png', 'frame_05.png', 'frame_06.png'];
    let counter = 1;
    for (const f of frameList) {
      const src = path.join(framesDir, f);
      for (let r = 0; r < 24; r++) {
        const destName = `img_${String(counter++).padStart(4, '0')}.png`;
        fs.copyFileSync(src, path.join(expandedDir, destName));
      }
    }

    const ffmpegCmd = `ffmpeg -y -framerate 12 -i "${path.join(expandedDir, 'img_%04d.png')}" -vf "fps=12,scale=1080:-1:flags=lanczos,split[s0][s1];[s0]palettegen=max_colors=128[p];[s1][p]paletteuse=dither=bayer" "${gifPath}"`;
    execSync(ffmpegCmd, { stdio: 'inherit' });
    console.log(`✓ Successfully compiled ${gifPath}`);

    try {
      fs.rmSync(framesDir, { recursive: true, force: true });
      fs.rmSync(expandedDir, { recursive: true, force: true });
    } catch (e) {}

  } finally {
    chromeProc.kill('SIGKILL');
    stackloomProc.kill('SIGKILL');
    try {
      fs.rmSync(tempProfile, { recursive: true, force: true });
    } catch (e) {}
  }
}

run().catch((err) => {
  console.error('Fatal error during capture:', err);
  process.exit(1);
});
