import { useState, useEffect, useCallback, useRef } from "preact/hooks";
import NewRoomForm from "./components/NewRoomForm";
import RoomCard from "./components/RoomCard";
import AudioUpload from "./components/AudioUpload";
import LoginPage from "./components/LoginPage";
import {
  AudioFile,
  BotStatus,
  RoomInfo,
  listAudio,
  uploadAudio,
  startRoom,
  stopRoom,
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
    listAudio().then(setFiles).catch(() => {});
    listRooms().then((r) => {
      const map = new Map<string, RoomInfo>();
      r.forEach((room) => map.set(room.id, room));
      setRooms(map);
    }).catch(() => {});
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
        next.delete(roomId);
        return next;
      });
    } catch (err: any) {
      alert(err.message || "Ошибка остановки");
    }
  }, []);

  const handleUpload = useCallback((file: AudioFile) => {
    setFiles((prev) => [...prev, file]);
  }, []);

  const handleLogin = useCallback(() => {
    setIsLoggedIn(true);
    listAudio().then(setFiles).catch(() => {});
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

  return (
    <div className="app-container">
      <div className="app-header">
        <h1>AudioBot Panel</h1>
        <p>Управление аудио-ботами для WebRTC-комнат</p>
      </div>

      {roomList.length > 0 && (
        <div className="rooms-list">
          {roomList.map((room) => (
            <RoomCard
              key={room.id}
              room={room}
              onStop={() => handleStopRoom(room.id)}
            />
          ))}
        </div>
      )}

      {roomList.length === 0 && !showNewRoom && (
        <div className="empty-state">
          Нет активных комнат. Запустите ботов в комнату.
        </div>
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
        <div className="section">
          <button
            className="btn btn-primary"
            onClick={() => setShowNewRoom(true)}
            type="button"
          >
            + Новая комната
          </button>
        </div>
      )}
    </div>
  );
}

export default App;
