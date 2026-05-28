const stateEl = document.getElementById("status");
const logsEl = document.getElementById("logs");

const controls = {
  inspectionStartBtn: document.getElementById("inspection-start-btn"),
  cardSelected: document.getElementById("card-selected"),
  cardCom: document.getElementById("card-com"),
  irsSelected: document.getElementById("irs-selected"),
  irsDevice1: document.getElementById("irs-device1"),
  irsUseDevice2: document.getElementById("irs-use-device2"),
  irsDevice2: document.getElementById("irs-device2"),
  nm43Selected: document.getElementById("nm43-selected"),
  nm43Com: document.getElementById("nm43-com"),
  applyBtn: document.getElementById("apply-btn"),
  reloadBtn: document.getElementById("reload-btn")
};

function appendLogs(lines) {
  if (!Array.isArray(lines) || lines.length === 0) {
    return;
  }
  const prefix = logsEl.textContent.trim() === "" ? "" : "\n";
  logsEl.textContent += `${prefix}${lines.join("\n")}`;
}

function toIntOrNull(value) {
  if (value === "") {
    return null;
  }
  const num = Number(value);
  return Number.isInteger(num) ? num : null;
}

function setStatus(text, ok, working = false) {
  stateEl.textContent = text;
  if (working) {
    stateEl.className = "working";
    return;
  }
  stateEl.className = ok ? "ok" : "ng";
}

function setCurrentText(id, text) {
  document.getElementById(id).textContent = text;
}

function applyState(state) {
  controls.cardCom.value = state.card.com ?? "";
  controls.irsDevice1.value = state.irs.device1Com ?? "";
  controls.irsUseDevice2.checked = !!state.irs.useDevice2;
  controls.irsDevice2.value = state.irs.device2Com ?? "";
  controls.nm43Com.value = state.nm43.com ?? "";

  setCurrentText(
    "card-current",
    state.card.error ? `現在値: 取得不可 (${state.card.error})` :
      `現在値: ${state.card.com}`
  );
  const irsMsg = state.irs.error ?
    `現在値: 取得不可 (${state.irs.error})` :
    `現在値: D1=${state.irs.device1Com} D2=${state.irs.device2Com ?? "-"}`
  setCurrentText("irs-current", irsMsg);
  setCurrentText(
    "nm43-current",
    state.nm43.error ? `現在値: 取得不可 (${state.nm43.error})` :
      `現在値: ${state.nm43.com}`
  );
}

async function fetchState() {
  const response = await fetch("/api/state");
  const body = await response.json();
  setStatus(body.message, body.statusError === true);
  if (body.state) {
    applyState(body.state);
  }
}

function buildRequest() {
  return {
    card: {
      selected: controls.cardSelected.checked,
      com: toIntOrNull(controls.cardCom.value)
    },
    irs: {
      selected: controls.irsSelected.checked,
      device1Com: toIntOrNull(controls.irsDevice1.value),
      useDevice2: controls.irsUseDevice2.checked,
      device2Com: toIntOrNull(controls.irsDevice2.value)
    },
    nm43: {
      selected: controls.nm43Selected.checked,
      com: toIntOrNull(controls.nm43Com.value)
    }
  };
}

function buildInspectionStartRequest() {
  return {
    card: {
      selected: controls.cardSelected.checked
    },
    irs: {
      selected: controls.irsSelected.checked
    },
    nm43: {
      selected: controls.nm43Selected.checked
    }
  };
}

async function applyRequest(event) {
  event.preventDefault();
  controls.applyBtn.disabled = true;
  try {
    const payload = buildRequest();
    const response = await fetch("/api/apply", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload)
    });
    const body = await response.json();
    setStatus(body.message, body.statusError === true);
    if (Array.isArray(body.logs)) {
      logsEl.textContent = body.logs.join("\n");
    }
    if (body.state) {
      applyState(body.state);
    }
  } catch (err) {
    setStatus(`通信エラー: ${err.message}`, false);
  } finally {
    controls.applyBtn.disabled = false;
  }
}

async function runInspectionStart() {
  if (controls.inspectionStartBtn.disabled) {
    return;
  }
  controls.inspectionStartBtn.disabled = true;
  controls.reloadBtn.disabled = true;
  controls.applyBtn.disabled = true;
  const startedAt = Date.now();
  const timeoutMs = 300000;
  appendLogs([
    `--- ${new Date().toLocaleString()} 出荷検査ツール開始処理開始 ---`,
    "停止対象停止 -> 開始対象開始 -> 検査ツール起動を実行します"
  ]);
  const intervalId = setInterval(() => {
    const elapsed = Math.floor((Date.now() - startedAt) / 1000);
    setStatus(
      `出荷検査ツール開始処理を実行中です... (${elapsed}秒経過)`,
      true,
      true
    );
  }, 1000);
  setStatus("出荷検査ツール開始処理を実行中です... (0秒経過)", true, true);
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), timeoutMs);
  try {
    const response = await fetch("/api/inspection/start", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(buildInspectionStartRequest()),
      signal: controller.signal
    });
    const bodyText = await response.text();
    const contentType = response.headers.get("content-type") || "";
    const isJson = contentType.includes("application/json");
    if (!isJson) {
      let message = `サーバー応答がJSONではありません (HTTP ${response.status})`;
      if (response.status === 404) {
        message =
          "出荷検査ツール開始APIが見つかりません (HTTP 404)。" +
          "旧バージョンのサーバーに接続している可能性があります";
      }
      setStatus(message, false);
      appendLogs([
        message,
        `応答本文: ${bodyText || "(空)"}`
      ]);
      return;
    }
    const body = bodyText === "" ? {} : JSON.parse(bodyText);
    if (!response.ok) {
      setStatus(
        body.message ||
          `出荷検査ツール開始処理に失敗しました (HTTP ${response.status})`,
        false
      );
      appendLogs(body.logs);
      return;
    }
    setStatus(
      body.message || "出荷検査ツール開始処理が完了しました",
      body.statusError === true
    );
    appendLogs(body.logs);
  } catch (err) {
    if (err.name === "AbortError") {
      setStatus(
        "出荷検査ツール開始処理が5分以内に完了しませんでした。サービス状態を確認してください",
        false
      );
      appendLogs([
        "タイムアウト: 出荷検査ツール開始処理の応答待ちを中断しました"
      ]);
      return;
    }
    setStatus(`通信エラー: ${err.message}`, false);
    appendLogs([`通信エラー詳細: ${err.message}`]);
  } finally {
    clearInterval(intervalId);
    clearTimeout(timeoutId);
    controls.inspectionStartBtn.disabled = false;
    controls.reloadBtn.disabled = false;
    controls.applyBtn.disabled = false;
  }
}

document.getElementById("apply-form").addEventListener("submit", applyRequest);
controls.inspectionStartBtn.addEventListener("click", runInspectionStart);
controls.reloadBtn.addEventListener("click", fetchState);
fetchState().catch((err) => setStatus(`初期読込失敗: ${err.message}`, false));
