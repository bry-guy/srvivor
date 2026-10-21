{ // own block: page scripts share one global scope
// Inside Track (prototype): top-down. Three narrow tracks fan out from the start pad like \|/, each ending
// in a hole. Flick one ball into each hole; roll off a track and it drops into the water and comes back.
const BOARD = { w: 2.2, h: 2.6 };
const PAD = { w: 1.0, h: 0.5 };   // start pad, centred, from y=0
const TRACK_W = 0.4;
const ANG = 30 * Math.PI / 180;   // diagonal tilt from straight up
const TRACKS = [-ANG, 0, ANG].map((a) => {
  const len = a ? 1.45 : 1.75, ux = Math.sin(a), uy = Math.cos(a), o = [0, PAD.h]; // all three leave from one shared spot
  return { a, len, ux, uy, o, hole: [o[0] + ux * (len - 0.2), o[1] + uy * (len - 0.2)] };
});
const R = 0.11;
const HOLE_R = 0.145;
const ROLL_DECEL = 1.6;  // heavy ball: slows steadily, never floats
const CAPTURE = 2.2;     // slower than this over a hole and it drops in
const DIP = 14;          // pull toward a hole centre while over it
const WALL_BOUNCE = 0.2; // the end wall soaks up most of the speed
const START_Y = 0.25;
// Throw assist: any swipe from gentle to hard lands in a speed band that reaches the holes, and the
// direction is pulled most of the way onto the nearest track, so a casual swipe toward a track just works.
const MIN_V = 1.3, MAX_V = 3.1, SNAP = 0.8;
const throwSpeed = (f) => MIN_V + (MAX_V - MIN_V) * (1 - Math.exp(-f / 3));

const $ = (id) => document.getElementById(id);
const canvas = $("c"), ctx = canvas.getContext("2d");
let scale = 1, cx = 0, cy = 0, W = 0, H = 0;
function resize() {
  const d = devicePixelRatio || 1;
  W = innerWidth; H = innerHeight;
  canvas.width = W * d; canvas.height = H * d;
  scale = Math.min(W / (BOARD.w + 0.1), (H - 60) / (BOARD.h + 0.15));
  cx = W / 2; cy = H - 14 - (H - 60 - BOARD.h * scale) / 2;
  ctx.setTransform(d, 0, 0, d, 0, 0);
  grain = null;
}
addEventListener("resize", resize);
const sx = (x) => cx + x * scale, sy = (y) => cy - y * scale;
const toBoard = (e) => ({ x: (e.clientX - cx) / scale, y: (cy - e.clientY) / scale });

let grain = null;
function woodPattern() {
  if (grain) return grain;
  const c = document.createElement("canvas"); c.width = 256; c.height = 256;
  const g = c.getContext("2d");
  g.fillStyle = "#c79a5c"; g.fillRect(0, 0, 256, 256);
  for (let i = 0; i < 70; i++) {
    const x = Math.random() * 256;
    g.strokeStyle = `rgba(${90 + Math.random() * 40},${55 + Math.random() * 25},20,${0.08 + Math.random() * 0.14})`;
    g.lineWidth = 0.5 + Math.random() * 2.5; g.beginPath(); g.moveTo(x, 0);
    for (let y = 0; y <= 256; y += 16) g.lineTo(x + Math.sin(y / 30 + i) * 3, y);
    g.stroke();
  }
  return (grain = ctx.createPattern(c, "repeat"));
}

// Position along/across a track, measured from the origin.
const local = (t, x, y) => { const dx = x - t.o[0], dy = y - t.o[1]; return { along: dx * t.ux + dy * t.uy, across: dx * t.uy - dy * t.ux }; };
function onBoard(x, y) {
  if (y >= 0 && y <= PAD.h && Math.abs(x) <= PAD.w / 2) return true;
  return TRACKS.some((t) => { const l = local(t, x, y); return l.along >= -0.15 && l.along <= t.len && Math.abs(l.across) <= TRACK_W / 2; });
}

let filled, ball, tries, started, finished, fall, particles, playing = false, path = [], pathFade = 0;
function reset() { filled = [false, false, false]; tries = 0; started = null; finished = null; particles = []; path = []; newBall(); updateHud(); }
function newBall() { ball = { x: 0, y: START_Y, vx: 0, vy: 0, held: false, rolling: false, spin: 0 }; fall = null; }

let drag = [], target = null;
canvas.addEventListener("pointerdown", (e) => {
  const p = toBoard(e);
  if (!playing || ball.rolling || fall || finished || p.y > PAD.h + 0.3) return;
  canvas.setPointerCapture(e.pointerId);
  ball.held = true; drag = [{ ...p, t: performance.now() }]; target = { x: ball.x, y: ball.y }; ball.grab = { x: ball.x - p.x, y: ball.y - p.y };
});
canvas.addEventListener("pointermove", (e) => {
  if (!ball.held) return;
  for (const ev of e.getCoalescedEvents?.() ?? [e]) {
    const p = toBoard(ev);
    drag.push({ x: p.x, y: p.y, t: ev.timeStamp || performance.now() });
    target = { x: Math.max(-(PAD.w / 2 - R), Math.min(PAD.w / 2 - R, p.x + ball.grab.x)), y: Math.max(R, Math.min(PAD.h - R, p.y + ball.grab.y)) };
  }
  const now = drag[drag.length - 1].t;
  while (drag.length > 2 && drag[0].t < now - 110) drag.shift();
  if (drag[drag.length - 1].y + ball.grab.y > PAD.h + 0.05) release(); // swiped past the start line: it's thrown
});
function release() {
  if (!ball.held) return;
  ball.held = false;
  const a = drag[0], b = drag[drag.length - 1], dt = Math.max(0.016, (b.t - a.t) / 1000);
  const fx = (b.x - a.x) / dt, fy = (b.y - a.y) / dt;
  if (fy < 0.4) return; // a tap or a pull back: no throw
  // Snap toward the track the swipe points at, aimed from where the ball is to that track's hole.
  const ang = Math.atan2(fx, fy);
  const t = TRACKS.reduce((best, tr) => (Math.abs(tr.a - ang) < Math.abs(best.a - ang) ? tr : best));
  const want = Math.atan2(t.hole[0] - target.x, t.hole[1] - target.y), dir = ang + (want - ang) * SNAP;
  const v = throwSpeed(Math.hypot(fx, fy)) * (t.a ? 0.93 : 1); // diagonals are a touch shorter
  Object.assign(ball, { x: target.x, y: target.y, vx: v * Math.sin(dir), vy: v * Math.cos(dir), rolling: true });
  path = [[ball.x, ball.y]]; pathFade = 1;
  tries++; started ??= performance.now(); updateHud();
}
canvas.addEventListener("pointerup", release);
canvas.addEventListener("pointercancel", release);

function step(dt) {
  if (!ball.rolling) return;
  for (let i = 0; i < TRACKS.length; i++) {
    if (filled[i]) continue;
    const [hx, hy] = TRACKS[i].hole, dx = hx - ball.x, dy = hy - ball.y, d = Math.hypot(dx, dy);
    if (d < HOLE_R) {
      ball.vx += dx / (d || 1) * DIP * dt; ball.vy += dy / (d || 1) * DIP * dt;
      if (Math.hypot(ball.vx, ball.vy) < CAPTURE && d < HOLE_R - R / 2) { filled[i] = true; return land(i); }
    }
  }
  const s = Math.hypot(ball.vx, ball.vy), ns = Math.max(0, s - ROLL_DECEL * dt);
  if (ns === 0) { ball.vx = ball.vy = 0; } else { ball.vx *= ns / s; ball.vy *= ns / s; }
  ball.x += ball.vx * dt; ball.y += ball.vy * dt; ball.spin += ns * dt / R; path.push([ball.x, ball.y]);
  // End wall on each track.
  for (const t of TRACKS) {
    const l = local(t, ball.x, ball.y), vin = ball.vx * t.ux + ball.vy * t.uy;
    if (Math.abs(l.across) <= TRACK_W / 2 && l.along > t.len - R && vin > 0) {
      ball.x -= t.ux * (l.along - (t.len - R)); ball.y -= t.uy * (l.along - (t.len - R));
      ball.vx -= (1 + WALL_BOUNCE) * vin * t.ux; ball.vy -= (1 + WALL_BOUNCE) * vin * t.uy;
      navigator.vibrate?.(20); burst(ball.x + t.ux * R, ball.y + t.uy * R, "#d9b37a", 6);
    }
  }
  if (!onBoard(ball.x, ball.y)) { ball.rolling = false; fall = { t: 0, x: ball.x, y: ball.y, vx: ball.vx * 0.3, vy: ball.vy * 0.3 }; navigator.vibrate?.(60); toast("Off the edge! Back to the start."); return; }
  if (ns === 0) { ball.rolling = false; fall = { t: 0, stopped: true }; toast(ball.y > PAD.h + 1.1 ? "Stopped short. Aim for an open hole." : "Too soft. Give it more."); }
}
function land(i) {
  ball.rolling = false;
  navigator.vibrate?.([30, 40, 30]);
  burst(...TRACKS[i].hole, "#ffb436", 24);
  updateHud();
  const n = filled.filter(Boolean).length;
  if (n === 3) {
    finished = performance.now();
    for (let k = 0; k < 80; k++) burst((Math.random() - 0.5) * BOARD.w, BOARD.h * Math.random(), ["#ffb436", "#e2572b", "#f5e1a4", "#6b8e3a"][k % 4], 1);
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
function updateHud() { $("tries").textContent = tries; [...$("balls").children].forEach((el, i) => el.classList.toggle("on", filled[i])); }

// A track as a rotated rectangle path, in screen px.
function trackPath(t, extra = 0) {
  const hw = TRACK_W / 2, px = t.uy, py = -t.ux, s0 = -0.15, s1 = t.len + extra;
  const pts = [[s0, -hw], [s1, -hw], [s1, hw], [s0, hw]].map(([a, c]) => [t.o[0] + t.ux * a + px * c, t.o[1] + t.uy * a + py * c]);
  ctx.moveTo(sx(pts[0][0]), sy(pts[0][1])); pts.slice(1).forEach(([x, y]) => ctx.lineTo(sx(x), sy(y))); ctx.closePath();
}
function boardPath() {
  ctx.beginPath();
  ctx.rect(sx(-PAD.w / 2), sy(PAD.h), PAD.w * scale, PAD.h * scale);
  TRACKS.forEach((t) => trackPath(t));
}
const t0 = performance.now();
const rnd = ((seed) => () => ((seed = (seed * 16807) % 2147483647) / 2147483647))(51); // fixed scenery
const PALMS = Array.from({ length: 7 }, () => ({ x: rnd(), y: rnd(), r: 0.6 + rnd() * 0.6, a: rnd() * 6 }));
const SHELLS = Array.from({ length: 26 }, () => ({ x: rnd(), y: rnd(), r: 1 + rnd() * 2.5 }));
const TORCHES = [[-0.62, 0.12], [0.62, 0.12]];

function scenery(time) {
  // wet sand fading into surf at the top
  const g = ctx.createLinearGradient(0, 0, 0, H);
  g.addColorStop(0, "#1f7d86"); g.addColorStop(0.16, "#3fa9a3"); g.addColorStop(0.22, "#d9c08d"); g.addColorStop(1, "#c7a46a");
  ctx.fillStyle = g; ctx.fillRect(0, 0, W, H);
  ctx.strokeStyle = "rgba(255,255,255,.55)"; ctx.lineWidth = 3;
  for (let k = 0; k < 2; k++) {
    const yy = H * (0.19 + k * 0.025) + Math.sin(time * 0.8 + k) * 6;
    ctx.beginPath(); for (let x = 0; x <= W; x += 16) ctx.lineTo(x, yy + Math.sin(x / 35 + time * 1.2 + k * 2) * 4); ctx.stroke();
  }
  ctx.fillStyle = "rgba(120,85,40,.25)";
  SHELLS.forEach((d) => { ctx.beginPath(); ctx.arc(d.x * W, H * 0.25 + d.y * H * 0.75, d.r, 0, 7); ctx.fill(); });
  // palm-frond shadows drifting in the breeze
  ctx.fillStyle = "rgba(40,50,20,.16)";
  PALMS.forEach((p) => {
    const px = p.x * W, py = H * 0.2 + p.y * H * 0.8, sway = Math.sin(time * 0.6 + p.a) * 0.05;
    for (let f = 0; f < 7; f++) {
      const ang = p.a + f * 0.9 + sway, len = 90 * p.r;
      ctx.save(); ctx.translate(px, py); ctx.rotate(ang);
      ctx.beginPath(); ctx.moveTo(0, 0); ctx.quadraticCurveTo(len * 0.5, -14 * p.r, len, 0); ctx.quadraticCurveTo(len * 0.5, 14 * p.r, 0, 0); ctx.fill();
      ctx.restore();
    }
  });
}
// Bamboo canes laid along a track, with node bands across, clipped to the track.
function bamboo(t) {
  ctx.save(); ctx.beginPath(); trackPath(t); ctx.clip();
  const n = 5, cw = TRACK_W / n, px = t.uy, py = -t.ux;
  for (let i = 0; i < n; i++) {
    const c = -TRACK_W / 2 + cw * (i + 0.5), x0 = t.o[0] + px * c, y0 = t.o[1] + py * c;
    const a0 = [x0 - t.ux * 0.2, y0 - t.uy * 0.2], a1 = [x0 + t.ux * (t.len + 0.1), y0 + t.uy * (t.len + 0.1)];
    ctx.lineCap = "butt";
    ctx.strokeStyle = i % 2 ? "#b49a4c" : "#c4ab5a"; ctx.lineWidth = cw * scale;
    ctx.beginPath(); ctx.moveTo(sx(a0[0]), sy(a0[1])); ctx.lineTo(sx(a1[0]), sy(a1[1])); ctx.stroke();
    ctx.strokeStyle = "rgba(255,250,210,.35)"; ctx.lineWidth = cw * scale * 0.18; // shine
    ctx.beginPath(); ctx.moveTo(sx(a0[0] - px * cw * 0.2), sy(a0[1] - py * cw * 0.2)); ctx.lineTo(sx(a1[0] - px * cw * 0.2), sy(a1[1] - py * cw * 0.2)); ctx.stroke();
    ctx.strokeStyle = "rgba(70,50,15,.45)"; ctx.lineWidth = 1.5; // seam
    ctx.beginPath(); ctx.moveTo(sx(a0[0] + px * cw / 2), sy(a0[1] + py * cw / 2)); ctx.lineTo(sx(a1[0] + px * cw / 2), sy(a1[1] + py * cw / 2)); ctx.stroke();
    for (let s = 0.15 + (i % 2) * 0.18; s < t.len; s += 0.36) { // nodes, staggered
      const nx = x0 + t.ux * s, ny = y0 + t.uy * s;
      ctx.strokeStyle = "rgba(90,65,20,.55)"; ctx.lineWidth = 2.5;
      ctx.beginPath(); ctx.moveTo(sx(nx - px * cw / 2), sy(ny - py * cw / 2)); ctx.lineTo(sx(nx + px * cw / 2), sy(ny + py * cw / 2)); ctx.stroke();
    }
  }
  ctx.restore();
}
function rope(x0, y0, x1, y1, w) { // a twisted lashing between two board points
  const L = Math.hypot(x1 - x0, y1 - y0) * scale, a = Math.atan2(sy(y1) - sy(y0), sx(x1) - sx(x0));
  ctx.save(); ctx.translate(sx(x0), sy(y0)); ctx.rotate(a);
  ctx.fillStyle = "#8a6a3a"; ctx.fillRect(0, -w / 2, L, w);
  ctx.strokeStyle = "#5a4020"; ctx.lineWidth = 1.5;
  for (let x = 0; x < L; x += w * 0.6) { ctx.beginPath(); ctx.moveTo(x, -w / 2); ctx.lineTo(x + w * 0.5, w / 2); ctx.stroke(); }
  ctx.restore();
}
function lashing(t, s) {
  const hw = TRACK_W / 2 + 0.01, px = t.uy, py = -t.ux, x = t.o[0] + t.ux * s, y = t.o[1] + t.uy * s;
  rope(x - px * hw, y - py * hw, x + px * hw, y + py * hw, 0.035 * scale);
}
function mat() { // woven palm mat start pad
  const x0 = sx(-PAD.w / 2), y0 = sy(PAD.h), w = PAD.w * scale, h = PAD.h * scale, k = 0.08 * scale;
  ctx.save(); ctx.beginPath(); ctx.rect(x0, y0, w, h); ctx.clip();
  ctx.fillStyle = "#a8834a"; ctx.fillRect(x0, y0, w, h);
  for (let i = 0; i * k < w; i++) for (let j = 0; j * k < h; j++) {
    ctx.fillStyle = (i + j) % 2 ? "rgba(255,230,170,.18)" : "rgba(80,50,15,.18)";
    ctx.fillRect(x0 + i * k + 1, y0 + j * k + 1, k - 2, k - 2);
  }
  ctx.restore();
  ctx.strokeStyle = "#6b4a22"; ctx.lineWidth = 4; ctx.strokeRect(x0, y0, w, h);
}
function torch(x, y, time, i) {
  const px = sx(x), py = sy(y), u = scale * 0.05;
  ctx.fillStyle = "rgba(0,0,0,.3)"; ctx.beginPath(); ctx.ellipse(px + u, py + u * 0.6, u * 1.4, u * 0.7, 0, 0, 7); ctx.fill();
  ctx.fillStyle = "#4a3018"; ctx.beginPath(); ctx.arc(px, py, u * 0.9, 0, 7); ctx.fill(); // bamboo pole top-down
  ctx.strokeStyle = "#2a1a0a"; ctx.lineWidth = 2; ctx.stroke();
  const f = 1 + Math.sin(time * 13 + i * 3) * 0.12 + Math.sin(time * 7.3 + i) * 0.08;
  const glow = ctx.createRadialGradient(px, py, 0, px, py, u * 6 * f);
  glow.addColorStop(0, "rgba(255,190,80,.55)"); glow.addColorStop(1, "rgba(255,120,20,0)");
  ctx.fillStyle = glow; ctx.beginPath(); ctx.arc(px, py, u * 6 * f, 0, 7); ctx.fill();
  const fl = ctx.createRadialGradient(px, py - u * 0.3, 0, px, py, u * 1.3 * f);
  fl.addColorStop(0, "#fff6c0"); fl.addColorStop(0.4, "#ffb436"); fl.addColorStop(1, "rgba(220,60,10,.0)");
  ctx.fillStyle = fl; ctx.beginPath(); ctx.arc(px, py, u * 1.3 * f, 0, 7); ctx.fill();
}

function draw(now) {
  const time = (now - t0) / 1000;
  ctx.clearRect(0, 0, W, H);
  scenery(time);
  // board shadow and raised edge
  ctx.save(); ctx.translate(8, 14); boardPath(); ctx.fillStyle = "rgba(60,40,10,.35)"; ctx.filter = "blur(8px)"; ctx.fill(); ctx.restore();
  ctx.save(); ctx.translate(0, 6); boardPath(); ctx.fillStyle = "#6b5220"; ctx.fill(); ctx.restore();
  TRACKS.forEach(bamboo);
  mat();
  TRACKS.forEach((t) => lashing(t, t.len * 0.6));
  // end posts: a thick cross log on each track
  TRACKS.forEach((t) => {
    const hw = TRACK_W / 2 + 0.03, e = t.o.map((o, k) => o + [t.ux, t.uy][k] * (t.len + 0.02));
    ctx.lineCap = "round"; ctx.strokeStyle = "#5b3a1a"; ctx.lineWidth = 0.075 * scale;
    ctx.beginPath(); ctx.moveTo(sx(e[0] + t.uy * hw), sy(e[1] - t.ux * hw)); ctx.lineTo(sx(e[0] - t.uy * hw), sy(e[1] + t.ux * hw)); ctx.stroke();
    ctx.strokeStyle = "rgba(255,220,160,.25)"; ctx.lineWidth = 0.02 * scale; ctx.stroke();
  });
  // start line: a rope across the mat
  rope(-PAD.w / 2 + 0.04, PAD.h - 0.05, PAD.w / 2 - 0.04, PAD.h - 0.05, 0.025 * scale);
  // holes: dug into the bamboo, ringed with rope, flagged with a flickering glow until filled
  TRACKS.forEach((t, i) => {
    const [x, y] = t.hole;
    const g = ctx.createRadialGradient(sx(x), sy(y) - 4, 2, sx(x), sy(y), HOLE_R * scale);
    g.addColorStop(0, "#0d0702"); g.addColorStop(0.75, "#24150a"); g.addColorStop(1, "#5a3c18");
    ctx.fillStyle = g; ctx.beginPath(); ctx.arc(sx(x), sy(y), HOLE_R * scale, 0, 7); ctx.fill();
    ctx.setLineDash([5, 3]); ctx.strokeStyle = "#8a6a3a"; ctx.lineWidth = 5;
    ctx.beginPath(); ctx.arc(sx(x), sy(y), HOLE_R * scale + 3, 0, 7); ctx.stroke(); ctx.setLineDash([]);
    if (!filled[i]) {
      ctx.strokeStyle = `rgba(255,170,60,${0.3 + 0.25 * Math.sin(time * 3 + i)})`; ctx.lineWidth = 3;
      ctx.beginPath(); ctx.arc(sx(x), sy(y), HOLE_R * scale + 8, 0, 7); ctx.stroke();
    } else drawBall(x, y, R * 0.92, 0, 0.85);
  });
  TORCHES.forEach(([x, y], i) => torch(x, y, time, i));
  if (path.length > 1 && pathFade > 0) { // where the last throw went: footprints in the dust
    ctx.strokeStyle = `rgba(70,40,10,${0.4 * pathFade})`; ctx.lineWidth = 4; ctx.lineCap = "round"; ctx.lineJoin = "round"; ctx.setLineDash([1, 9]);
    ctx.beginPath(); path.forEach(([x, y], i) => (i ? ctx.lineTo(sx(x), sy(y)) : ctx.moveTo(sx(x), sy(y)))); ctx.stroke(); ctx.setLineDash([]);
  }
  if (!finished) {
    if (fall && !fall.stopped) {
      const k = Math.max(0, 1 - fall.t / 0.55), fx = fall.x + fall.vx * fall.t, fy = fall.y + fall.vy * fall.t;
      drawBall(fx, fy, R * (0.35 + 0.65 * k), ball.spin, k);
      if (fall.t > 0.4 && !fall.splashed) { fall.splashed = true; burst(fx, fy, "#e6cf9a", 16); } // sand puff
    } else drawBall(ball.x, ball.y, ball.held ? R * 1.06 : R, ball.spin, fall?.stopped ? Math.max(0, 1 - fall.t / 0.6) : 1, ball.held);
  }
  particles.forEach((p) => { ctx.globalAlpha = Math.max(0, p.life); ctx.fillStyle = p.color; ctx.beginPath(); ctx.arc(sx(p.x), sy(p.y), 3, 0, 7); ctx.fill(); });
  ctx.globalAlpha = 1;
  $("time").textContent = started ? fmt((finished ?? performance.now()) - started) : "0:00.0";
}
// A coconut-shell ball: dark husk with fibres, three eyes that rotate as it rolls.
function drawBall(x, y, r, spin, alpha = 1, lifted = false) {
  const px = sx(x), py = sy(y), rr = r * scale;
  ctx.globalAlpha = alpha;
  ctx.fillStyle = "rgba(40,25,5,.4)"; ctx.beginPath(); ctx.ellipse(px + rr * 0.3, py + rr * (lifted ? 0.7 : 0.35), rr * 1.05, rr * 0.85, 0, 0, 7); ctx.fill();
  const g = ctx.createRadialGradient(px - rr * 0.35, py - rr * 0.4, rr * 0.1, px, py, rr);
  g.addColorStop(0, "#b07a45"); g.addColorStop(0.5, "#6e4522"); g.addColorStop(1, "#3a2210");
  ctx.fillStyle = g; ctx.beginPath(); ctx.arc(px, py, rr, 0, 7); ctx.fill();
  ctx.save(); ctx.beginPath(); ctx.arc(px, py, rr, 0, 7); ctx.clip();
  ctx.strokeStyle = "rgba(200,150,90,.35)"; ctx.lineWidth = 1;
  for (let i = 0; i < 9; i++) { const a = spin * 0.5 + i * 0.7; ctx.beginPath(); ctx.moveTo(px + Math.cos(a) * rr, py + Math.sin(a) * rr); ctx.lineTo(px + Math.cos(a + 2.4) * rr, py + Math.sin(a + 2.4) * rr); ctx.stroke(); }
  const d = Math.sin(spin) * rr * 0.6; // the eyes roll past
  ctx.fillStyle = "#24140a";
  [[-0.22, 0], [0.22, 0], [0, 0.3]].forEach(([ex, ey]) => { ctx.beginPath(); ctx.arc(px + ex * rr, py + ey * rr + d, rr * 0.12, 0, 7); ctx.fill(); });
  ctx.restore();
  ctx.globalAlpha = 1;
}

let last = performance.now();
function frame(now) {
  const dt = Math.min(0.05, (now - last) / 1000); last = now;
  if (ball.held && target) { const k = 1 - Math.exp(-dt * 28); ball.x += (target.x - ball.x) * k; ball.y += (target.y - ball.y) * k; pathFade = Math.max(0, pathFade - dt * 3); }
  for (let i = 0; i < 8; i++) step(dt / 8); // substeps so a fast ball can't skip a hole
  if (fall && (fall.t += dt) > (fall.stopped ? 0.6 : 0.8)) newBall();
  particles.forEach((p) => { p.x += p.vx * dt; p.y += p.vy * dt; p.vx *= 0.96; p.vy *= 0.96; p.life -= dt; });
  particles = particles.filter((p) => p.life > 0);
  draw(now);
  requestAnimationFrame(frame);
}

$("go").onclick = () => { $("intro").classList.add("hidden"); playing = true; toast("Swipe up toward a track."); };
$("again").onclick = () => { $("done").classList.add("hidden"); reset(); };
resize(); reset(); requestAnimationFrame(frame);
}
