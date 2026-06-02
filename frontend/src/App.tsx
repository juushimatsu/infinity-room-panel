import { useState, useEffect, useCallback, useRef } from "preact/hooks";
import NewRoomForm from "./components/NewRoomForm";
import RoomCard from "./components/RoomCard";
import LoginPage from "./components/LoginPage";
import EditRoomForm from "./components/EditRoomForm";
import WBAccountSettings from "./components/WBAccountSettings";
import {
  AudioFile,
  BotStatus,
  RoomInfo,
  listAudio,
  startRoom,
  stopRoom,
  deleteRoom,
  restartRoom,
  startRoomFromConfig,
  updateRoom,
  listRooms,
  checkAuth,
  getAuthMode,
  createStatusWS,
} from "./api/client";

function App() {
  const [isLoggedIn, setIsLoggedIn] = useState(false);
  const [isElectron, setIsElectron] = useState(false);
  const [authChecked, setAuthChecked] = useState(false);

  const [rooms, setRooms] = useState<Map<string, RoomInfo>>(new Map());
  const [showNewRoom, setShowNewRoom] = useState(false);
  const [showWBSettings, setShowWBSettings] = useState(false);
  const [editRoomId, setEditRoomId] = useState<string | null>(null);

  const [files, setFiles] = useState<AudioFile[]>([]);

  const wsRef = useRef<WebSocket | null>(null);

  // Determine auth mode on mount
  useEffect(() => {
    async function checkMode() {
      const electronMode = await getAuthMode();
      if (electronMode) {
        setIsElectron(true);
        setIsLoggedIn(true);
        setAuthChecked(true);
        return;
      }

      const electronAPI = (window as any).electronAPI;
      if (electronAPI && electronAPI.isElectron) {
        setIsElectron(true);
        setIsLoggedIn(true);
        setAuthChecked(true);
        return;
      }

      const valid = await checkAuth();
      setIsLoggedIn(valid);
      setAuthChecked(true);
    }

    checkMode();
  }, []);

  // Load audio files + initial rooms when logged in
  useEffect(() => {
    if (!isLoggedIn) return;
    listAudio()
      .then(setFiles)
      .catch(() => {});
    listRooms()
      .then((r) => {
        const map = new Map<string, RoomInfo>();
        r.forEach((room) => map.set(room.id, room));
        setRooms(map);
      })
      .catch(() => {});
  }, [isLoggedIn]);

  // WebSocket for bot status updates
  useEffect(() => {
    if (!isLoggedIn) return;

    const ws = createStatusWS();
    if (!ws) return;
    wsRef.current = ws;

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data) as BotStatus;
        const roomId = data.room_id;
        if (!roomId) return;

        setRooms((prev) => {
          const next = new Map(prev);
          const room = next.get(roomId);
          if (!room) return prev;

          const updatedBots = [...room.bots];
          const idx = updatedBots.findIndex((b) => b.bot_id === data.bot_id);
          if (idx >= 0) {
            updatedBots[idx] = { ...updatedBots[idx], ...data };
          } else {
            updatedBots.push(data);
          }

          next.set(roomId, { ...room, bots: updatedBots });
          return next;
        });
      } catch {}
    };

    ws.onclose = () => {
      wsRef.current = null;
    };

    return () => {
      ws.close();
      wsRef.current = null;
    };
  }, [isLoggedIn]);

  const handleStartRoom = useCallback(
    async (params: {
      service: string;
      room_input: string;
      bot_count: number;
      file_id: string;
      loop: boolean;
    }) => {
      try {
        const roomId = await startRoom(params);

        const newRoom: RoomInfo = {
          id: roomId,
          service: params.service,
          room_input: params.room_input,
          active: true,
          bot_count: params.bot_count,
          file_id: params.file_id,
          loop: params.loop,
          bots: Array.from({ length: params.bot_count }, (_, i) => ({
            bot_id: i + 1,
            name: "...",
            status: "connecting" as const,
            service: params.service,
            room_id: roomId,
          })),
        };

        setRooms((prev) => {
          const next = new Map(prev);
          next.set(roomId, newRoom);
          return next;
        });

        setShowNewRoom(false);
      } catch (err: any) {
        alert(err.message || "Ошибка запуска");
      }
    },
    [],
  );

  const handleStopRoom = useCallback(async (roomId: string) => {
    try {
      await stopRoom(roomId);
      setRooms((prev) => {
        const next = new Map(prev);
        const room = next.get(roomId);
        if (room) {
          next.set(roomId, { ...room, active: false, bots: [] });
        }
        return next;
      });
    } catch (err: any) {
      alert(err.message || "Ошибка остановки");
    }
  }, []);

  const handleDeleteRoom = useCallback(
    async (roomId: string) => {
      if (!confirm("Удалить комнату из панели?")) return;
      try {
        await deleteRoom(roomId);
        setRooms((prev) => {
          const next = new Map(prev);
          next.delete(roomId);
          return next;
        });
        if (editRoomId === roomId) setEditRoomId(null);
      } catch (err: any) {
        alert(err.message || "Ошибка удаления");
      }
    },
    [editRoomId],
  );

  const handleRestartRoom = useCallback(async (roomId: string) => {
    try {
      await restartRoom(roomId);
      setRooms((prev) => {
        const next = new Map(prev);
        const room = next.get(roomId);
        if (room) {
          next.set(roomId, { ...room, active: true, bots: [] });
        }
        return next;
      });
    } catch (err: any) {
      alert(err.message || "Ошибка перезапуска");
    }
  }, []);

  const handleStartFromConfig = useCallback(async (roomId: string) => {
    try {
      await startRoomFromConfig(roomId);
      setRooms((prev) => {
        const next = new Map(prev);
        const room = next.get(roomId);
        if (room) {
          next.set(roomId, { ...room, active: true, bots: [] });
        }
        return next;
      });
    } catch (err: any) {
      alert(err.message || "Ошибка запуска");
    }
  }, []);

  const handleUpdateRoom = useCallback(
    async (params: {
      room_id: string;
      service: string;
      room_input: string;
      bot_count: number;
      file_id: string;
      loop: boolean;
    }) => {
      try {
        await updateRoom(params);
        setRooms((prev) => {
          const next = new Map(prev);
          const room = next.get(params.room_id);
          if (room) {
            next.set(params.room_id, {
              ...room,
              service: params.service,
              room_input: params.room_input,
              bot_count: params.bot_count,
              file_id: params.file_id,
              loop: params.loop,
              bots: room.active ? [] : room.bots,
            });
          }
          return next;
        });
        setEditRoomId(null);
      } catch (err: any) {
        alert(err.message || "Ошибка обновления");
      }
    },
    [],
  );

  const handleUpload = useCallback((file: AudioFile) => {
    setFiles((prev) => [...prev, file]);
  }, []);

  const handleLogin = useCallback(() => {
    setIsLoggedIn(true);
    listAudio()
      .then(setFiles)
      .catch(() => {});
    listRooms()
      .then((r) => {
        const map = new Map<string, RoomInfo>();
        r.forEach((room) => map.set(room.id, room));
        setRooms(map);
      })
      .catch(() => {});
  }, []);

  if (!authChecked) {
    return (
      <div
        className="app-container"
        style={{ textAlign: "center", paddingTop: 80 }}
      >
        Загрузка...
      </div>
    );
  }

  if (!isLoggedIn) {
    return <LoginPage onLogin={handleLogin} />;
  }

  const roomList = Array.from(rooms.values());
  const editingRoom = editRoomId ? rooms.get(editRoomId) : undefined;

  return (
    <div className="app-container">
      <div className="app-header">
        <h1>AudioBot Panel</h1>
        <p>Управление аудио-ботами для WebRTC-комнат</p>
      </div>

      {showWBSettings && (
        <div style={{ marginBottom: 24 }}>
          <WBAccountSettings onClose={() => setShowWBSettings(false)} />
        </div>
      )}

      {editingRoom && (
        <div style={{ marginBottom: 24 }}>
          <EditRoomForm
            files={files}
            room={editingRoom}
            initialConfig={{
              service: editingRoom.service,
              room_input: editingRoom.room_input,
              bot_count: editingRoom.bot_count || 2,
              file_id: editingRoom.file_id || "",
              loop: editingRoom.loop || true,
            }}
            onUpdate={handleUpdateRoom}
            onCancel={() => setEditRoomId(null)}
          />
        </div>
      )}

      {roomList.length > 0 && (
        <div className="rooms-list">
          {roomList.map((room) => (
            <RoomCard
              key={room.id}
              room={room}
              onStop={() => handleStopRoom(room.id)}
              onStart={() => handleStartFromConfig(room.id)}
              onRestart={() => handleRestartRoom(room.id)}
              onEdit={() => setEditRoomId(room.id)}
              onDelete={() => handleDeleteRoom(room.id)}
            />
          ))}
        </div>
      )}

      {roomList.length === 0 && !showNewRoom && (
        <div className="empty-state">Нет комнат. Добавьте новую комнату.</div>
      )}

      {showNewRoom && (
        <NewRoomForm
          files={files}
          onUpload={handleUpload}
          onStart={handleStartRoom}
          onCancel={() => setShowNewRoom(false)}
        />
      )}

      {!showNewRoom && (
        <div className="section" style={{ display: "flex", gap: 12 }}>
          <button
            className="btn btn-primary"
            onClick={() => setShowNewRoom(true)}
            type="button"
          >
            + Новая комната
          </button>
          <button
            className="btn btn-secondary"
            onClick={() => setShowWBSettings(true)}
            type="button"
          >
            ⚙️ WB Аккаунт
          </button>
        </div>
      )}
    </div>
  );
}

export default App;
