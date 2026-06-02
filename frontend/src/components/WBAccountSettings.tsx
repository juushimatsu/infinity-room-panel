import { useState, useEffect } from "preact/hooks";
import {
  WBAccountConfig,
  getWBAccount,
  setWBAccount,
  startWBAccount,
  stopWBAccount,
} from "../api/client";

interface WBAccountSettingsProps {
  onClose: () => void;
}

function WBAccountSettings({ onClose }: WBAccountSettingsProps) {
  const [cfg, setCfg] = useState<WBAccountConfig>({
    enabled: false,
    cookies: "",
    access_token: "",
    user_agent: "",
    display_name: "",
    interval_sec: 300,
    stay_duration_sec: 5,
  });
  const [jsonDump, setJsonDump] = useState("");
  const [dumpError, setDumpError] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [starting, setStarting] = useState(false);

  useEffect(() => {
    getWBAccount()
      .then((data) => {
        setCfg({
          enabled: data.enabled ?? false,
          cookies: data.cookies ?? "",
          access_token: data.access_token ?? "",
          user_agent: data.user_agent ?? "",
          display_name: data.display_name ?? "",
          interval_sec: data.interval_sec ?? 300,
          stay_duration_sec: data.stay_duration_sec ?? 5,
        });
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  function parseJsonDump(dump: string): Partial<WBAccountConfig> | null {
    try {
      const data = JSON.parse(dump);
      const result: Partial<WBAccountConfig> = {};

      if (data.userAgent && typeof data.userAgent === "string") {
        result.user_agent = data.userAgent;
      }
      if (data.cookies && typeof data.cookies === "string") {
        result.cookies = data.cookies;
      }

      // Extract accessToken from localStorage.wb_auth_auth_slice
      const authSlice = data.localStorage?.wb_auth_auth_slice;
      if (authSlice && typeof authSlice === "string") {
        try {
          const authData = JSON.parse(authSlice);
          if (
            authData.accessToken &&
            typeof authData.accessToken === "string"
          ) {
            result.access_token = authData.accessToken;
          }
        } catch {
          // authSlice might be the token directly
          if (authSlice.length > 50) {
            result.access_token = authSlice;
          }
        }
      }

      return result;
    } catch (e) {
      setDumpError("Невалидный JSON");
      return null;
    }
  }

  function handleParseDump() {
    setDumpError("");
    const parsed = parseJsonDump(jsonDump);
    if (!parsed) return;

    if (!parsed.access_token && !parsed.cookies) {
      setDumpError("Не удалось найти access_token или cookies в JSON");
      return;
    }

    setCfg((prev) => ({
      ...prev,
      ...parsed,
    }));
    setJsonDump("");
    setDumpError("Разобрано успешно — поля ниже заполнены автоматически");
  }

  const handleSave = async () => {
    setSaving(true);
    try {
      await setWBAccount(cfg);
      alert("Сохранено");
    } catch (e: any) {
      alert(e.message || "Ошибка сохранения");
    } finally {
      setSaving(false);
    }
  };

  const handleStart = async () => {
    setStarting(true);
    try {
      await startWBAccount();
      setCfg((prev) => ({ ...prev, enabled: true }));
      alert("Бот запущен");
    } catch (e: any) {
      alert(e.message || "Ошибка запуска");
    } finally {
      setStarting(false);
    }
  };

  const handleStopNow = async () => {
    try {
      await stopWBAccount();
      const data = await getWBAccount();
      setCfg({
        enabled: data.enabled ?? false,
        cookies: data.cookies ?? "",
        access_token: data.access_token ?? "",
        user_agent: data.user_agent ?? "",
        display_name: data.display_name ?? "",
        interval_sec: data.interval_sec ?? 300,
        stay_duration_sec: data.stay_duration_sec ?? 5,
      });
      alert("Бот остановлен");
    } catch (e: any) {
      alert(e.message || "Ошибка остановки");
    }
  };

  if (loading) {
    return (
      <div className="card">
        <div className="card-header-row">
          <h2 className="card-title">WB Stream — аккаунт для антиотключения</h2>
          <button
            className="btn btn-secondary btn-sm"
            onClick={onClose}
            type="button"
          >
            Закрыть
          </button>
        </div>
        <p>Загрузка...</p>
      </div>
    );
  }

  return (
    <div className="card">
      <div className="card-header-row">
        <h2 className="card-title">WB Stream — аккаунт для антиотключения</h2>
        <button
          className="btn btn-secondary btn-sm"
          onClick={onClose}
          type="button"
        >
          Закрыть
        </button>
      </div>

      {/* JSON Dump */}
      <div className="form-section">
        <label className="section-label">
          JSON dump из WB Stream (вставьте целиком — поля разберутся
          автоматически)
        </label>
        <textarea
          className="text-input"
          style={{
            minHeight: 100,
            resize: "vertical",
            fontSize: 12,
            fontFamily: "monospace",
          }}
          placeholder={`Скопируйте JSON из консоли после выполнения скрипта scripts/wb-extract-cookies.js в WB Stream...`}
          value={jsonDump}
          onChange={(e) => {
            setJsonDump((e.target as HTMLTextAreaElement).value);
            setDumpError("");
          }}
        />
        {dumpError && (
          <p
            style={{
              fontSize: 13,
              marginTop: 4,
              color: dumpError.includes("успешно")
                ? "var(--success)"
                : "var(--error)",
            }}
          >
            {dumpError}
          </p>
        )}
        <button
          className="btn btn-secondary btn-sm"
          onClick={handleParseDump}
          disabled={!jsonDump.trim()}
          type="button"
          style={{ marginTop: 8 }}
        >
          Разобрать JSON
        </button>
      </div>

      <hr style={{ borderColor: "var(--hairline)", margin: "var(--md) 0" }} />

      <div className="form-section">
        <div className="toggle-container">
          <label className="toggle">
            <input
              type="checkbox"
              checked={cfg.enabled}
              onChange={(e) =>
                setCfg({
                  ...cfg,
                  enabled: (e.target as HTMLInputElement).checked,
                })
              }
            />
            <span className="toggle-slider" />
          </label>
          <span className="toggle-label">
            {cfg.enabled ? "Включено" : "Отключено"}
          </span>
        </div>
      </div>

      <div className="form-section">
        <label className="section-label">
          Access Token (извлечён из JSON или вставьте вручную)
        </label>
        <textarea
          className="text-input"
          style={{ minHeight: 60, resize: "vertical", fontSize: 12 }}
          placeholder="eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9..."
          value={cfg.access_token}
          onChange={(e) =>
            setCfg({
              ...cfg,
              access_token: (e.target as HTMLTextAreaElement).value,
            })
          }
        />
      </div>

      <div className="form-section">
        <label className="section-label">Cookies (document.cookie)</label>
        <textarea
          className="text-input"
          style={{ minHeight: 60, resize: "vertical", fontSize: 12 }}
          placeholder="_wbauid=...; x_wbaas_token=..."
          value={cfg.cookies}
          onChange={(e) =>
            setCfg({ ...cfg, cookies: (e.target as HTMLTextAreaElement).value })
          }
        />
      </div>

      <div className="form-section">
        <label className="section-label">User-Agent</label>
        <input
          className="text-input"
          type="text"
          value={cfg.user_agent}
          onChange={(e) =>
            setCfg({ ...cfg, user_agent: (e.target as HTMLInputElement).value })
          }
        />
      </div>

      <div className="form-section">
        <label className="section-label">
          Имя бота (пусто = случайное, как у обычных ботов)
        </label>
        <input
          className="text-input"
          type="text"
          placeholder="Иван Иванов"
          value={cfg.display_name}
          onChange={(e) =>
            setCfg({
              ...cfg,
              display_name: (e.target as HTMLInputElement).value,
            })
          }
        />
      </div>

      <div className="form-section" style={{ display: "flex", gap: 20 }}>
        <div style={{ flex: 1 }}>
          <label className="section-label">Интервал между циклами (сек)</label>
          <input
            className="text-input"
            type="number"
            min={10}
            style={{ width: "100%" }}
            value={cfg.interval_sec}
            onChange={(e) =>
              setCfg({
                ...cfg,
                interval_sec:
                  parseInt((e.target as HTMLInputElement).value) || 300,
              })
            }
          />
        </div>
        <div style={{ flex: 1 }}>
          <label className="section-label">Время в комнате (сек)</label>
          <input
            className="text-input"
            type="number"
            min={1}
            style={{ width: "100%" }}
            value={cfg.stay_duration_sec}
            onChange={(e) =>
              setCfg({
                ...cfg,
                stay_duration_sec:
                  parseInt((e.target as HTMLInputElement).value) || 5,
              })
            }
          />
        </div>
      </div>

      <div className="form-actions" style={{ display: "flex", gap: 12 }}>
        <button
          className="btn btn-primary"
          onClick={handleSave}
          disabled={saving}
          type="button"
        >
          {saving ? "Сохранение..." : "Сохранить"}
        </button>
        <button
          className="btn btn-primary"
          onClick={handleStart}
          disabled={starting}
          type="button"
        >
          {starting ? "Запуск..." : "Запустить"}
        </button>
        <button
          className="btn btn-secondary"
          onClick={handleStopNow}
          type="button"
        >
          Остановить сейчас
        </button>
      </div>
    </div>
  );
}

export default WBAccountSettings;
