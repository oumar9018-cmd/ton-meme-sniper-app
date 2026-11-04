// Telegram WebApp: safe to run in a plain browser too (guarded).
if (window.Telegram && window.Telegram.WebApp) {
  try { window.Telegram.WebApp.ready(); window.Telegram.WebApp.expand(); } catch (e) {}
}
// TON Meme Sniper — Magic Screen prototype. ALL data below is MOCK demo data.
// No wallet, no TON Connect, no real transaction, no network call is made.
const COINS = [
  {
    id: "novac", name: "NovaCat", symbol: "NOVAC", age: "launched 4 min ago",
    price: "0.00042 TON", change: "+18.4%", chart: [12,18,15,22,28,26,34,42], light: "green", lightLabel: "Green",
    reason: "Holders badh rahe, liquidity locked, dev % kam.",
    holders: "1,284 holders (demo)", liquidity: "$48,000 (demo)",
    lpLock: "Locked 12 months (demo)", mintAdmin: "Mint closed (demo)",
    topHolders: "Top 5 holders: 12% (demo)",
    firstBuyers: "First 20 buyers: mostly small retail (demo)"
  },
  {
    id: "sant", name: "Stellar Ant", symbol: "SANT", age: "launched 18 min ago",
    price: "0.00031 TON", change: "+9.2%", chart: [10,14,13,19,22,20,27,31], light: "green", lightLabel: "Green",
    reason: "Liquidity locked, no honeypot flag in demo check.",
    holders: "962 holders (demo)", liquidity: "$31,500 (demo)",
    lpLock: "Locked 6 months (demo)", mintAdmin: "Mint closed (demo)",
    topHolders: "Top 5 holders: 15% (demo)",
    firstBuyers: "First 20 buyers: mixed retail (demo)"
  },
  {
    id: "pfrog", name: "Pixel Frog", symbol: "PFROG", age: "launched 31 min ago",
    price: "0.00018 TON", change: "-4.1%", chart: [30,27,29,24,22,25,21,19], light: "yellow", lightLabel: "Yellow — Risky",
    reason: "Dev holds some supply — buy with extra caution.",
    holders: "540 holders (demo)", liquidity: "$9,800 (demo)",
    lpLock: "Locked 1 month (demo)", mintAdmin: "Mint open (demo)",
    topHolders: "Top 5 holders: 34%, dev wallet 18% (demo)",
    firstBuyers: "First 20 buyers: 3 large wallets (demo)"
  },
  {
    id: "qpump", name: "QuickPump", symbol: "QPUMP", age: "launched 9 min ago",
    price: "0.00009 TON", change: "-31.0%", chart: [28,24,26,18,14,16,10,7], light: "red", lightLabel: "Red — Hidden by default",
    reason: "Honeypot simulation failed in demo check — do not buy.",
    holders: "210 holders (demo)", liquidity: "$2,100 (demo)",
    lpLock: "Not locked (demo)", mintAdmin: "Mint open (demo)",
    topHolders: "Top 5 holders: 82% (demo)",
    firstBuyers: "First 5 buyers took 80% of supply (demo)"
  }
];

let selectedCoin = null;
let selectedAmount = 1;

function esc(s) { return String(s).replace(/[&<>"]/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c])); }

function renderFeed() {
  const showHidden = document.getElementById("showHidden").checked;
  const list = document.getElementById("feedList");
  const visible = COINS.filter(c => c.light !== "red" || showHidden);
  list.innerHTML = visible.map(c => `
    <article class="coin-card ${c.light}" data-id="${c.id}">
      <div class="coin-head" data-detail="${c.id}">
        <span class="logo">${esc(c.symbol.slice(0, 2))}</span>
        <div><h2>${esc(c.name)} <span style="color:var(--muted);font-weight:400">${esc(c.symbol)}</span></h2>
        <p>${esc(c.age)} &middot; DEMO DATA</p></div>
        <div class="price"><span class="dot ${c.light}" aria-label="${c.lightLabel}"></span> <strong>${c.price}</strong><br><span class="${c.change.startsWith("+") ? "up" : "down"}">${c.change}</span></div>
      </div>
      <p class="reason" data-detail="${c.id}">${esc(c.reason)} Tap card for breakdown.</p>
      <svg class="chart" viewBox="0 0 100 32" preserveAspectRatio="none" aria-hidden="true"><polyline points="${c.chart.map((v,i)=>`${i*(100/(c.chart.length-1))},${32-v}`).join(" ")}" fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round" stroke-linecap="round"/></svg>
      <div class="buy-row">
        ${[0.5, 1, 2].map(a => `<button type="button" data-buy="${c.id}" data-amount="${a}" ${c.light === "red" ? "disabled" : ""}>Buy ${a} TON (DEMO)</button>`).join("")}
      </div>
      ${c.light === "red" ? `<p class="hint">Buy disabled: ${esc(c.reason)}</p>` : ""}
    </article>`).join("");

  list.querySelectorAll("[data-detail]").forEach(el => {
    el.addEventListener("click", () => openDetail(el.dataset.detail));
  });
  list.querySelectorAll("[data-buy]").forEach(btn => {
    btn.addEventListener("click", (e) => { e.stopPropagation(); openBuy(btn.dataset.buy, parseFloat(btn.dataset.amount)); });
  });
}

function openSheet(id) { document.getElementById(id).classList.remove("hidden"); }
function closeSheet(id) { document.getElementById(id).classList.add("hidden"); }

function openDetail(id) {
  const c = COINS.find(x => x.id === id);
  document.getElementById("detailBody").innerHTML = `
    <span class="demo-badge">DEMO DATA</span>
    <h2>${esc(c.name)} (${esc(c.symbol)})</h2>
    <p>${esc(c.price)} &middot; <span class="${c.change.startsWith("+") ? "up" : "down"}">${esc(c.change)}</span> &middot; <span class="dot ${c.light}"></span> ${esc(c.lightLabel)} &middot; ${esc(c.age)}</p>
    <svg class="chart big" viewBox="0 0 100 32" preserveAspectRatio="none" aria-hidden="true"><polyline points="${c.chart.map((v,i)=>`${i*(100/(c.chart.length-1))},${32-v}`).join(" ")}" fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round" stroke-linecap="round"/></svg>
    <p>${esc(c.reason)}</p>
    <dl>
      <dt>Holders</dt><dd>${esc(c.holders)}</dd>
      <dt>Liquidity</dt><dd>${esc(c.liquidity)}</dd>
      <dt>LP lock</dt><dd>${esc(c.lpLock)}</dd>
      <dt>Mint admin</dt><dd>${esc(c.mintAdmin)}</dd>
      <dt>Top holders</dt><dd>${esc(c.topHolders)}</dd>
      <dt>First-buyer pattern</dt><dd>${esc(c.firstBuyers)}</dd>
    </dl>
    <p>Mock breakdown only. Later this will come from the watcher feed + Risk Engine flags — not from this file.</p>`;
  openSheet("detailSheet");
}

function openBuy(id, amount) {
  selectedCoin = COINS.find(x => x.id === id);
  selectedAmount = amount;
  // Mock quote: 1 TON = 1,000 demo tokens, slippage 1%, fee 0.05 TON. Demo only.
  const expected = Math.round(amount * 1000 * 0.99);
  document.getElementById("buyBody").innerHTML = `
    <span class="demo-badge">DEMO ONLY — NO REAL TRADE</span>
    <h2>Confirm Buy (DEMO)</h2>
    <p>${esc(selectedCoin.name)} (${esc(selectedCoin.symbol)}) — ${esc(selectedCoin.reason)}</p>
    <dl>
      <dt>Amount</dt><dd>${amount} TON</dd>
      <dt>Expected tokens (mock)</dt><dd>~${expected.toLocaleString()} ${esc(selectedCoin.symbol)}</dd>
      <dt>Slippage (mock)</dt><dd>1%</dd>
      <dt>Fee (mock)</dt><dd>0.05 TON</dd>
      <dt>Total you pay (mock)</dt><dd>${(amount + 0.05).toFixed(2)} TON</dd>
    </dl>
    <p>No wallet is connected. No signing happens. Pressing Confirm only shows a demo success screen.</p>
    <button class="confirm-btn" id="confirmDemo" type="button">Confirm Buy — DEMO ONLY</button>`;
  openSheet("buySheet");
  document.getElementById("confirmDemo").addEventListener("click", () => {
    closeSheet("buySheet");
    document.getElementById("successBody").innerHTML = `
      <span class="demo-badge">DEMO ONLY</span>
      <h2>Demo buy recorded</h2>
      <p>You did <strong>not</strong> buy ${esc(selectedCoin.symbol)}. No wallet, no transaction, no TON was spent.</p>
      <p>Demo: ${selectedAmount} TON &rarr; ${esc(selectedCoin.symbol)} (mock). In the real Phase 1 flow this step will show the wallet signing screen instead.</p>`;
    openSheet("successSheet");
  });
}

document.querySelectorAll(".tab").forEach(tab => {
  tab.addEventListener("click", () => {
    document.querySelectorAll(".tab").forEach(t => t.classList.remove("active"));
    document.querySelectorAll(".screen").forEach(s => s.classList.remove("active"));
    tab.classList.add("active");
    document.getElementById(tab.dataset.tab).classList.add("active");
  });
});
document.getElementById("showHidden").addEventListener("change", renderFeed);
document.querySelectorAll("[data-close]").forEach(btn => {
  btn.addEventListener("click", () => closeSheet(btn.dataset.close));
});
document.querySelectorAll(".sheet-backdrop").forEach(bd => {
  bd.addEventListener("click", (e) => { if (e.target === bd) bd.classList.add("hidden"); });
});

renderFeed();
