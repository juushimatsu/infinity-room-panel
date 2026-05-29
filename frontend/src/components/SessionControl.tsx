interface SessionControlProps {
  canStart: boolean;
  sessionActive: boolean;
  onStart: () => void;
  onStop: () => void;
}

function SessionControl({ canStart, sessionActive, onStart, onStop }: SessionControlProps) {
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
}

export default SessionControl;
