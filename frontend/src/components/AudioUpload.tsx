import { useRef, useState } from 'preact/hooks';
import { JSX } from 'preact';
import { AudioFile, uploadAudio } from '../api/client';

interface AudioUploadProps {
  selectedFileId: string | null;
  files: AudioFile[];
  onFileSelect: (id: string) => void;
  onUpload: (file: AudioFile) => void;
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B';
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
}

function AudioUpload({ selectedFileId, files, onFileSelect, onUpload }: AudioUploadProps) {
  const [dragOver, setDragOver] = useState(false);
  const [uploading, setUploading] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  const handleFile = async (file: File) => {
    if (!file.name.endsWith('.mp3')) {
      alert('Пожалуйста, загрузите MP3-файл');
      return;
    }
    setUploading(true);
    try {
      const result = await uploadAudio(file);
      onUpload({
        id: result.id,
        filename: result.filename,
        size: file.size,
        uploaded_at: new Date().toISOString(),
      });
    } catch (err: any) {
      alert(err.message || 'Ошибка загрузки');
    } finally {
      setUploading(false);
    }
  };

  const handleDrop = (e: JSX.TargetedDragEvent<HTMLDivElement>) => {
    e.preventDefault();
    setDragOver(false);
    const file = e.dataTransfer?.files[0];
    if (file) handleFile(file);
  };

  const handleInputChange = (e: JSX.TargetedEvent<HTMLInputElement, Event>) => {
    const target = e.target as HTMLInputElement;
    const file = target.files?.[0];
    if (file) handleFile(file);
    target.value = '';
  };

  return (
    <div className="section">
      <label className="section-label">Аудио</label>
      <div
        className={`upload-zone ${dragOver ? 'drag-over' : ''}`}
        onDragOver={(e) => { e.preventDefault(); setDragOver(true); }}
        onDragLeave={() => setDragOver(false)}
        onDrop={handleDrop}
        onClick={() => inputRef.current?.click()}
      >
        <div className="upload-icon">🎵</div>
        <div className="upload-zone-text">
          {uploading ? 'Загрузка...' : 'Перетащите MP3-файл сюда или нажмите для выбора'}
        </div>
        <input
          ref={inputRef}
          type="file"
          accept=".mp3"
          style={{ display: 'none' }}
          onChange={handleInputChange}
        />
      </div>
      {files.length > 0 && (
        <ul className="file-list">
          {files.map((f) => (
            <li
              key={f.id}
              className={`file-item ${selectedFileId === f.id ? 'selected' : ''}`}
              onClick={() => onFileSelect(f.id)}
            >
              <span className="file-name">{f.filename}</span>
              <span className="file-size">{formatSize(f.size)}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

export default AudioUpload;
