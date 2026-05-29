import React, { useState } from "react";
import ServiceSelector from "./ServiceSelector";
import RoomInput from "./RoomInput";
import BotConfig from "./BotConfig";
import AudioUpload from "./AudioUpload";
import { AudioFile } from "../api/client";

interface NewRoomFormProps {
  files: AudioFile[];
  onUpload: (file: AudioFile) => void;
  onStart: (params: {
    service: string;
    room_input: string;
    bot_count: number;
    file_id: string;
    loop: boolean;
  }) => void;
  onCancel: () => void;
}

const NewRoomForm: React.FC<NewRoomFormProps> = ({
  files,
  onUpload,
  onStart,
  onCancel,
}) => {
  const [service, setService] = useState<"jitsi" | "telemost" | "wbstream">(
    "jitsi",
  );
  const [roomInput, setRoomInput] = useState("");
  const [botCount, setBotCount] = useState(2);
  const [loop, setLoop] = useState(true);
  const [selectedFileId, setSelectedFileId] = useState<string | null>(null);

  const canStart = !!selectedFileId && !!roomInput;

  return (
    <div className="card new-room-form">
      <div className="card-header-row">
        <h2 className="card-title">Новая комната</h2>
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
        <AudioUpload
          selectedFileId={selectedFileId}
          files={files}
          onFileSelect={setSelectedFileId}
          onUpload={onUpload}
        />
      </div>

      <div className="form-actions">
        <button
          className="btn btn-primary"
          disabled={!canStart}
          onClick={() =>
            selectedFileId &&
            onStart({
              service,
              room_input: roomInput,
              bot_count: botCount,
              file_id: selectedFileId,
              loop,
            })
          }
          type="button"
        >
          Запустить
        </button>
      </div>
    </div>
  );
};

export default NewRoomForm;
