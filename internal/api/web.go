package api

import "net/http"

func (s *Server) handleWeb(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("X-NESD-Web-Build", "webrtc-stats-v2")
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
    <div class="status">UI build: webrtc-stats-v2</div>
    <div class="row">
      <div class="panel">
        <video id="video" controls autoplay muted playsinline></video>
        <audio id="audio" autoplay></audio>
        <div style="margin-top:10px">
          <button id="btn-unmute">Unmute</button>
          <button id="btn-webrtc">Use WebRTC</button>
          <button id="btn-hls">Use HLS</button>
          <button id="btn-reload">Reconnect</button>
        </div>
        <div style="margin-top:10px">
          <input id="rom-file" type="file" accept=".nes,application/octet-stream" />
          <button id="btn-upload">Upload ROM</button>
        </div>
      </div>
      <div class="panel">
        <div id="hls-status" class="status">Stream: connecting...</div>
        <div id="webrtc-status" class="status" style="margin-top:10px">WebRTC: idle</div>
        <div id="webrtc-stats-status" class="status" style="margin-top:10px">WebRTC stats: idle</div>
        <div id="receiver-status" class="status" style="margin-top:10px">Receiver: idle</div>
        <div id="audio-status" class="status" style="margin-top:10px">Audio: muted</div>
        <div id="hls-url-status" class="status" style="margin-top:10px">HLS URL: /hls/index.m3u8</div>
        <div id="pad-status" class="status" style="margin-top:10px">Gamepad: waiting...</div>
        <div id="api-status" class="status" style="margin-top:10px">API: idle</div>
        <div id="rom-status" class="status" style="margin-top:10px">ROM: not loaded</div>
        <div id="boot-status" class="status" style="margin-top:10px">Boot: init</div>
        <div class="map">
          <div>A / B</div><div>Pad 0 / 1, Keyboard Z / X</div>
          <div>Start / Select</div><div>Pad 9 / 8, Keyboard Enter / Shift</div>
          <div>D-Pad</div><div>Pad 12-15, Keyboard Arrows</div>
          <div>Fallback stick</div><div>axes 0 / 1</div>
        </div>
      </div>
    </div>
  </div>

  <script>
    const video = document.getElementById("video");
    const audio = document.getElementById("audio");
    const hlsStatus = document.getElementById("hls-status");
    const webrtcStatus = document.getElementById("webrtc-status");
    const webrtcStatsStatus = document.getElementById("webrtc-stats-status");
    const receiverStatus = document.getElementById("receiver-status");
    const audioStatus = document.getElementById("audio-status");
    const hlsURLStatus = document.getElementById("hls-url-status");
    const padStatus = document.getElementById("pad-status");
    const apiStatus = document.getElementById("api-status");
    const romStatus = document.getElementById("rom-status");
    const bootStatus = document.getElementById("boot-status");
    const HLS_URL = "/hls/index.m3u8";
    const WEBRTC_OFFER_URL = "/v1/webrtc/offer";
    const WEBRTC_STATS_URL = "/v1/webrtc/stats";
    const INPUT_URL = "/v1/input/player/1";
    const ROM_UPLOAD_URL = "/v1/rom/upload";
    const STATE_URL = "/v1/state";

    function setStatus(el, text, ok) {
      el.textContent = text;
      el.className = "status " + (ok ? "ok" : "warn");
    }

    function absoluteURL(path) {
      return new URL(path, window.location.href).toString();
    }

    let hls;
    let pc;
    let remoteStream;
    let webrtcStatsTimer;
    let lastWebRTCStats;

    function logBoot(text, ok) {
      setStatus(bootStatus, "Boot: " + text, ok);
    }

    function renderAudioStatus() {
      const hasAudio = remoteStream && remoteStream.getAudioTracks().length > 0;
      const text = "Audio: " + (audio.muted ? "muted" : "unmuted") + (hasAudio ? " track=present" : " track=missing");
      setStatus(audioStatus, text, hasAudio && !audio.muted);
    }

    function destroyStream() {
      if (webrtcStatsTimer) {
        window.clearInterval(webrtcStatsTimer);
        webrtcStatsTimer = null;
      }
      if (hls) {
        hls.destroy();
        hls = null;
      }
      if (pc) {
        pc.close();
        pc = null;
      }
      if (remoteStream) {
        remoteStream.getTracks().forEach((track) => track.stop());
        remoteStream = null;
      }
      video.srcObject = null;
      audio.srcObject = null;
      video.removeAttribute("src");
      audio.removeAttribute("src");
      video.load();
      audio.load();
      lastWebRTCStats = null;
      setStatus(webrtcStatus, "WebRTC: idle", true);
      setStatus(webrtcStatsStatus, "WebRTC stats: idle", true);
      setStatus(receiverStatus, "Receiver: idle", true);
      renderAudioStatus();
    }

    function renderHLSURL() {
      setStatus(hlsURLStatus, "HLS URL: " + absoluteURL(HLS_URL), true);
    }

    function describePC() {
      if (!pc) {
        return "idle";
      }
      return "conn=" + pc.connectionState + " ice=" + pc.iceConnectionState + " gather=" + pc.iceGatheringState;
    }

    function renderWebRTCStatus() {
      const parts = ["WebRTC: " + describePC()];
      if (lastWebRTCStats) {
        parts.push("peers=" + lastWebRTCStats.peer_count);
        parts.push("video_pkts=" + lastWebRTCStats.video_packets);
        parts.push("audio_pkts=" + lastWebRTCStats.audio_packets);
        parts.push("frames_in=" + lastWebRTCStats.frames_in);
      }
      setStatus(
        webrtcStatus,
        parts.join(" "),
        !!pc && !!lastWebRTCStats && lastWebRTCStats.running && pc.connectionState === "connected",
      );
    }

    function tryPlayMedia(el) {
      const playPromise = el.play();
      if (playPromise && typeof playPromise.catch === "function") {
        playPromise.catch(() => {});
      }
    }

    async function refreshWebRTCStats() {
      if (!pc) {
        return;
      }
      try {
        const res = await fetch(WEBRTC_STATS_URL, { cache: "no-store" });
        if (!res.ok) {
          setStatus(webrtcStatsStatus, "WebRTC stats failed: " + res.status, false);
          setStatus(webrtcStatus, "WebRTC stats failed: " + res.status, false);
          return;
        }
        lastWebRTCStats = await res.json();
        setStatus(
          webrtcStatsStatus,
          "WebRTC stats: peers=" + lastWebRTCStats.peer_count + " video_pkts=" + lastWebRTCStats.video_packets + " audio_pkts=" + lastWebRTCStats.audio_packets + " frames_in=" + lastWebRTCStats.frames_in + " ffmpeg_exited=" + lastWebRTCStats.ffmpeg_exited + (lastWebRTCStats.last_error ? " err=" + lastWebRTCStats.last_error : ""),
          lastWebRTCStats.running,
        );
        renderWebRTCStatus();
        await refreshReceiverStats();
      } catch (err) {
        setStatus(webrtcStatsStatus, "WebRTC stats error: " + err, false);
        setStatus(webrtcStatus, "WebRTC stats error: " + err, false);
      }
    }

    async function refreshReceiverStats() {
      if (!pc || typeof pc.getStats !== "function") {
        return;
      }
      try {
        const stats = await pc.getStats();
        let audioLine = "audio=none";
        let videoLine = "video=none";
        stats.forEach((report) => {
          if (report.type !== "inbound-rtp" || report.kind === undefined) {
            return;
          }
          const line =
            "bytes=" + (report.bytesReceived || 0) +
            " pkts=" + (report.packetsReceived || 0) +
            (report.jitter !== undefined ? " jitter=" + report.jitter : "") +
            (report.audioLevel !== undefined ? " level=" + report.audioLevel : "");
          if (report.kind === "audio") {
            audioLine = "audio=" + line;
          } else if (report.kind === "video") {
            videoLine = "video=" + line;
          }
        });
        setStatus(receiverStatus, "Receiver: " + audioLine + " " + videoLine, true);
      } catch (err) {
        setStatus(receiverStatus, "Receiver error: " + err, false);
      }
    }

    async function attachWebRTC() {
      if (!window.RTCPeerConnection) {
        logBoot("RTCPeerConnection unavailable", false);
        return false;
      }
      destroyStream();
      try {
        logBoot("creating WebRTC offer", true);
        setStatus(webrtcStatsStatus, "WebRTC stats: attach start", true);
        pc = new RTCPeerConnection({ iceServers: [] });
        remoteStream = new MediaStream();
        video.srcObject = remoteStream;
        audio.srcObject = remoteStream;
        pc.onconnectionstatechange = () => {
          renderWebRTCStatus();
        };
        pc.oniceconnectionstatechange = () => {
          renderWebRTCStatus();
        };
        pc.ontrack = (event) => {
          const stream = event.streams && event.streams[0];
          if (stream) {
            remoteStream = stream;
            video.srcObject = remoteStream;
            audio.srcObject = remoteStream;
            tryPlayMedia(video);
            tryPlayMedia(audio);
            renderAudioStatus();
            logBoot("WebRTC track stream attached", true);
            return;
          }
          remoteStream.addTrack(event.track);
          event.track.onunmute = () => {
            audio.srcObject = remoteStream;
            tryPlayMedia(video);
            tryPlayMedia(audio);
            renderAudioStatus();
            logBoot("WebRTC track unmuted: " + event.track.kind, true);
          };
        };
        pc.addTransceiver("video", { direction: "recvonly" });
        pc.addTransceiver("audio", { direction: "recvonly" });
        const offer = await pc.createOffer();
        setStatus(webrtcStatsStatus, "WebRTC stats: local offer created", true);
        await pc.setLocalDescription(offer);
        setStatus(webrtcStatsStatus, "WebRTC stats: local description set", true);
        await waitForIceGathering(pc, 1000);
        setStatus(webrtcStatsStatus, "WebRTC stats: posting offer", true);
        const res = await fetch(WEBRTC_OFFER_URL, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(pc.localDescription),
        });
        if (!res.ok) {
          throw new Error("offer failed: " + res.status);
        }
        const answer = await res.json();
        setStatus(webrtcStatsStatus, "WebRTC stats: answer received", true);
        await pc.setRemoteDescription(answer);
        setStatus(webrtcStatsStatus, "WebRTC stats: remote description set", true);
        setStatus(webrtcStatsStatus, "WebRTC stats: waiting...", true);
        webrtcStatsTimer = window.setInterval(refreshWebRTCStats, 1000);
        setStatus(webrtcStatsStatus, "WebRTC stats: timer started", true);
        tryPlayMedia(video);
        tryPlayMedia(audio);
        refreshWebRTCStats().catch(() => {});
        logBoot("WebRTC connected", true);
        setStatus(hlsStatus, "Stream: WebRTC", true);
        return true;
      } catch (err) {
        logBoot("WebRTC failed: " + err, false);
        setStatus(hlsStatus, "WebRTC failed: " + err, false);
        if (pc) {
          pc.close();
          pc = null;
        }
        return false;
      }
    }

    function waitForIceGathering(peer, timeoutMs) {
      if (peer.iceGatheringState === "complete") {
        return Promise.resolve();
      }
      return new Promise((resolve) => {
        const timeout = window.setTimeout(() => {
          peer.removeEventListener("icegatheringstatechange", checkState);
          resolve();
        }, timeoutMs);
        function checkState() {
          if (peer.iceGatheringState === "complete") {
            window.clearTimeout(timeout);
            peer.removeEventListener("icegatheringstatechange", checkState);
            resolve();
          }
        }
        peer.addEventListener("icegatheringstatechange", checkState);
      });
    }

    function attachHLS() {
      destroyStream();
      logBoot("falling back to HLS", true);
      renderHLSURL();
      if (video.canPlayType("application/vnd.apple.mpegurl")) {
        video.src = HLS_URL;
        tryPlayMedia(video);
        setStatus(hlsStatus, "Stream: HLS native", true);
        return;
      }
      if (!window.Hls || !window.Hls.isSupported()) {
        setStatus(hlsStatus, "Stream: HLS not supported", false);
        return;
      }
      hls = new window.Hls({ lowLatencyMode: true, backBufferLength: 8 });
      hls.loadSource(HLS_URL);
      hls.attachMedia(video);
      hls.on(window.Hls.Events.MANIFEST_PARSED, () => {
        tryPlayMedia(video);
        setStatus(hlsStatus, "Stream: HLS", true);
      });
      hls.on(window.Hls.Events.ERROR, (_e, data) => {
        setStatus(hlsStatus, "HLS error: " + data.details, false);
      });
    }

    async function probeAPI() {
      try {
        const res = await fetch(STATE_URL, { cache: "no-store" });
        if (!res.ok) {
          logBoot("state probe failed: " + res.status, false);
          return;
        }
        const st = await res.json();
        logBoot("state ok rom_loaded=" + !!st.rom_loaded, true);
      } catch (err) {
        logBoot("state probe error: " + err, false);
      }
    }

    document.getElementById("btn-unmute").addEventListener("click", () => {
      video.muted = false;
      audio.muted = false;
      video.volume = 1.0;
      audio.volume = 1.0;
      tryPlayMedia(video);
      tryPlayMedia(audio);
      renderAudioStatus();
    });
    document.getElementById("btn-webrtc").addEventListener("click", async () => {
      if (!(await attachWebRTC())) {
        attachHLS();
      }
    });
    document.getElementById("btn-hls").addEventListener("click", () => {
      attachHLS();
    });
    document.getElementById("btn-reload").addEventListener("click", async () => {
      if (!(await attachWebRTC())) {
        attachHLS();
      }
    });

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
    const keyboard = {
      a: false,
      b: false,
      select: false,
      start: false,
      up: false,
      down: false,
      left: false,
      right: false,
    };

    const keyMap = {
      KeyZ: "a",
      KeyX: "b",
      ShiftLeft: "select",
      ShiftRight: "select",
      Enter: "start",
      ArrowUp: "up",
      ArrowDown: "down",
      ArrowLeft: "left",
      ArrowRight: "right",
    };

    function pressed(gp, idx) {
      return gp.buttons[idx] && gp.buttons[idx].pressed;
    }

    function buildButtons(gp) {
      const ax = gp.axes || [0, 0];
      return {
        a: pressed(gp, 0) || keyboard.a,
        b: pressed(gp, 1) || keyboard.b,
        select: pressed(gp, 8) || keyboard.select,
        start: pressed(gp, 9) || keyboard.start,
        up: pressed(gp, 12) || ax[1] < -0.5 || keyboard.up,
        down: pressed(gp, 13) || ax[1] > 0.5 || keyboard.down,
        left: pressed(gp, 14) || ax[0] < -0.5 || keyboard.left,
        right: pressed(gp, 15) || ax[0] > 0.5 || keyboard.right,
      };
    }

    function updateKeyboardState(code, down) {
      const key = keyMap[code];
      if (!key) {
        return false;
      }
      keyboard[key] = down;
      return true;
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
        setStatus(padStatus, "Gamepad: not connected (keyboard available)", false);
      }
      if (gp && connectedIndex !== gp.index) {
        connectedIndex = gp.index;
        setStatus(padStatus, "Gamepad: " + (gp.id || "connected") + " (#" + gp.index + ")", true);
      }
      const buttons = buildButtons(gp || { buttons: [], axes: [0, 0] });
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
      setStatus(padStatus, "Gamepad disconnected (keyboard available)", false);
    });
    window.addEventListener("keydown", (e) => {
      if (updateKeyboardState(e.code, true)) {
        e.preventDefault();
      }
    });
    window.addEventListener("keyup", (e) => {
      if (updateKeyboardState(e.code, false)) {
        e.preventDefault();
      }
    });

    logBoot("script running", true);
    probeAPI();
    renderHLSURL();
    renderAudioStatus();
    attachWebRTC().then((ok) => {
      if (!ok) {
        attachHLS();
      }
    });
    setInterval(tickGamepad, 16);
  </script>
</body>
</html>
`
