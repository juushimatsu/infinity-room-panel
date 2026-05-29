import React from 'react';
import { BotStatus } from '../api/client';

interface BotStatusCardProps {
  bots: BotStatus[];
}

const statusLabels: Record<string, string> = {
  connecting: 'Подключение...',
  active: 'Активен',
  error: 'Ошибка',
  stopped: 'Остановлен',
};

const statusBadgeClass: Record<string, string> = {
  connecting: 'badge-connecting',
  active: 'badge-active',
  error: 'badge-error',
  stopped: 'badge-stopped',
};

const serviceColors: Record<string, string> = {
  jitsi: 'jitsi',
  telemost: 'telemost',
  wbstream: 'wbstream',
};

function getInitials(name: string): string {
  const parts = name.split(' ');
  if (parts.length >= 2) {
    return (parts[0][0] + parts[1][0]).toUpperCase();
  }
  return name.substring(0, 2).toUpperCase();
}

const BotStatusCard: React.FC<BotStatusCardProps> = ({ bots }) => {
  if (bots.length === 0) {
    return (
      <div className="section">
        <label className="section-label">Статус ботов</label>
        <div className="empty-state">Нет активных ботов</div>
      </div>
    );
  }

  return (
    <div className="section">
      <label className="section-label">Статус ботов</label>
      <div className="bot-status-list">
        {bots.map((bot) => (
          <div key={bot.bot_id} className="bot-status-card">
            <div className={`bot-avatar ${serviceColors[bot.service || 'jitsi']}`}>
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
    </div>
  );
};

export default BotStatusCard;
