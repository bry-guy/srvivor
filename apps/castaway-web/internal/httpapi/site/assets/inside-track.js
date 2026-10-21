{ // own block: page scripts share one global scope
// Inside Track (prototype), top-down. A rack of 10 balls feeds through a gate onto the start pad; flick each
// one up a foam track and butter it into a shallow cup. Every ball is simulated all the time, on the table,
// in the air and on the ground, so balls knock into each other, rim out, fall off and stay where they land.
const BOARD = { w: 2.1, h: 2.75, y0: -0.12, xc: -0.1 }; // xc: the rack makes the table lopsided
const R = 0.085, CUP_R = 0.8 * R; // the cup is smaller than the ball: it sits in the lip
const PAD = { x0: -0.5, x1: 0.5, y0: 0, y1: 0.67 };
const CH = 2 * R + 0.03;                       // chute width
const COL = { x0: -1.06, x1: -1.06 + CH, top: 1.75 }; // the rack's upright column
const TRACK_W = 0.36, ANG = 30 * Math.PI / 180;
const COLORS = ["#c8372d", "#e8b923", "#2f5fa8"];
const TRACKS = [-ANG, 0, ANG].map((a, i) => {
  const len = a < 0 ? 1.45 : a > 0 ? 1.09 : 1.72, // blue a quarter shorter than red
    ux = Math.sin(a), uy = Math.cos(a), o = [0, PAD.y1];
  return { a, len, ux, uy, o, color: COLORS[i], cup: [o[0] + ux * (len - 0.17), o[1] + uy * (len - 0.17)] };
});
const TABLE_H = 0.6, G = 9.8;
const ROLL = (v) => 0.65 + 0.11 * v;  // foam rolling resistance
const SAND = 6;                      // sand stops a ball fast
const WALL_E = 0.22, BALL_E = 0.65;   // heavy ball: walls soak up speed
const ROLL_G = 7; // (5/7)·g: gravity along a slope for a rolling ball
const THROW_K = 0.55, MAX_V = 3.0, SNAP = 0.3;
const RACK_N = 12, DEBUG = location.search.includes("debug");

const local = (t, x, y) => { const dx = x - t.o[0], dy = y - t.o[1]; return { along: dx * t.ux + dy * t.uy, across: dx * t.uy - dy * t.ux }; };
const blocked = (x, y) => [[0, 0], [R, 0], [-R, 0], [0, R], [0, -R]].some(([ox, oy]) => onTable(x + ox, y + oy));
const inRect = (x, y, x0, x1, y0, y1) => x >= x0 && x <= x1 && y >= y0 && y <= y1;
function onTable(x, y) {
  if (inRect(x, y, PAD.x0, PAD.x1, PAD.y0, PAD.y1) || inRect(x, y, COL.x1, PAD.x0, 0, CH) || inRect(x, y, COL.x0, COL.x1, 0, COL.top)) return true;
  return TRACKS.some((t) => { const l = local(t, x, y); return l.along >= -0.15 && l.along <= t.len && Math.abs(l.across) <= TRACK_W / 2; });
}
// Table-level walls: [ax, ay, bx, by, kind].
const WALLS = [
  [COL.x0, 0, PAD.x1, 0], [PAD.x1, 0, PAD.x1, PAD.y1, "stop"],
  [COL.x0, 0, COL.x0, COL.top], [COL.x1, CH, COL.x1, COL.top], [COL.x0, COL.top, COL.x1, COL.top], [COL.x1, CH, PAD.x0, CH],
  [PAD.x0, 0, PAD.x0, CH, "gate"],
]; // the track ends are open: overshoot and it drops off

const $ = (id) => document.getElementById(id);
const canvas = $("c"), ctx = canvas.getContext("2d");
let scale = 1, cx = 0, cy = 0, W = 0, H = 0, sand = null;
function resize() {
  const d = devicePixelRatio || 1;
  W = innerWidth; H = innerHeight; canvas.width = W * d; canvas.height = H * d;
  scale = Math.min(W / BOARD.w, (H - 50) / BOARD.h) * 0.95;
  cx = W / 2 - BOARD.xc * scale; cy = H / 2 + 22 + (BOARD.y0 + BOARD.h / 2) * scale;
  ctx.setTransform(d, 0, 0, d, 0, 0); sand = null;
}
addEventListener("resize", resize);
const sx = (x) => cx + x * scale, sy = (y) => cy - y * scale;
const toBoard = (e) => ({ x: (e.clientX - cx) / scale, y: (cy - e.clientY) / scale });
const view = () => ({ x0: -cx / scale + R, x1: (W - cx) / scale - R, y0: (cy - H) / scale + R, y1: cy / scale - R });

// Ball: { x, y, vx, vy, h (height above ground), vh, level: rack|table|air|ground|cup, spin }
let balls, cur, cups, gate, wait, done, playing = false, path = [];
function reset() {
  balls = []; cups = [null, null, null]; cur = null; done = false; path = []; gate = 0; wait = 0.6;
  for (let i = 0; i < RACK_N; i++) {
    const inRow = i < 3, x = inRow ? PAD.x0 - R - 0.003 - i * 2 * R : (COL.x0 + COL.x1) / 2, y = inRow ? CH / 2 : CH / 2 + 0.02 + (i - 2) * 2 * R;
    balls.push({ x, y, vx: 0, vy: 0, h: TABLE_H, vh: 0, level: "rack", spin: 0, ang: 0 });
  }
  $("done").classList.add("hidden"); hud();
}
const speed = (b) => Math.hypot(b.vx, b.vy);
const used = () => balls.filter((b) => b.level !== "rack").length;
const cupCount = () => cups.filter(Boolean).length;
function hud() { $("left").textContent = RACK_N - used(); $("cups").textContent = `${cupCount()}/3`; }

function release() { // gate opens, front ball rolls out across the pad toward the stop wall
  const front = balls.filter((b) => b.level === "rack" && b.y < CH).sort((a, b) => b.x - a.x)[0];
  if (!front) return;
  cur = front; cur.level = "table"; cur.thrown = false; cur.vx = 1.8; gate = 0.35; hud();
}

// ---- input ----
let drag = [], target = null, held = false, grab = { x: 0, y: 0 };
const ready = () => cur && cur.level === "table" && cur.onPad && !cur.thrown && inRect(cur.x, cur.y, PAD.x0, PAD.x1, PAD.y0, PAD.y1);
canvas.addEventListener("pointerdown", (e) => {
  const p = toBoard(e);
  if (!playing || done || !ready() || Math.hypot(p.x - cur.x, p.y - cur.y) > 0.3) return;
  canvas.setPointerCapture(e.pointerId);
  held = true; drag = [{ ...p, t: e.timeStamp }]; grab = { x: cur.x - p.x, y: cur.y - p.y }; target = { x: cur.x, y: cur.y };
});
canvas.addEventListener("pointermove", (e) => {
  if (!held) return;
  for (const ev of e.getCoalescedEvents?.() ?? [e]) {
    const p = toBoard(ev);
    drag.push({ x: p.x, y: p.y, t: ev.timeStamp });
    target = { x: Math.max(PAD.x0 + R, Math.min(PAD.x1 - R, p.x + grab.x)), y: Math.max(R, Math.min(PAD.y1 - R, p.y + grab.y)) };
  }
  const now = drag[drag.length - 1].t;
  while (drag.length > 2 && drag[0].t < now - 110) drag.shift();
  if (drag[drag.length - 1].y + grab.y > PAD.y1 + 0.04) letGo();
});
function letGo() {
  if (!held) return;
  held = false;
  const a = drag[0], b = drag[drag.length - 1], dt = Math.max(0.016, (b.t - a.t) / 1000);
  const fx = (b.x - a.x) / dt, fy = (b.y - a.y) / dt;
  if (fy < 0.4) return; // a tap or a pull back: no throw
  const ang = Math.atan2(fx, fy), t = TRACKS.reduce((best, tr) => (Math.abs(tr.a - ang) < Math.abs(best.a - ang) ? tr : best));
  const dir = ang + (Math.atan2(t.cup[0] - cur.x, t.cup[1] - cur.y) - ang) * SNAP;
  let v = THROW_K * Math.hypot(fx, fy); v = MAX_V * Math.tanh(v / MAX_V); // linear, soft-clamped: speed control is the skill
  throwBall(v * Math.sin(dir), v * Math.cos(dir));
}
function throwBall(vx, vy) { Object.assign(cur, { vx, vy }); path = [[cur.x, cur.y]]; }
canvas.addEventListener("pointerup", letGo);
canvas.addEventListener("pointercancel", letGo);

// ---- physics (fixed 240 Hz) ----
function wall(b, [ax, ay, bx, by, kind]) {
  if (kind === "gate" && (b === cur && b.level === "table" && gate > 0)) return;
  const ex = bx - ax, ey = by - ay, t = Math.max(0, Math.min(1, ((b.x - ax) * ex + (b.y - ay) * ey) / (ex * ex + ey * ey)));
  const px = ax + ex * t, py = ay + ey * t, dx = b.x - px, dy = b.y - py, d = Math.hypot(dx, dy);
  if (d >= R || d === 0) return;
  const nx = dx / d, ny = dy / d, vn = b.vx * nx + b.vy * ny;
  b.x += nx * (R - d); b.y += ny * (R - d);
  if (vn < 0) {
    const tx = b.vx - vn * nx, ty = b.vy - vn * ny;
    b.vx = tx * 0.9 - vn * WALL_E * nx; b.vy = ty * 0.9 - vn * WALL_E * ny;
    if (-vn > 0.8 && b === cur) navigator.vibrate?.(15);
  }
}
function collide(a, b) {
  const dx = b.x - a.x, dy = b.y - a.y, d = Math.hypot(dx, dy);
  if (d >= 2 * R || d === 0) return;
  const nx = dx / d, ny = dy / d, fixedA = a.level === "cup", fixedB = b.level === "cup";
  if (fixedA && fixedB) return;
  const push = 2 * R - d;
  if (fixedA) { b.x += nx * push; b.y += ny * push; } else if (fixedB) { a.x -= nx * push; a.y -= ny * push; } else { a.x -= nx * push / 2; a.y -= ny * push / 2; b.x += nx * push / 2; b.y += ny * push / 2; }
  const vn = (b.vx - a.vx) * nx + (b.vy - a.vy) * ny;
  if (vn >= 0) return;
  if (fixedA) { b.vx -= (1 + BALL_E) * vn * nx; b.vy -= (1 + BALL_E) * vn * ny; return; }
  if (fixedB) { a.vx += (1 + BALL_E) * vn * nx; a.vy += (1 + BALL_E) * vn * ny; return; }
  const j = (1 + BALL_E) * vn / 2;
  a.vx += j * nx; a.vy += j * ny; b.vx -= j * nx; b.vy -= j * ny;
}
function friction(b, decel, dt) {
  const s = speed(b), ns = Math.max(0, s - decel * dt);
  if (ns === 0) { b.vx = b.vy = 0; } else { b.vx *= ns / s; b.vy *= ns / s; }
  if (ns > 0.05) { b.spin += ns * dt / R; b.ang = Math.atan2(b.vy, b.vx); }
}
function step(dt) {
  if (gate > 0) gate -= dt;
  for (const b of balls) {
    if (b === cur && held) { b.vx = b.vy = 0; continue; }
    if (b.level === "cup") { if (b.sink < 1) { b.sink = Math.min(1, b.sink + dt / 0.15); b.x += (b.cx - b.x) * 0.2; b.y += (b.cy - b.y) * 0.2; } continue; }
    if (b.level === "rack") { // the rack is tilted toward the gate
      if (b.x < COL.x1) b.vy -= 2 * dt; else b.vx += 2 * dt;
      if (b.x < COL.x1 && b.y < CH) b.vx += 2 * dt;
      friction(b, 0.5, dt);
    } else if (b.level === "table") {
      // Over the cup the ball rests on the lip, its centre lower the nearer it is to the middle:
      // height = sqrt(R² − (CUP_R − d)²). Rolling down that slope is the only pull, and it is zero at the lip,
      // so a ball keeps going unless it is slow enough not to climb back out.
      for (let i = 0; i < 3; i++) {
        if (cups[i]) continue;
        const [hx, hy] = TRACKS[i].cup, dx = hx - b.x, dy = hy - b.y, d = Math.hypot(dx, dy);
        if (d >= CUP_R) continue;
        const q = CUP_R - d, slope = q / Math.sqrt(R * R - q * q);
        b.vx += (dx / (d || 1)) * ROLL_G * slope * dt; b.vy += (dy / (d || 1)) * ROLL_G * slope * dt;
        b.vx *= 1 - 2 * dt; b.vy *= 1 - 2 * dt; // knocking against the lip
        if (d < CUP_R * 0.3 && speed(b) < 0.08) { cups[i] = b; Object.assign(b, { level: "cup", cx: hx, cy: hy, sink: 0, vx: 0, vy: 0 }); if (b === cur) navigator.vibrate?.([30, 40, 30]); hud(); break; }
      }
      if (b.level !== "table") continue;
      friction(b, ROLL(speed(b)), dt);
    } else if (b.level === "air") {
      b.vh -= G * dt; b.h += b.vh * dt;
      if (b.h <= 0) {
        b.h = 0;
        if (b.vh < -0.8) { b.vh = -b.vh * 0.25; b.vx *= 0.6; b.vy *= 0.6; } else { b.vh = 0; b.level = "ground"; }
      }
    } else if (b.level === "ground") friction(b, SAND, dt);
    const px = b.x, py = b.y;
    b.x += b.vx * dt; b.y += b.vy * dt;
    if (b === cur && b.thrown && b.level !== "ground") path.push([b.x, b.y]);
    if (b.level === "table" || b.level === "rack") {
      WALLS.forEach((w) => wall(b, w));
      if (!onTable(b.x, b.y)) { b.level = "air"; b.vh = 0; navigator.vibrate?.(40); }
    } else { // ground and air: the table is a solid block to them; the screen edge is a wall
      const below = b.h < TABLE_H;
      if (below && blocked(b.x, b.y) && !blocked(px, py)) {
        b.x = px; b.y = py; b.vx *= -0.2; b.vy *= -0.2;
      }
      const v = view();
      if (b.x < v.x0 || b.x > v.x1) { b.x = Math.max(v.x0, Math.min(v.x1, b.x)); b.vx *= -WALL_E; }
      if (b.y < v.y0 || b.y > v.y1) { b.y = Math.max(v.y0, Math.min(v.y1, b.y)); b.vy *= -WALL_E; }
    }
  }
  // Nothing below table height may sit inside the table's footprint: push it out to the nearest clear spot.
  for (const b of balls) if ((b.level === "ground" || b.level === "air") && b.h < TABLE_H * 0.9 && blocked(b.x, b.y)) {
    out: for (let d = 0.01; d < 0.6; d += 0.01) for (let k = 0; k < 16; k++) {
      const a = (k / 16) * Math.PI * 2, x = b.x + Math.cos(a) * d, y = b.y + Math.sin(a) * d;
      if (!blocked(x, y)) { b.x = x; b.y = y; b.vx *= 0.3; b.vy *= 0.3; break out; }
    }
  }
  for (let i = 0; i < balls.length; i++) for (let j = i + 1; j < balls.length; j++) {
    const a = balls[i], b = balls[j], la = a.level === "cup" ? "table" : a.level, lb = b.level === "cup" ? "table" : b.level;
    if (la === lb && la !== "air" || (la === "rack" && lb === "table") || (la === "table" && lb === "rack")) collide(a, b);
  }
  if (cur && !cur.onPad && inRect(cur.x, cur.y, PAD.x0 + R, PAD.x1, PAD.y0, PAD.y1)) cur.onPad = true;
  if (cur && cur.onPad && !cur.thrown && !held && !inRect(cur.x, cur.y, PAD.x0, PAD.x1, PAD.y0, PAD.y1)) cur.thrown = true; // off the mat: it counts
  // The current ball is done once it sits in a cup, stops on the ground, or stops on the table after a throw.
  if (cur && cur.thrown && !held) {
    const settled = cur.level === "cup" || (cur.level === "ground" && speed(cur) === 0) || (cur.level === "table" && speed(cur) < 0.02);
    if (settled) {
      cur = null; wait = 0.6;
    }
  }
  if (!cur && !done) {
    if (cupCount() === 3 || !balls.some((b) => b.level === "rack")) { done = true; finish(); }
    else if ((wait -= dt) <= 0) release();
  }
}
function finish() {
  setTimeout(() => { $("final").textContent = `${cupCount()} of 3 cups`; $("finalRolls").textContent = `${used()} of ${RACK_N} balls used`; $("done").classList.remove("hidden"); }, 700);
}

// ---- drawing: flat, matte, one light from the top left ----
const INK = "#2b2418";
const SUN = { x: 0.55, y: 0.75 }; // shadow offset per unit of height, in screen directions (down-right)
const shadowAt = (h) => [SUN.x * h * scale, SUN.y * h * scale];
function sandPattern() {
  if (sand) return sand;
  const c = document.createElement("canvas"); c.width = c.height = 128;
  const g = c.getContext("2d"); g.fillStyle = "#dcc596"; g.fillRect(0, 0, 128, 128);
  for (let i = 0; i < 700; i++) { g.fillStyle = Math.random() < 0.5 ? "rgba(110,80,40,.09)" : "rgba(255,248,230,.14)"; g.fillRect(Math.random() * 128, Math.random() * 128, 1, 1); }
  return (sand = ctx.createPattern(c, "repeat"));
}
const rnd = ((seed) => () => ((seed = (seed * 16807) % 2147483647) / 2147483647))(51);
const PALMS = Array.from({ length: 4 }, (_, i) => ({ x: [0.08, 0.92, 0.1, 0.88][i], y: [0.35, 0.55, 0.85, 0.95][i], a: rnd() * 6 }));
function beach(time) {
  ctx.fillStyle = sandPattern(); ctx.fillRect(0, 0, W, H);
  const shore = H * 0.13;
  ctx.fillStyle = "#c9ae7c"; ctx.fillRect(0, 0, W, shore + 18); // wet sand
  ctx.fillStyle = "#3f9a9c"; ctx.beginPath(); ctx.moveTo(0, 0); ctx.lineTo(W, 0);
  for (let x = W; x >= 0; x -= 12) ctx.lineTo(x, shore + Math.sin(x / 45 + time * 0.9) * 5);
  ctx.fill();
  ctx.fillStyle = "#2a7f86"; ctx.fillRect(0, 0, W, shore * 0.45); // deeper water
  ctx.strokeStyle = "#f3ead2"; ctx.lineWidth = 3; ctx.beginPath();
  for (let x = 0; x <= W; x += 12) ctx.lineTo(x, shore + Math.sin(x / 45 + time * 0.9) * 5);
  ctx.stroke();
  ctx.fillStyle = "rgba(70,55,25,.14)"; // palm shadows, cast down-right like everything else
  PALMS.forEach((p) => {
    const px = p.x * W, py = p.y * H;
    for (let f = 0; f < 6; f++) {
      ctx.save(); ctx.translate(px, py); ctx.rotate(p.a + f * 1.05 + Math.sin(time * 0.5 + p.a) * 0.03);
      ctx.beginPath(); ctx.moveTo(0, 0); ctx.quadraticCurveTo(40, -9, 80, 0); ctx.quadraticCurveTo(40, 9, 0, 0); ctx.fill(); ctx.restore();
    }
  });
}
function poly(pts) { ctx.moveTo(sx(pts[0][0]), sy(pts[0][1])); pts.slice(1).forEach(([x, y]) => ctx.lineTo(sx(x), sy(y))); ctx.closePath(); }
const rectPts = (x0, x1, y0, y1) => [[x0, y0], [x1, y0], [x1, y1], [x0, y1]];
function trackPts(t) {
  const h = TRACK_W / 2, px = t.uy, py = -t.ux;
  return [[-0.15, -h], [t.len, -h], [t.len, h], [-0.15, h]].map(([a, c]) => [t.o[0] + t.ux * a + px * c, t.o[1] + t.uy * a + py * c]);
}
const SHAPES = () => [
  { pts: rectPts(COL.x0, COL.x1, 0, COL.top), color: "#7a5230" }, { pts: rectPts(COL.x1, PAD.x0, 0, CH), color: "#7a5230" },
  ...[TRACKS[0], TRACKS[2], TRACKS[1]].map((t) => ({ pts: trackPts(t), color: t.color })),
  { pts: rectPts(PAD.x0, PAD.x1, PAD.y0, PAD.y1), color: "#b98a55", wood: true },
];
function planks(pts) { // flat plank seams and a few grain strokes
  const x0 = sx(pts[0][0]), x1 = sx(pts[1][0]), y1 = sy(pts[0][1]), y0 = sy(pts[2][1]), n = 5, h = (y1 - y0) / n;
  ctx.save(); ctx.beginPath(); ctx.rect(x0, y0, x1 - x0, y1 - y0); ctx.clip();
  for (let i = 0; i < n; i++) {
    ctx.fillStyle = i % 2 ? "rgba(255,230,190,.06)" : "rgba(80,45,15,.06)"; ctx.fillRect(x0, y0 + i * h, x1 - x0, h);
    ctx.strokeStyle = "rgba(70,40,15,.45)"; ctx.lineWidth = 1.5; ctx.beginPath(); ctx.moveTo(x0, y0 + i * h); ctx.lineTo(x1, y0 + i * h); ctx.stroke();
    ctx.strokeStyle = "rgba(90,55,20,.18)"; ctx.lineWidth = 1;
    for (let k = 0; k < 3; k++) { const yy = y0 + i * h + h * (0.25 + k * 0.25); ctx.beginPath(); ctx.moveTo(x0, yy); ctx.bezierCurveTo(x0 + (x1 - x0) * 0.3, yy - 2, x0 + (x1 - x0) * 0.6, yy + 2, x1, yy); ctx.stroke(); }
    const butt = x0 + (x1 - x0) * ((i * 0.37) % 1); ctx.strokeStyle = "rgba(70,40,15,.4)"; ctx.beginPath(); ctx.moveTo(butt, y0 + i * h); ctx.lineTo(butt, y0 + (i + 1) * h); ctx.stroke();
  }
  ctx.restore();
}
const TORCHES = [[PAD.x0 - 0.14, PAD.y1 + 0.02], [PAD.x1 + 0.14, PAD.y1 + 0.02]];
function torch([x, y], time, i) { // seen from above: a bamboo pole top with a flame; pole shadow falls with the sun
  const px = sx(x), py = sy(y), u = 0.045 * scale, [ox, oy] = shadowAt(0.5);
  ctx.strokeStyle = "rgba(60,40,15,.28)"; ctx.lineWidth = u * 0.9; ctx.lineCap = "round";
  ctx.beginPath(); ctx.moveTo(px, py); ctx.lineTo(px + ox, py + oy); ctx.stroke();
  const f = 1 + Math.sin(time * 11 + i * 3) * 0.1 + Math.sin(time * 6.7 + i) * 0.07;
  const glow = ctx.createRadialGradient(px, py, 0, px, py, u * 5 * f);
  glow.addColorStop(0, "rgba(255,170,60,.35)"); glow.addColorStop(1, "rgba(255,140,40,0)");
  ctx.fillStyle = glow; ctx.beginPath(); ctx.arc(px, py, u * 5 * f, 0, 7); ctx.fill();
  ctx.fillStyle = "#5a3a1c"; ctx.beginPath(); ctx.arc(px, py, u, 0, 7); ctx.fill();
  ctx.strokeStyle = INK; ctx.lineWidth = 1.5; ctx.stroke();
  ctx.fillStyle = "#e8742a"; ctx.beginPath(); ctx.arc(px, py, u * 0.7 * f, 0, 7); ctx.fill();
  ctx.fillStyle = "#ffd36a"; ctx.beginPath(); ctx.arc(px - u * 0.1, py - u * 0.1, u * 0.35 * f, 0, 7); ctx.fill();
}
function drawTable() {
  const [ox, oy] = shadowAt(TABLE_H * 0.12);
  ctx.save(); ctx.translate(ox, oy); ctx.fillStyle = "rgba(60,40,15,.28)"; ctx.beginPath(); SHAPES().forEach((s) => poly(s.pts)); ctx.fill(); ctx.restore();
  for (const s of SHAPES()) {
    ctx.beginPath(); poly(s.pts); ctx.fillStyle = s.color; ctx.fill();
    if (s.wood) planks(s.pts);
    ctx.save(); ctx.clip(); ctx.lineWidth = 5; // soft foam bevel
    ctx.strokeStyle = "rgba(255,255,255,.22)"; ctx.translate(2, 2); ctx.beginPath(); poly(s.pts); ctx.stroke();
    ctx.strokeStyle = "rgba(0,0,0,.18)"; ctx.translate(-4, -4); ctx.beginPath(); poly(s.pts); ctx.stroke(); ctx.restore();
  }
  TRACKS.forEach((t) => { // cups: shallow dimples pressed into the foam
    const [x, y] = t.cup, r = CUP_R * scale;
    const g = ctx.createRadialGradient(sx(x) + r * 0.25, sy(y) + r * 0.3, r * 0.1, sx(x), sy(y), r);
    g.addColorStop(0, "rgba(255,255,255,.12)"); g.addColorStop(0.7, "rgba(0,0,0,.28)"); g.addColorStop(1, "rgba(0,0,0,.42)");
    ctx.fillStyle = g; ctx.beginPath(); ctx.arc(sx(x), sy(y), r, 0, 7); ctx.fill();
    ctx.strokeStyle = "rgba(255,255,255,.35)"; ctx.lineWidth = 1.5; ctx.beginPath(); ctx.arc(sx(x), sy(y), r, Math.PI * 0.15, Math.PI * 0.85); ctx.stroke();
  });
  ctx.lineCap = "round";
  for (const w of WALLS) {
    if (w[4] === "gate") continue;
    ctx.strokeStyle = INK; ctx.lineWidth = w[4] === "stop" ? 0.035 * scale : 0.022 * scale;
    ctx.beginPath(); ctx.moveTo(sx(w[0]), sy(w[1])); ctx.lineTo(sx(w[2]), sy(w[3])); ctx.stroke();
  }
  const open = gate > 0 ? 1 : 0, gl = CH * scale; // hinged gate bar, swings up when open
  ctx.save(); ctx.translate(sx(PAD.x0), sy(0)); ctx.rotate(-Math.PI / 2 - open * 1.3);
  ctx.strokeStyle = COLORS[0]; ctx.lineWidth = 0.025 * scale; ctx.beginPath(); ctx.moveTo(0, 0); ctx.lineTo(gl, 0); ctx.stroke(); ctx.restore();
}
function drawBall(b) {
  const lift = b.level === "ground" || b.level === "air" ? b.h - TABLE_H : 0; // nearer the camera looks bigger
  const sink = b.level === "cup" ? b.sink : 0;
  const r = R * scale * (1 + lift * 0.25) * (1 - sink * 0.08), px = sx(b.x), py = sy(b.y);
  const [ox, oy] = shadowAt((b.level === "air" ? b.h - (b.y > -1 && onTable(b.x, b.y) ? TABLE_H : 0) : 0) + R * 0.6);
  ctx.fillStyle = `rgba(50,30,10,${b.level === "air" ? 0.2 : 0.32})`;
  ctx.beginPath(); ctx.ellipse(px + ox, py + oy, r * (1 - sink * 0.3), r * 0.9 * (1 - sink * 0.3), 0.6, 0, 7); ctx.fill();
  const g = ctx.createRadialGradient(px - r * 0.4, py - r * 0.45, r * 0.05, px, py, r); // lit from the upper left
  g.addColorStop(0, "#d79a5e"); g.addColorStop(0.55, "#9a5f2e"); g.addColorStop(1, "#5b3418");
  ctx.fillStyle = g; ctx.beginPath(); ctx.arc(px, py, r, 0, 7); ctx.fill();
  ctx.save(); ctx.beginPath(); ctx.arc(px, py, r, 0, 7); ctx.clip();
  ctx.translate(px, py); ctx.rotate(-b.ang + Math.PI / 2);
  const off = Math.sin(b.spin) * r * 0.9; // grain rings and a dark band roll past
  ctx.strokeStyle = "rgba(70,38,15,.35)"; ctx.lineWidth = 1;
  for (let k = -2; k <= 2; k++) { ctx.beginPath(); ctx.ellipse(0, off + k * r * 0.35, r * 1.1, r * 0.18, 0, 0, 7); ctx.stroke(); }
  ctx.restore();
  ctx.strokeStyle = "rgba(43,24,10,.7)"; ctx.lineWidth = 1; ctx.beginPath(); ctx.arc(px, py, r, 0, 7); ctx.stroke();
  if (sink) { ctx.strokeStyle = `rgba(0,0,0,${0.25 * sink})`; ctx.lineWidth = 3; ctx.beginPath(); ctx.arc(px, py, r, Math.PI * 0.1, Math.PI * 0.9); ctx.stroke(); }
}
function draw(now) {
  beach((now - t0) / 1000);
  balls.filter((b) => b.level === "ground").forEach(drawBall);
  drawTable();
  TORCHES.forEach((t, i) => torch(t, (now - t0) / 1000, i));
  if (path.length > 1) {
    ctx.strokeStyle = "rgba(43,36,24,.22)"; ctx.lineWidth = 1.5; ctx.beginPath();
    path.forEach(([x, y], i) => (i ? ctx.lineTo(sx(x), sy(y)) : ctx.moveTo(sx(x), sy(y)))); ctx.stroke();
  }
  balls.filter((b) => b.level === "cup" || b.level === "rack" || b.level === "table").forEach(drawBall);
  balls.filter((b) => b.level === "air").sort((a, b) => a.h - b.h).forEach(drawBall);
}

const t0 = performance.now();
let last = t0, acc = 0;
const STEP = 1 / 240;
function frame(now) {
  acc += Math.min(0.05, (now - last) / 1000); last = now;
  while (acc >= STEP) {
    if (held && target) { const k = 1 - Math.exp(-STEP * 24); cur.x += (target.x - cur.x) * k; cur.y += (target.y - cur.y) * k; }
    step(STEP); acc -= STEP;
  }
  draw(now);
  requestAnimationFrame(frame);
}

if (DEBUG) window.__it = {
  throw: (vx, vy) => throwBall(vx, vy), ready, place: (x, y) => { cur.x = x; cur.y = y; },
  state: () => ({ cups: cupCount(), used: used(), done, cur: cur && { level: cur.level, x: cur.x, y: cur.y, v: speed(cur), thrown: cur.thrown }, balls: balls.map((b) => ({ level: b.level, x: +b.x.toFixed(3), y: +b.y.toFixed(3), v: +speed(b).toFixed(3) })) }),
  start: () => { $("intro").classList.add("hidden"); playing = true; },
};
$("go").onclick = () => { $("intro").classList.add("hidden"); playing = true; };
$("again").onclick = () => reset();
resize(); reset(); requestAnimationFrame(frame);
}
