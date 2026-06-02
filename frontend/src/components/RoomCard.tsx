import { RoomInfo, BotStatus } from "../api/client";

const serviceLabels: Record<string, string> = {
  jitsi: "Jitsi",
  telemost: "Телемост",
  wbstream: "WB Stream",
};

const serviceColors: Record<string, string> = {
  jitsi: "jitsi",
  telemost: "telemost",
  wbstream: "wbstream",
};

const statusLabels: Record<string, string> = {
  connecting: "Подключение...",
  active: "Активен",
  error: "Ошибка",
  stopped: "Остановлен",
};

const statusBadgeClass: Record<string, string> = {
  connecting: "badge-connecting",
  active: "badge-active",
  error: "badge-error",
  stopped: "badge-stopped",
};

function getInitials(name: string): string {
  const parts = name.split(" ");
  if (parts.length >= 2) {
    return (parts[0][0] + parts[1][0]).toUpperCase();
  }
  return name.substring(0, 2).toUpperCase();
}

interface RoomCardProps {
  room: RoomInfo;
  onStop: () => void;
  onStart: () => void;
  onRestart: () => void;
  onEdit: () => void;
  onDelete: () => void;
}

const RoomCard = ({
  room,
  onStop,
  onStart,
  onRestart,
  onEdit,
  onDelete,
}: RoomCardProps) => {
  const activeBots = room.bots.filter(
    (b) => b.status === "active" || b.status === "connecting",
  );
  const hasError = room.bots.some((b) => b.status === "error");
  const isActive = room.active;

  return (
    <div className="card room-card">
      <div className="card-header-row">
        <div className="room-info">
          <span className={`service-badge ${serviceColors[room.service]}`}>
            {serviceLabels[room.service] || room.service}
          </span>
          <span className="room-input-text">{room.room_input}</span>
        </div>
        <div className="room-actions">
          {isActive ? (
            <>
              <button
                className="btn btn-secondary btn-sm"
                onClick={onRestart}
                type="button"
                title="Перезапустить"
              >
                ⟳
              </button>
              <button
                className="btn btn-secondary btn-sm"
                onClick={onEdit}
                type="button"
                title="Редактировать"
              >
                ✎
              </button>
              <button
                className="btn btn-secondary btn-sm"
                onClick={onStop}
                type="button"
              >
                Остановить
              </button>
            </>
          ) : (
            <>
              <button
                className="btn btn-primary btn-sm"
                onClick={onStart}
                type="button"
              >
                Запустить
              </button>
              <button
                className="btn btn-secondary btn-sm"
                onClick={onEdit}
                type="button"
                title="Редактировать"
              >
                ✎
              </button>
              <button
                className="btn btn-danger btn-sm"
                onClick={onDelete}
                type="button"
              >
                Удалить
              </button>
            </>
          )}
        </div>
      </div>

      {room.bots.length > 0 && isActive && (
        <div className="bot-status-list">
          {room.bots.map((bot) => (
            <div key={bot.bot_id} className="bot-status-card">
              <div
                className={`bot-avatar ${serviceColors[bot.service || room.service]}`}
              >
                {getInitials(bot.name)}
              </div>
              <div className="bot-info">
                <div className="bot-name">{bot.name}</div>
                {bot.error && <div className="bot-error">{bot.error}</div>}
              </div>
              <span className={`badge ${statusBadgeClass[bot.status]}`}>
                {statusLabels[bot.status]}
              </span>
            </div>
          ))}
        </div>
      )}

      {!isActive && (
        <div className="room-inactive-hint">
          Комната остановлена. Нажмите «Запустить» для запуска ботов.
        </div>
      )}
    </div>
  );
};

export default RoomCard;
