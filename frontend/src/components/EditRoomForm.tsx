import { useState } from "preact/hooks";
import ServiceSelector from "./ServiceSelector";
import RoomInput from "./RoomInput";
import BotConfig from "./BotConfig";
import { AudioFile } from "../api/client";

interface EditRoomFormProps {
  files: AudioFile[];
  room: {
    id: string;
    service: string;
    room_input: string;
    active: boolean;
  };
  initialConfig: {
    service: string;
    room_input: string;
    bot_count: number;
    file_id: string;
    loop: boolean;
  };
  onUpdate: (params: {
    room_id: string;
    service: string;
    room_input: string;
    bot_count: number;
    file_id: string;
    loop: boolean;
  }) => void;
  onCancel: () => void;
}

function EditRoomForm({
  files,
  room,
  initialConfig,
  onUpdate,
  onCancel,
}: EditRoomFormProps) {
  const [service, setService] = useState<
    "jitsi" | "telemost" | "wbstream"
  >(initialConfig.service as "jitsi" | "telemost" | "wbstream");
  const [roomInput, setRoomInput] = useState(initialConfig.room_input);
  const [botCount, setBotCount] = useState(initialConfig.bot_count);
  const [loop, setLoop] = useState(initialConfig.loop);
  const [selectedFileId, setSelectedFileId] = useState<string | null>(
    initialConfig.file_id,
  );

  const canUpdate = !!selectedFileId && !!roomInput;

  return (
    <div className="card new-room-form">
      <div className="card-header-row">
        <h2 className="card-title">Редактировать комнату</h2>
        <button className="btn btn-secondary btn-sm" onClick={onCancel} type="button">
          Отмена
        </button>
      </div>

      <div className="form-section">
        <label className="section-label">Сервис</label>
        <ServiceSelector value={service} onChange={setService} />
      </div>

      <div className="form-section">
        <RoomInput service={service} value={roomInput} onChange={setRoomInput} />
      </div>

      <div className="form-section">
        <BotConfig
          botCount={botCount}
          loop={loop}
          onBotCountChange={setBotCount}
          onLoopChange={setLoop}
        />
      </div>

      <div className="form-section">
        <label className="section-label">Аудиофайл</label>
        <ul className="file-list">
          {files.map((file) => (
            <li
              key={file.id}
              className={`file-item ${selectedFileId === file.id ? "selected" : ""}`}
              onClick={() => setSelectedFileId(file.id)}
            >
              <span className="file-name">{file.filename}</span>
              <span className="file-size">{(file.size / 1024 / 1024).toFixed(1)} MB</span>
            </li>
          ))}
        </ul>
      </div>

      <div className="form-actions">
        <button
          className="btn btn-primary"
          disabled={!canUpdate}
          onClick={() =>
            selectedFileId &&
            onUpdate({
              room_id: room.id,
              service,
              room_input: roomInput,
              bot_count: botCount,
              file_id: selectedFileId,
              loop,
            })
          }
          type="button"
        >
          Сохранить
        </button>
      </div>
    </div>
  );
}

export default EditRoomForm;
