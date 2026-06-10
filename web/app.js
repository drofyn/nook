const state = {
  selfId: null,
  name: '',
  peers: new Map(),
  connections: new Map(),
  transfers: new Map(),
  ws: null,
  reconnectTimer: null,
  selectedPeerId: null,
};

const CHUNK_SIZE = 64 * 1024;

function getDeviceName() {
  const ua = navigator.userAgent;
  const suffix = '-' + Math.random().toString(36).slice(2, 6);
  if (ua.includes('iPhone')) return 'iPhone' + suffix;
  if (ua.includes('iPad') || (ua.includes('Mac') && navigator.maxTouchPoints > 1)) return 'iPad' + suffix;
  if (ua.includes('Android')) return 'Android' + suffix;
  if (ua.includes('Windows')) return 'Windows' + suffix;
  if (ua.includes('Mac')) return 'Mac' + suffix;
  if (ua.includes('Linux')) return 'Linux' + suffix;
  return 'Device' + suffix;
}

function connect() {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
  const ws = new WebSocket(proto + '//' + location.host + '/ws');

  ws.onopen = () => {
    state.ws = ws;
    updateStatus(true);
    const name = document.getElementById('deviceName').value || getDeviceName();
    state.name = name;
    ws.send(JSON.stringify({ type: 'hello', name }));
  };

  ws.onmessage = (e) => {
    const msg = JSON.parse(e.data);
    handleMessage(msg);
  };

  ws.onclose = () => {
    state.ws = null;
    updateStatus(false);
    state.peers.clear();
    renderPeers();
    scheduleReconnect();
  };

  ws.onerror = () => { ws.close(); };
}

function scheduleReconnect() {
  if (state.reconnectTimer) return;
  state.reconnectTimer = setTimeout(() => {
    state.reconnectTimer = null;
    connect();
  }, 3000);
}

function updateStatus(connected) {
  const dot = document.getElementById('statusDot');
  const text = document.getElementById('statusText');
  dot.className = 'dot ' + (connected ? 'connected' : 'disconnected');
  text.textContent = connected ? 'Connected' : 'Reconnecting...';
}

function handleMessage(msg) {
  switch (msg.type) {
    case 'welcome':
      state.selfId = msg.id;
      for (const p of msg.peers || []) {
        state.peers.set(p.id, p);
      }
      renderPeers();
      break;

    case 'peer-joined':
      state.peers.set(msg.peer.id, msg.peer);
      renderPeers();
      break;

    case 'peer-left':
      state.peers.delete(msg.id);
      closePeerConnection(msg.id);
      renderPeers();
      break;

    case 'signal':
      handleSignal(msg);
      break;

    case 'error':
      console.error('server error:', msg.error);
      break;
  }
}

function handleSignal(msg) {
  const peerId = msg.from;
  let pc = state.connections.get(peerId);

  if (!pc) {
    pc = createPeerConnection(peerId);
  }

  const data = msg.data;
  if (data.kind === 'offer') {
    pc.setRemoteDescription(new RTCSessionDescription(data))
      .then(() => pc.createAnswer())
      .then(answer => pc.setLocalDescription(answer))
      .then(() => {
        sendSignal(peerId, { kind: 'answer', sdp: pc.localDescription.sdp, type: pc.localDescription.type });
      })
      .catch(err => console.error('handle offer error:', err));
  } else if (data.kind === 'answer') {
    pc.setRemoteDescription(new RTCSessionDescription(data)).catch(err => console.error('set remote answer error:', err));
  } else if (data.kind === 'candidate') {
    pc.addIceCandidate(new RTCIceCandidate(data.candidate)).catch(err => console.error('add ice candidate error:', err));
  }
}

function createPeerConnection(peerId) {
  const pc = new RTCPeerConnection({ iceServers: [] });
  pc._dataChannels = [];

  pc.onicecandidate = (e) => {
    if (e.candidate) {
      sendSignal(peerId, {
        kind: 'candidate',
        candidate: e.candidate.toJSON(),
      });
    }
  };

  pc.ondatachannel = (e) => {
    setupDataChannel(peerId, e.channel, false);
    pc._dataChannels.push(e.channel);
  };

  pc.onconnectionstatechange = () => {
    if (pc.connectionState === 'disconnected' || pc.connectionState === 'failed' || pc.connectionState === 'closed') {
      state.connections.delete(peerId);
    }
  };

  state.connections.set(peerId, pc);
  return pc;
}

function setupDataChannel(peerId, channel, isOutgoing) {
  channel.binaryType = 'arraybuffer';
  let receiveBuffer = [];
  let receiveMeta = null;
  let receivedSize = 0;

  channel.onmessage = (e) => {
    if (typeof e.data === 'string') {
      const msg = JSON.parse(e.data);
      handleDataMessage(peerId, msg, channel);
    } else {
      receiveBuffer.push(e.data);
      receivedSize += e.data.byteLength;

      if (receiveMeta) {
        updateTransferProgress(receiveMeta.id, receivedSize, receiveMeta.size);
      }
    }
  };

  channel.onclose = () => {};

  channel._receiveBuffer = receiveBuffer;
  channel._getReceiveMeta = () => receiveMeta;
  channel._setReceiveMeta = (m) => { receiveMeta = m; };
  channel._getReceivedSize = () => receivedSize;
  channel._resetReceive = () => {
    receiveBuffer = [];
    receiveMeta = null;
    receivedSize = 0;
  };
}

function handleDataMessage(peerId, msg, channel) {
  switch (msg.type) {
    case 'file-offer': {
      const peer = state.peers.get(peerId);
      const name = peer ? peer.name : peerId;
      showReceiveModal(name, msg, peerId, channel);
      break;
    }
    case 'file-accept': {
      const t = state.transfers.get(msg.id);
      if (t) {
        t.accepted = true;
        startSendingChunks(t);
      }
      break;
    }
    case 'file-reject': {
      const t = state.transfers.get(msg.id);
      if (t) {
        t.status = 'rejected';
        updateTransferStatus(msg.id, 'Rejected', 'error');
      }
      break;
    }
    case 'file-done': {
      finishReceive(msg.id, channel);
      break;
    }
    case 'text': {
      const peer = state.peers.get(peerId);
      const name = peer ? peer.name : peerId;
      showTextMessage(name, msg.text);
      break;
    }
  }
}

function sendSignal(peerId, data) {
  if (!state.ws || state.ws.readyState !== WebSocket.OPEN) return;
  state.ws.send(JSON.stringify({
    type: 'signal',
    to: peerId,
    data,
  }));
}

function openSendModal(peerId) {
  state.selectedPeerId = peerId;
  const peer = state.peers.get(peerId);
  document.getElementById('sendTargetName').textContent = peer ? peer.name : peerId;
  document.getElementById('textInput').value = '';
  document.getElementById('fileInput').value = '';
  document.getElementById('sendModal').classList.remove('hidden');
}

function closeSendModal() {
  document.getElementById('sendModal').classList.add('hidden');
  state.selectedPeerId = null;
}

function sendText() {
  const peerId = state.selectedPeerId;
  if (!peerId) return;

  const text = document.getElementById('textInput').value.trim();
  if (!text) return;

  const channel = getOrCreateDataChannel(peerId);
  if (!channel) return;

  const id = 't_' + Date.now();
  sendWhenOpen(channel, JSON.stringify({ type: 'text', id, text }));
  document.getElementById('textInput').value = '';
  closeSendModal();
}

function sendFile(files) {
  const peerId = state.selectedPeerId;
  if (!peerId || !files.length) return;

  const channel = getOrCreateDataChannel(peerId);
  if (!channel) return;

  for (const file of files) {
    const id = 't_' + Date.now() + '_' + Math.random().toString(36).slice(2, 6);
    const transfer = {
      id,
      file,
      channel,
      status: 'waiting',
      offset: 0,
      accepted: false,
    };
    state.transfers.set(id, transfer);
    addTransferItem(id, file.name, file.size, 'sending');

    sendWhenOpen(channel, JSON.stringify({
      type: 'file-offer',
      id,
      name: file.name,
      size: file.size,
      mime: file.type || 'application/octet-stream',
    }));
  }

  closeSendModal();
}

function startSendingChunks(transfer) {
  transfer.status = 'sending';
  updateTransferStatus(transfer.id, 'Sending...', 'sending');

  const reader = new FileReader();
  let offset = 0;

  reader.onload = (e) => {
    sendWhenOpen(transfer.channel, e.target.result);
    offset += e.target.result.byteLength;
    transfer.offset = offset;
    updateTransferProgress(transfer.id, offset, transfer.file.size);

    if (offset < transfer.file.size) {
      readNext();
    } else {
      sendWhenOpen(transfer.channel, JSON.stringify({ type: 'file-done', id: transfer.id }));
      transfer.status = 'done';
      updateTransferStatus(transfer.id, 'Sent', 'done');
    }
  };

  function readNext() {
    const slice = transfer.file.slice(offset, offset + CHUNK_SIZE);
    reader.readAsArrayBuffer(slice);
  }

  readNext();
}

function sendWhenOpen(channel, data) {
  if (channel.readyState === 'open') {
    channel.send(data);
    return;
  }
  const oldOpen = channel.onopen;
  channel.onopen = (e) => {
    if (oldOpen) oldOpen(e);
    channel.send(data);
  };
}

function getOrCreateDataChannel(peerId) {
  let pc = state.connections.get(peerId);
  if (!pc) {
    pc = createPeerConnection(peerId);
  }

  for (const dc of pc._dataChannels || []) {
    if (dc.readyState === 'open') return dc;
  }

  const channel = pc.createDataChannel('filetransfer');
  setupDataChannel(peerId, channel, true);

  if (!pc._dataChannels) pc._dataChannels = [];
  pc._dataChannels.push(channel);

  pc.createOffer()
    .then(offer => pc.setLocalDescription(offer))
    .then(() => {
      sendSignal(peerId, { kind: 'offer', sdp: pc.localDescription.sdp, type: pc.localDescription.type });
    })
    .catch(err => console.error('create offer error:', err));

  return channel;
}

function showReceiveModal(fromName, msg, peerId, channel) {
  const modal = document.getElementById('receiveModal');
  document.getElementById('receiveFromName').textContent = fromName;
  document.getElementById('receiveBody').innerHTML =
    '<p>' + fromName + ' wants to send <strong>' + msg.name + '</strong> (' + formatSize(msg.size) + ')</p>' +
    '<div style="margin-top:12px">' +
    '<button class="accept-btn" onclick="acceptFile(\'' + peerId + '\',\'' + msg.id + '\',' + msg.size + ')">Accept</button>' +
    '<button class="reject-btn" onclick="rejectFile(\'' + peerId + '\',\'' + msg.id + '\')">Reject</button>' +
    '</div>';
  modal.classList.remove('hidden');

  channel._pendingOffer = { peerId, id: msg.id, name: msg.name, size: msg.size };
}

function acceptFile(peerId, id, size) {
  document.getElementById('receiveModal').classList.add('hidden');

  const pc = state.connections.get(peerId);
  if (!pc) return;

  for (const dc of pc._dataChannels || []) {
    if (dc._pendingOffer && dc._pendingOffer.id === id) {
      sendWhenOpen(dc, JSON.stringify({ type: 'file-accept', id }));
      dc._setReceiveMeta({ id, name: dc._pendingOffer.name, size });
      addTransferItem(id, dc._pendingOffer.name, size, 'receiving');
      updateTransferStatus(id, 'Receiving...', 'receiving');
      return;
    }
  }
}

function rejectFile(peerId, id) {
  document.getElementById('receiveModal').classList.add('hidden');

  const pc = state.connections.get(peerId);
  if (!pc) return;

  for (const dc of pc._dataChannels || []) {
    if (dc._pendingOffer && dc._pendingOffer.id === id) {
      sendWhenOpen(dc, JSON.stringify({ type: 'file-reject', id }));
      return;
    }
  }
}

function finishReceive(id, channel) {
  const meta = channel._getReceiveMeta();
  if (!meta) return;

  const blob = new Blob(channel._receiveBuffer);
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = meta.name || 'file';
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);

  channel._resetReceive();
  updateTransferStatus(id, 'Downloaded', 'done');
}

function showTextMessage(from, text) {
  const modal = document.getElementById('receiveModal');
  document.getElementById('receiveFromName').textContent = from;
  document.getElementById('receiveBody').innerHTML =
    '<div class="text-msg">' + escapeHtml(text) + '</div>' +
    '<button class="copy-btn" onclick="copyText(this)">Copy</button>' +
    '<button class="close-btn" style="float:right;margin-top:6px" onclick="document.getElementById(\'receiveModal\').classList.add(\'hidden\')">Close</button>';
  modal.classList.remove('hidden');
}

function copyText(btn) {
  const textEl = btn.parentElement.querySelector('.text-msg');
  navigator.clipboard.writeText(textEl.textContent).catch(() => {});
  btn.textContent = 'Copied!';
  setTimeout(() => { btn.textContent = 'Copy'; }, 2000);
}

function closePeerConnection(peerId) {
  const pc = state.connections.get(peerId);
  if (pc) {
    pc.close();
    state.connections.delete(peerId);
  }
}

function renderPeers() {
  const list = document.getElementById('peerList');
  if (state.peers.size === 0) {
    list.innerHTML = '<div class="empty-hint">No other devices online</div>';
    return;
  }

  let html = '';
  for (const [id, peer] of state.peers) {
    html += '<div class="peer-card" onclick="openSendModal(\'' + id + '\')">' +
      '<span class="peer-name">' + escapeHtml(peer.name) + '</span>' +
      '<span class="peer-id">' + escapeHtml(id) + '</span>' +
      '</div>';
  }
  list.innerHTML = html;
}

function addTransferItem(id, name, size, type) {
  const list = document.getElementById('transferList');
  const empty = list.querySelector('.empty-hint');
  if (empty) empty.remove();

  const div = document.createElement('div');
  div.className = 'transfer-item';
  div.id = 'transfer-' + id;
  div.innerHTML =
    '<div class="info">' +
    '<div class="name">' + escapeHtml(name) + '</div>' +
    '<div class="meta">' + formatSize(size) + '</div>' +
    '<div class="progress-bar"><div class="fill" style="width:0%"></div></div>' +
    '</div>' +
    '<span class="status-badge ' + type + '">Waiting</span>';
  list.prepend(div);
}

function updateTransferProgress(id, loaded, total) {
  const item = document.getElementById('transfer-' + id);
  if (!item) return;
  const fill = item.querySelector('.fill');
  if (fill) fill.style.width = Math.min(100, (loaded / total) * 100) + '%';
}

function updateTransferStatus(id, text, cls) {
  const item = document.getElementById('transfer-' + id);
  if (!item) return;
  const badge = item.querySelector('.status-badge');
  if (badge) {
    badge.textContent = text;
    badge.className = 'status-badge ' + cls;
  }
}

function formatSize(bytes) {
  if (bytes < 1024) return bytes + ' B';
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
  if (bytes < 1024 * 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
  return (bytes / (1024 * 1024 * 1024)).toFixed(1) + ' GB';
}

function escapeHtml(str) {
  const div = document.createElement('div');
  div.textContent = str;
  return div.innerHTML;
}

document.getElementById('deviceName').value = getDeviceName();
document.getElementById('deviceName').addEventListener('change', function() {
  state.name = this.value;
  if (state.ws && state.ws.readyState === WebSocket.OPEN) {
    state.ws.send(JSON.stringify({ type: 'hello', name: this.value }));
  }
});

connect();
