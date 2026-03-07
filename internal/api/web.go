package api

import "net/http"

func (s *Server) handleWeb(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(webPageHTML))
}

const webPageHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>nesemud web</title>
  <style>
    :root {
      --bg: #0f1115;
      --panel: #171a21;
      --text: #f0f3f8;
      --muted: #a8b0c0;
      --ok: #3ed38b;
      --warn: #ffb020;
      --accent: #57a6ff;
    }
    html, body {
      margin: 0;
      background: radial-gradient(circle at 20% 0%, #1a2233 0%, var(--bg) 45%);
      color: var(--text);
      font-family: ui-sans-serif, system-ui, -apple-system, Segoe UI, Roboto, Helvetica, Arial, sans-serif;
    }
    .wrap {
      max-width: 1100px;
      margin: 0 auto;
      padding: 16px;
    }
    .row {
      display: grid;
      grid-template-columns: 2fr 1fr;
      gap: 14px;
    }
    .panel {
      background: var(--panel);
      border: 1px solid #232838;
      border-radius: 12px;
      padding: 12px;
    }
    video {
      width: 100%;
      max-height: 72vh;
      background: #000;
      border-radius: 8px;
    }
    .status {
      font-size: 13px;
      color: var(--muted);
      line-height: 1.5;
      word-break: break-word;
    }
    .ok { color: var(--ok); }
    .warn { color: var(--warn); }
    button {
      background: #24314c;
      color: #f8fbff;
      border: 1px solid #385181;
      border-radius: 8px;
      padding: 8px 10px;
      cursor: pointer;
      margin-right: 6px;
    }
    .map {
      display: grid;
      grid-template-columns: 1fr 1fr;
      font-size: 13px;
      gap: 6px 10px;
      color: var(--muted);
      margin-top: 8px;
    }
    @media (max-width: 900px) {
      .row { grid-template-columns: 1fr; }
    }
  </style>
  <script src="https://cdn.jsdelivr.net/npm/hls.js@latest"></script>
</head>
<body>
  <div class="wrap">
    <h2>nesemud /web</h2>
    <div class="row">
      <div class="panel">
        <video id="video" controls autoplay muted playsinline></video>
        <div style="margin-top:10px">
          <button id="btn-unmute">Unmute</button>
          <button id="btn-reload">Reload HLS</button>
        </div>
        <div style="margin-top:10px">
          <input id="rom-file" type="file" accept=".nes,application/octet-stream" />
          <button id="btn-upload">Upload ROM</button>
        </div>
      </div>
      <div class="panel">
        <div id="hls-status" class="status">HLS: connecting...</div>
        <div id="pad-status" class="status" style="margin-top:10px">Gamepad: waiting...</div>
        <div id="api-status" class="status" style="margin-top:10px">API: idle</div>
        <div id="rom-status" class="status" style="margin-top:10px">ROM: not loaded</div>
        <div class="map">
          <div>A / B</div><div>Gamepad buttons 0 / 1</div>
          <div>Start / Select</div><div>buttons 9 / 8</div>
          <div>D-Pad</div><div>buttons 12/13/14/15</div>
          <div>Fallback stick</div><div>axes 0 / 1</div>
        </div>
      </div>
    </div>
  </div>

  <script>
    const video = document.getElementById("video");
    const hlsStatus = document.getElementById("hls-status");
    const padStatus = document.getElementById("pad-status");
    const apiStatus = document.getElementById("api-status");
    const romStatus = document.getElementById("rom-status");
    const HLS_URL = "/hls/index.m3u8";
    const INPUT_URL = "/v1/input/player/1";
    const ROM_UPLOAD_URL = "/v1/rom/upload";

    function setStatus(el, text, ok) {
      el.textContent = text;
      el.className = "status " + (ok ? "ok" : "warn");
    }

    let hls;
    function attachHLS() {
      if (hls) {
        hls.destroy();
        hls = null;
      }
      if (video.canPlayType("application/vnd.apple.mpegurl")) {
        video.src = HLS_URL;
        video.play().catch(() => {});
        setStatus(hlsStatus, "HLS: native playback", true);
        return;
      }
      if (!window.Hls || !window.Hls.isSupported()) {
        setStatus(hlsStatus, "HLS: not supported in this browser", false);
        return;
      }
      hls = new window.Hls({ lowLatencyMode: true, backBufferLength: 8 });
      hls.loadSource(HLS_URL);
      hls.attachMedia(video);
      hls.on(window.Hls.Events.MANIFEST_PARSED, () => {
        video.play().catch(() => {});
        setStatus(hlsStatus, "HLS: streaming", true);
      });
      hls.on(window.Hls.Events.ERROR, (_e, data) => {
        setStatus(hlsStatus, "HLS error: " + data.details, false);
      });
    }

    document.getElementById("btn-unmute").addEventListener("click", () => {
      video.muted = false;
      video.volume = 1.0;
      video.play().catch(() => {});
    });
    document.getElementById("btn-reload").addEventListener("click", attachHLS);

    async function uploadROM() {
      const fileInput = document.getElementById("rom-file");
      const file = fileInput.files && fileInput.files[0];
      if (!file) {
        setStatus(romStatus, "ROM: select a .nes file", false);
        return;
      }
      const fd = new FormData();
      fd.append("rom", file, file.name);
      try {
        setStatus(romStatus, "ROM: uploading...", true);
        const res = await fetch(ROM_UPLOAD_URL, { method: "POST", body: fd });
        if (!res.ok) {
          const msg = await res.text();
          setStatus(romStatus, "ROM upload failed: " + res.status + " " + msg, false);
          return;
        }
        setStatus(romStatus, "ROM loaded: " + file.name, true);
      } catch (err) {
        setStatus(romStatus, "ROM upload error: " + err, false);
      }
    }

    document.getElementById("btn-upload").addEventListener("click", uploadROM);

    let lastPayload = "";
    let connectedIndex = -1;
    let sentCount = 0;

    function pressed(gp, idx) {
      return gp.buttons[idx] && gp.buttons[idx].pressed;
    }

    function buildButtons(gp) {
      const ax = gp.axes || [0, 0];
      return {
        a: pressed(gp, 0),
        b: pressed(gp, 1),
        select: pressed(gp, 8),
        start: pressed(gp, 9),
        up: pressed(gp, 12) || ax[1] < -0.5,
        down: pressed(gp, 13) || ax[1] > 0.5,
        left: pressed(gp, 14) || ax[0] < -0.5,
        right: pressed(gp, 15) || ax[0] > 0.5,
      };
    }

    async function pushInput(payload) {
      try {
        const res = await fetch(INPUT_URL, {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: payload,
          keepalive: true,
        });
        if (!res.ok) {
          setStatus(apiStatus, "API input failed: " + res.status, false);
          return;
        }
        sentCount++;
        setStatus(apiStatus, "API input ok (sent=" + sentCount + ")", true);
      } catch (err) {
        setStatus(apiStatus, "API input error: " + err, false);
      }
    }

    function tickGamepad() {
      const pads = navigator.getGamepads ? navigator.getGamepads() : [];
      const gp = Array.from(pads || []).find(Boolean);
      if (!gp) {
        connectedIndex = -1;
        setStatus(padStatus, "Gamepad: not connected", false);
        return;
      }
      if (connectedIndex !== gp.index) {
        connectedIndex = gp.index;
        setStatus(padStatus, "Gamepad: " + (gp.id || "connected") + " (#" + gp.index + ")", true);
      }
      const buttons = buildButtons(gp);
      const payload = JSON.stringify(buttons);
      if (payload !== lastPayload) {
        lastPayload = payload;
        pushInput(payload);
      }
    }

    window.addEventListener("gamepadconnected", (e) => {
      const gp = e.gamepad;
      setStatus(padStatus, "Gamepad connected: " + (gp.id || "unknown"), true);
    });
    window.addEventListener("gamepaddisconnected", () => {
      setStatus(padStatus, "Gamepad disconnected", false);
    });

    attachHLS();
    setInterval(tickGamepad, 16);
  </script>
</body>
</html>
`
