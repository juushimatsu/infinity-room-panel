const API_BASE = "";

function getAuthHeaders(): Record<string, string> {
  const token = localStorage.getItem("audiobot_token");
  if (token) {
    return { Authorization: `Bearer ${token}` };
  }
  return {};
}

export interface AudioFile {
  id: string;
  filename: string;
  size: number;
  uploaded_at: string;
}

export interface BotStatus {
  bot_id: number;
  name: string;
  status: "connecting" | "active" | "error" | "stopped";
  error?: string;
  service?: string;
  room_id?: string;
}

export interface RoomInfo {
  id: string;
  service: string;
  room_input: string;
  active: boolean;
  bot_count: number;
  file_id: string;
  loop: boolean;
  bots: BotStatus[];
}

export async function uploadAudio(
  file: File,
): Promise<{ id: string; filename: string }> {
  const formData = new FormData();
  formData.append("file", file);

  const resp = await fetch(`${API_BASE}/api/audio/upload`, {
    method: "POST",
    headers: getAuthHeaders(),
    body: formData,
  });

  if (!resp.ok) {
    const err = await resp.json().catch(() => ({ error: resp.statusText }));
    throw new Error(err.error || "Upload failed");
  }

  return resp.json();
}

export async function listAudio(): Promise<AudioFile[]> {
  const resp = await fetch(`${API_BASE}/api/audio/list`, {
    headers: getAuthHeaders(),
  });

  if (!resp.ok) {
    throw new Error("Failed to list audio files");
  }

  return resp.json();
}

export async function startRoom(params: {
  service: string;
  room_input: string;
  bot_count: number;
  file_id: string;
  loop: boolean;
}): Promise<string> {
  const resp = await fetch(`${API_BASE}/api/room/start`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...getAuthHeaders(),
    },
    body: JSON.stringify(params),
  });

  if (!resp.ok) {
    const err = await resp.json().catch(() => ({ error: resp.statusText }));
    throw new Error(err.error || "Start failed");
  }

  const data = await resp.json();
  return data.room_id;
}

export async function stopRoom(roomId: string): Promise<void> {
  const resp = await fetch(`${API_BASE}/api/room/stop`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...getAuthHeaders(),
    },
    body: JSON.stringify({ room_id: roomId }),
  });

  if (!resp.ok) {
    const err = await resp.json().catch(() => ({ error: resp.statusText }));
    throw new Error(err.error || "Stop failed");
  }
}

export async function deleteRoom(roomId: string): Promise<void> {
  const resp = await fetch(`${API_BASE}/api/room/delete`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...getAuthHeaders(),
    },
    body: JSON.stringify({ room_id: roomId }),
  });

  if (!resp.ok) {
    const err = await resp.json().catch(() => ({ error: resp.statusText }));
    throw new Error(err.error || "Delete failed");
  }
}

export async function restartRoom(roomId: string): Promise<void> {
  const resp = await fetch(`${API_BASE}/api/room/restart`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...getAuthHeaders(),
    },
    body: JSON.stringify({ room_id: roomId }),
  });

  if (!resp.ok) {
    const err = await resp.json().catch(() => ({ error: resp.statusText }));
    throw new Error(err.error || "Restart failed");
  }
}

export async function updateRoom(params: {
  room_id: string;
  service: string;
  room_input: string;
  bot_count: number;
  file_id: string;
  loop: boolean;
}): Promise<void> {
  const resp = await fetch(`${API_BASE}/api/room/update`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...getAuthHeaders(),
    },
    body: JSON.stringify(params),
  });

  if (!resp.ok) {
    const err = await resp.json().catch(() => ({ error: resp.statusText }));
    throw new Error(err.error || "Update failed");
  }
}

export async function startRoomFromConfig(roomId: string): Promise<string> {
  const resp = await fetch(`${API_BASE}/api/room/start-from-config`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...getAuthHeaders(),
    },
    body: JSON.stringify({ room_id: roomId }),
  });

  if (!resp.ok) {
    const err = await resp.json().catch(() => ({ error: resp.statusText }));
    throw new Error(err.error || "Start failed");
  }
  const data = await resp.json();
  return data.room_id;
}

export interface WBAccountConfig {
  enabled: boolean;
  cookies: string;
  access_token: string;
  user_agent: string;
  display_name: string;
  interval_sec: number;
  stay_duration_sec: number;
}

export async function getWBAccount(): Promise<WBAccountConfig> {
  const resp = await fetch(`${API_BASE}/api/wbstream/account`, {
    headers: getAuthHeaders(),
  });
  if (!resp.ok) {
    throw new Error("Failed to get WB account settings");
  }
  return resp.json();
}

export async function setWBAccount(cfg: WBAccountConfig): Promise<void> {
  const resp = await fetch(`${API_BASE}/api/wbstream/account`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...getAuthHeaders(),
    },
    body: JSON.stringify(cfg),
  });

  if (!resp.ok) {
    const err = await resp.json().catch(() => ({ error: resp.statusText }));
    throw new Error(err.error || "Save failed");
  }
}

export async function stopWBAccount(): Promise<void> {
  const resp = await fetch(`${API_BASE}/api/wbstream/account/stop`, {
    method: "POST",
    headers: getAuthHeaders(),
  });

  if (!resp.ok) {
    const err = await resp.json().catch(() => ({ error: resp.statusText }));
    throw new Error(err.error || "Stop failed");
  }
}

export async function listRooms(): Promise<RoomInfo[]> {
  const resp = await fetch(`${API_BASE}/api/room/list`, {
    headers: getAuthHeaders(),
  });

  if (!resp.ok) {
    return [];
  }

  return resp.json();
}

export async function login(password: string): Promise<string> {
  const resp = await fetch(`${API_BASE}/api/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ password }),
  });

  if (!resp.ok) {
    const err = await resp.json().catch(() => ({ error: resp.statusText }));
    throw new Error(err.error || "Login failed");
  }

  const data = await resp.json();
  localStorage.setItem("audiobot_token", data.token);
  return data.token;
}

export async function checkAuth(): Promise<boolean> {
  const token = localStorage.getItem("audiobot_token");
  if (!token) return false;

  const resp = await fetch(`${API_BASE}/api/auth/check`, {
    headers: { Authorization: `Bearer ${token}` },
  });

  if (!resp.ok) return false;

  const data = await resp.json();
  return data.valid === true;
}

export async function getAuthMode(): Promise<boolean> {
  try {
    const resp = await fetch(`${API_BASE}/api/auth/mode`);
    if (!resp.ok) return false;
    const data = await resp.json();
    return data.electron === true;
  } catch {
    return false;
  }
}

export function createStatusWS(): WebSocket | null {
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  const wsUrl = `${proto}//${window.location.host}/api/room/status`;
  return new WebSocket(wsUrl);
}
