{ // own block: page scripts share one global scope
// Inside Track (prototype): top-down. Flick three balls up a narrow track into three slots; roll off the
// edge and the ball drops into the water and comes back. Board units are about 3.4× the real board so the
// ball is thumb-sized on a phone.
const BOARD = { w: 1.4, h: 2.6 };
const LANE = { w: 0.34, from: 0.5, to: 1.85 };
const TARGET = { w: 1.2, from: 1.85, to: 2.5 };
const PAD = 0.5;
const R = 0.075;
const SLOT_R = 0.1;
const SLOTS = [[-0.36, 2.25], [0, 2.25], [0.36, 2.25]];
const ROLL_DECEL = 0.6; // rolling resistance (plywood μr ≈ 0.035 × g, scaled)
const CAPTURE = 1.1;    // slower than this over a slot and it drops in
const DIP = 10;         // pull toward a slot centre while over it
const MAX_SPEED = 6;
const START_Y = 0.22;

const $ = (id) => document.getElementById(id);
const canvas = $("c"), ctx = canvas.getContext("2d");
let scale = 1, cx = 0, cy = 0, W = 0, H = 0;
function resize() {
  const d = devicePixelRatio || 1;
  W = innerWidth; H = innerHeight;
  canvas.width = W * d; canvas.height = H * d;
  scale = Math.min(W / (BOARD.w + 0.2), (H - 60) / (BOARD.h + 0.15));
  cx = W / 2; cy = H - 14 - (H - 60 - BOARD.h * scale) / 2; // board bottom, in CSS px
  ctx.setTransform(d, 0, 0, d, 0, 0);
  grain = null;
}
addEventListener("resize", resize);
const sx = (x) => cx + x * scale, sy = (y) => cy - y * scale;
const toBoard = (e) => ({ x: (e.clientX - cx) / scale, y: (cy - e.clientY) / scale });

// Wood grain, drawn once per size into an offscreen canvas.
let grain = null;
function woodPattern() {
  if (grain) return grain;
  const c = document.createElement("canvas"); c.width = 256; c.height = 256;
  const g = c.getContext("2d");
  g.fillStyle = "#c79a5c"; g.fillRect(0, 0, 256, 256);
  for (let i = 0; i < 70; i++) {
    const x = Math.random() * 256, w = 0.5 + Math.random() * 2.5;
    g.strokeStyle = `rgba(${90 + Math.random() * 40},${55 + Math.random() * 25},20,${0.08 + Math.random() * 0.14})`;
    g.lineWidth = w; g.beginPath(); g.moveTo(x, 0);
    for (let y = 0; y <= 256; y += 16) g.lineTo(x + Math.sin(y / 30 + i) * 3, y);
    g.stroke();
  }
  return (grain = ctx.createPattern(c, "repeat"));
}

let filled, ball, tries, started, finished, fall, trailPts, particles, playing = false;
function reset() {
  filled = [false, false, false]; tries = 0; started = null; finished = null; particles = [];
  newBall(); updateHud();
}
function newBall() { ball = { x: 0, y: START_Y, vx: 0, vy: 0, held: false, rolling: false, spin: 0 }; fall = null; trailPts = []; }

function onBoard(x, y) {
  if (y < LANE.from) return y > 0 && Math.abs(x) < PAD;
  if (y < LANE.to) return Math.abs(x) < LANE.w / 2;
  if (y < TARGET.to) return Math.abs(x) < TARGET.w / 2;
  return false;
}

let drag = [], aim = null;
canvas.addEventListener("pointerdown", (e) => {
  const p = toBoard(e);
  if (!playing || ball.rolling || fall || finished || Math.hypot(p.x - ball.x, p.y - ball.y) > 0.3) return;
  canvas.setPointerCapture(e.pointerId);
  ball.held = true; drag = [{ ...p, t: performance.now() }];
});
canvas.addEventListener("pointermove", (e) => {
  if (!ball.held) return;
  const p = toBoard(e);
  ball.x = Math.max(-(PAD - R), Math.min(PAD - R, p.x)); ball.y = Math.max(R, Math.min(LANE.from - R, p.y));
  const now = performance.now();
  drag.push({ x: ball.x, y: ball.y, t: now });
  while (drag.length > 2 && drag[0].t < now - 80) drag.shift();
  const a = drag[0], dt = (now - a.t) / 1000;
  aim = dt > 0 ? { vx: (ball.x - a.x) / dt, vy: (ball.y - a.y) / dt } : null;
});
const release = () => {
  if (!ball.held) return;
  ball.held = false; aim = null;
  const a = drag[0], b = drag[drag.length - 1], dt = (b.t - a.t) / 1000;
  if (dt <= 0 || b.y - a.y < 0.02) return; // a tap or a pull back: no throw
  let vx = (b.x - a.x) / dt, vy = (b.y - a.y) / dt;
  const s = Math.hypot(vx, vy); if (s > MAX_SPEED) { vx *= MAX_SPEED / s; vy *= MAX_SPEED / s; }
  Object.assign(ball, { vx, vy, rolling: true });
  tries++; started ??= performance.now(); updateHud();
};
canvas.addEventListener("pointerup", release);
canvas.addEventListener("pointercancel", release);

function step(dt) {
  if (!ball.rolling) return;
  for (let i = 0; i < SLOTS.length; i++) {
    if (filled[i]) continue;
    const dx = SLOTS[i][0] - ball.x, dy = SLOTS[i][1] - ball.y, d = Math.hypot(dx, dy);
    if (d < SLOT_R) {
      ball.vx += dx / (d || 1) * DIP * dt; ball.vy += dy / (d || 1) * DIP * dt;
      if (Math.hypot(ball.vx, ball.vy) < CAPTURE && d < SLOT_R - R / 2) { filled[i] = true; return land(i); }
    }
  }
  const s = Math.hypot(ball.vx, ball.vy), ns = Math.max(0, s - ROLL_DECEL * dt);
  if (ns === 0) { ball.vx = ball.vy = 0; } else { ball.vx *= ns / s; ball.vy *= ns / s; }
  ball.x += ball.vx * dt; ball.y += ball.vy * dt; ball.spin += ns * dt / R;
  if (ball.y > TARGET.to - R && Math.abs(ball.x) < TARGET.w / 2) { ball.y = TARGET.to - R; ball.vy *= -0.4; burst(ball.x, ball.y + R, "#e9c48a", 6); }
  if (!onBoard(ball.x, ball.y)) { ball.rolling = false; fall = { t: 0, x: ball.x, y: ball.y, vx: ball.vx * 0.3, vy: ball.vy * 0.3 }; navigator.vibrate?.(60); toast("Off the edge! Back to the start."); return; }
  if (ns === 0) { ball.rolling = false; fall = { t: 0, stopped: true }; toast(ball.y < LANE.to ? "Too soft. Give it more." : "Stopped short of a slot."); }
}
function land(i) {
  ball.rolling = false;
  navigator.vibrate?.([30, 40, 30]);
  burst(SLOTS[i][0], SLOTS[i][1], "#ffd36b", 24);
  updateHud();
  const n = filled.filter(Boolean).length;
  if (n === 3) {
    finished = performance.now();
    for (let k = 0; k < 80; k++) burst((Math.random() - 0.5) * BOARD.w, BOARD.h * Math.random(), ["#ffd36b", "#ff7a59", "#7be0c3", "#fff"][k % 4], 1);
    setTimeout(() => { $("final").textContent = fmt(finished - started); $("finalRolls").textContent = `${tries} rolls`; $("done").classList.remove("hidden"); }, 900);
    return;
  }
  toast(`In! ${3 - n} to go.`); newBall();
}

function burst(x, y, color, n) {
  for (let i = 0; i < n; i++) {
    const a = Math.random() * Math.PI * 2, v = 0.3 + Math.random() * 1.2;
    particles.push({ x, y, vx: Math.cos(a) * v, vy: Math.sin(a) * v, life: 0.6 + Math.random() * 0.6, color });
  }
}
let toastTimer;
function toast(t) { const el = $("msg"); el.textContent = t; el.classList.add("show"); clearTimeout(toastTimer); toastTimer = setTimeout(() => el.classList.remove("show"), 1600); }
const fmt = (ms) => { const s = ms / 1000; return `${Math.floor(s / 60)}:${(s % 60).toFixed(1).padStart(4, "0")}`; };
function updateHud() {
  $("tries").textContent = tries;
  [...$("balls").children].forEach((el, i) => el.classList.toggle("on", filled[i]));
}

// Drawing, in screen px.
function rect(x, y, w, h) { ctx.rect(sx(x), sy(y + h), w * scale, h * scale); }
function boardPath() {
  ctx.beginPath();
  rect(-PAD, 0, PAD * 2, LANE.from);
  rect(-LANE.w / 2, LANE.from - 0.01, LANE.w, LANE.to - LANE.from + 0.02);
  rect(-TARGET.w / 2, TARGET.from, TARGET.w, TARGET.to - TARGET.from);
}
let t0 = performance.now();
function draw(now) {
  const time = (now - t0) / 1000;
  ctx.clearRect(0, 0, W, H);
  // water shimmer
  ctx.strokeStyle = "rgba(255,255,255,.06)"; ctx.lineWidth = 2;
  for (let y = 20; y < H; y += 28) {
    ctx.beginPath();
    for (let x = 0; x <= W; x += 20) ctx.lineTo(x, y + Math.sin(x / 40 + time * 1.5 + y) * 3);
    ctx.stroke();
  }
  // board: drop shadow, edge thickness, wood top
  ctx.save(); ctx.translate(8, 14); boardPath(); ctx.fillStyle = "rgba(0,0,0,.35)"; ctx.filter = "blur(8px)"; ctx.fill(); ctx.restore();
  ctx.save(); ctx.translate(0, 7); boardPath(); ctx.fillStyle = "#6e4420"; ctx.fill(); ctx.restore();
  boardPath(); ctx.fillStyle = woodPattern(); ctx.fill();
  const shade = ctx.createLinearGradient(0, sy(BOARD.h), 0, sy(0));
  shade.addColorStop(0, "rgba(0,0,0,.18)"); shade.addColorStop(1, "rgba(255,240,200,.12)");
  ctx.fillStyle = shade; ctx.fill();
  ctx.strokeStyle = "rgba(60,30,10,.5)"; ctx.lineWidth = 1.5; ctx.stroke();
  // lane guide paint + start line
  ctx.strokeStyle = "rgba(255,255,255,.35)"; ctx.setLineDash([8, 10]); ctx.lineWidth = 2;
  ctx.beginPath(); ctx.moveTo(sx(0), sy(LANE.from)); ctx.lineTo(sx(0), sy(LANE.to)); ctx.stroke(); ctx.setLineDash([]);
  ctx.strokeStyle = "rgba(255,255,255,.55)"; ctx.lineWidth = 3;
  ctx.beginPath(); ctx.moveTo(sx(-PAD + 0.05), sy(LANE.from - 0.04)); ctx.lineTo(sx(PAD - 0.05), sy(LANE.from - 0.04)); ctx.stroke();
  // back wall
  ctx.fillStyle = "#5a3416"; ctx.fillRect(sx(-TARGET.w / 2), sy(TARGET.to + 0.06), TARGET.w * scale, 0.06 * scale);
  ctx.fillStyle = "#8a5a2c"; ctx.fillRect(sx(-TARGET.w / 2), sy(TARGET.to + 0.06), TARGET.w * scale, 0.02 * scale);
  // slots: holes with depth, glowing ring when empty
  SLOTS.forEach(([x, y], i) => {
    const g = ctx.createRadialGradient(sx(x), sy(y) - 4, 2, sx(x), sy(y), SLOT_R * scale);
    g.addColorStop(0, "#120802"); g.addColorStop(0.75, "#2b1608"); g.addColorStop(1, "#7a4b22");
    ctx.fillStyle = g; ctx.beginPath(); ctx.arc(sx(x), sy(y), SLOT_R * scale, 0, 7); ctx.fill();
    if (!filled[i]) {
      ctx.strokeStyle = `rgba(255,215,120,${0.35 + 0.25 * Math.sin(time * 3 + i)})`; ctx.lineWidth = 3;
      ctx.beginPath(); ctx.arc(sx(x), sy(y), SLOT_R * scale + 3, 0, 7); ctx.stroke();
    } else drawBall(x, y, R * 0.92, 0, 0.8);
  });
  // aim guide while holding
  if (ball.held) {
    const v = aim && aim.vy > 0 ? aim : { vx: 0, vy: 1.5 };
    const s = Math.min(1, Math.hypot(v.vx, v.vy) / MAX_SPEED), len = 0.25 + s * 0.9, a = Math.atan2(v.vy, v.vx);
    ctx.strokeStyle = `rgba(255,${Math.round(230 - s * 150)},90,.9)`; ctx.lineWidth = 6; ctx.lineCap = "round"; ctx.setLineDash([2, 12]);
    ctx.beginPath(); ctx.moveTo(sx(ball.x), sy(ball.y)); ctx.lineTo(sx(ball.x + Math.cos(a) * len), sy(ball.y + Math.sin(a) * len)); ctx.stroke(); ctx.setLineDash([]);
  }
  // trail
  if (ball.rolling) { trailPts.push([ball.x, ball.y]); if (trailPts.length > 14) trailPts.shift(); } else if (trailPts.length) trailPts.shift();
  trailPts.forEach(([x, y], i) => { ctx.fillStyle = `rgba(255,240,200,${(i / trailPts.length) * 0.25})`; ctx.beginPath(); ctx.arc(sx(x), sy(y), R * scale * (i / trailPts.length), 0, 7); ctx.fill(); });
  // ball (falling: drifts, shrinks, splashes)
  if (!finished) {
    if (fall && !fall.stopped) {
      const k = Math.max(0, 1 - fall.t / 0.55);
      drawBall(fall.x + fall.vx * fall.t, fall.y + fall.vy * fall.t, R * (0.35 + 0.65 * k), ball.spin, k);
      if (fall.t > 0.4 && !fall.splashed) { fall.splashed = true; burst(fall.x + fall.vx * fall.t, fall.y + fall.vy * fall.t, "#bfeff5", 16); }
    } else drawBall(ball.x, ball.y, ball.held ? R * 1.1 : R, ball.spin, fall?.stopped ? Math.max(0, 1 - fall.t / 0.6) : 1, ball.held);
  }
  // particles
  particles.forEach((p) => { ctx.globalAlpha = Math.max(0, p.life); ctx.fillStyle = p.color; ctx.beginPath(); ctx.arc(sx(p.x), sy(p.y), 3, 0, 7); ctx.fill(); });
  ctx.globalAlpha = 1;
  $("time").textContent = started ? fmt((finished ?? performance.now()) - started) : "0:00.0";
}
function drawBall(x, y, r, spin, alpha = 1, lifted = false) {
  const px = sx(x), py = sy(y), rr = r * scale;
  ctx.globalAlpha = alpha;
  ctx.fillStyle = "rgba(0,0,0,.35)"; ctx.beginPath(); ctx.ellipse(px + rr * 0.35, py + rr * (lifted ? 0.8 : 0.45), rr, rr * 0.8, 0, 0, 7); ctx.fill();
  const g = ctx.createRadialGradient(px - rr * 0.35, py - rr * 0.4, rr * 0.1, px, py, rr);
  g.addColorStop(0, "#ffffff"); g.addColorStop(0.45, "#f1e6cf"); g.addColorStop(1, "#9c8a68");
  ctx.fillStyle = g; ctx.beginPath(); ctx.arc(px, py, rr, 0, 7); ctx.fill();
  // a stripe so you can see it roll
  ctx.save(); ctx.beginPath(); ctx.arc(px, py, rr, 0, 7); ctx.clip();
  ctx.strokeStyle = "rgba(200,70,40,.75)"; ctx.lineWidth = rr * 0.28;
  const off = Math.sin(spin) * rr;
  ctx.beginPath(); ctx.moveTo(px - rr, py + off); ctx.lineTo(px + rr, py + off); ctx.stroke(); ctx.restore();
  ctx.globalAlpha = 1;
}

let last = performance.now();
function frame(now) {
  const dt = Math.min(0.05, (now - last) / 1000); last = now;
  for (let i = 0; i < 8; i++) step(dt / 8); // substeps so a fast ball can't skip a slot
  if (fall && (fall.t += dt) > (fall.stopped ? 0.6 : 0.8)) newBall();
  particles.forEach((p) => { p.x += p.vx * dt; p.y += p.vy * dt; p.vx *= 0.96; p.vy *= 0.96; p.life -= dt; });
  particles = particles.filter((p) => p.life > 0);
  draw(now);
  requestAnimationFrame(frame);
}

$("go").onclick = () => { $("intro").classList.add("hidden"); playing = true; toast("Grab the ball and flick it up."); };
$("again").onclick = () => { $("done").classList.add("hidden"); reset(); };
resize(); reset(); requestAnimationFrame(frame);
}
