interface RoomInputProps {
  service: 'jitsi' | 'telemost' | 'wbstream';
  value: string;
  onChange: (value: string) => void;
}

const placeholders: Record<string, string> = {
  jitsi: 'https://meet.cryptopro.ru/e4r56y',
  telemost: 'https://telemost.yandex.ru/j/82696434768646',
  wbstream: '019e5e55-f062-78cc-8105-bbcdeed24cdd',
};

const labels: Record<string, string> = {
  jitsi: 'URL комнаты Jitsi',
  telemost: 'URL или ID конференции',
  wbstream: 'UUID комнаты',
};

function RoomInput({ service, value, onChange }: RoomInputProps) {
  return (
    <div className="section">
      <label className="section-label">{labels[service]}</label>
      <input
        type="text"
        className="text-input"
        placeholder={placeholders[service]}
        value={value}
        onInput={(e) => onChange((e.target as HTMLInputElement).value)}
      />
    </div>
  );
}

export default RoomInput;
