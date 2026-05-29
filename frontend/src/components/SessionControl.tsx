import React from 'react';

interface SessionControlProps {
  canStart: boolean;
  sessionActive: boolean;
  onStart: () => void;
  onStop: () => void;
}

const SessionControl: React.FC<SessionControlProps> = ({ canStart, sessionActive, onStart, onStop }) => {
  return (
    <div className="section">
      <div className="session-controls">
        <button
          className="btn btn-primary"
          disabled={!canStart || sessionActive}
          onClick={onStart}
          type="button"
        >
          Запустить
        </button>
        <button
          className="btn btn-secondary"
          disabled={!sessionActive}
          onClick={onStop}
          type="button"
        >
          Остановить
        </button>
      </div>
    </div>
  );
};

export default SessionControl;
