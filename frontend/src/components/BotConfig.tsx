import React from 'react';

interface BotConfigProps {
  botCount: number;
  loop: boolean;
  onBotCountChange: (count: number) => void;
  onLoopChange: (loop: boolean) => void;
}

const BotConfig: React.FC<BotConfigProps> = ({ botCount, loop, onBotCountChange, onLoopChange }) => {
  return (
    <div className="section">
      <label className="section-label">Параметры ботов</label>
      <div className="card">
        <div className="bot-config">
          <div className="config-field">
            <label>Количество ботов</label>
            <input
              type="number"
              className="number-input"
              min={1}
              max={3}
              value={botCount}
              onChange={(e) => {
                const v = parseInt(e.target.value, 10);
                if (v >= 1 && v <= 3) onBotCountChange(v);
              }}
            />
          </div>
          <div className="toggle-container">
            <label className="toggle">
              <input
                type="checkbox"
                checked={loop}
                onChange={(e) => onLoopChange(e.target.checked)}
              />
              <span className="toggle-slider" />
            </label>
            <span className="toggle-label">Зациклить аудио</span>
          </div>
        </div>
      </div>
    </div>
  );
};

export default BotConfig;
