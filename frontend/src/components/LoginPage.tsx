import { useState } from "preact/hooks";
import { JSX } from "preact";
import { login } from "../api/client";

interface LoginPageProps {
  onLogin: () => void;
}

function LoginPage({ onLogin }: LoginPageProps) {
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: JSX.TargetedEvent<HTMLFormElement, Event>) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      await login(password);
      onLogin();
    } catch (err: any) {
      setError(err.message || 'Неверный пароль');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="login-container">
      <div className="login-card">
        <h2 className="login-title">Вход в панель</h2>
        {error && <div className="login-error">{error}</div>}
        <form onSubmit={handleSubmit}>
          <input
            type="password"
            className="text-input"
            placeholder="Пароль"
            value={password}
            onInput={(e) => setPassword((e.target as HTMLInputElement).value)}
            autoFocus
          />
          <button
            className="btn btn-primary login-btn"
            type="submit"
            disabled={loading || !password}
          >
            {loading ? 'Вход...' : 'Войти'}
          </button>
        </form>
      </div>
    </div>
  );
}

export default LoginPage;
