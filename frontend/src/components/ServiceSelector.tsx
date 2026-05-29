import React from 'react';

interface ServiceSelectorProps {
  value: 'jitsi' | 'telemost' | 'wbstream';
  onChange: (service: 'jitsi' | 'telemost' | 'wbstream') => void;
}

const services: { key: 'jitsi' | 'telemost' | 'wbstream'; label: string }[] = [
  { key: 'jitsi', label: 'Jitsi' },
  { key: 'telemost', label: 'Telemost' },
  { key: 'wbstream', label: 'WB Stream' },
];

const ServiceSelector: React.FC<ServiceSelectorProps> = ({ value, onChange }) => {
  return (
    <div className="service-selector">
      {services.map((s) => (
        <button
          key={s.key}
          className={`pill-tab ${value === s.key ? 'active' : ''}`}
          onClick={() => onChange(s.key)}
          type="button"
        >
          {s.label}
        </button>
      ))}
    </div>
  );
};

export default ServiceSelector;
