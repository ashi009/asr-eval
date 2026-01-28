interface ColorConfig {
  dot: string;
  ring: string;
  text: string;
  border: string;
}

export interface ServiceConfigItem {
  name: string;
  enabled?: boolean;
  color: ColorConfig;
}

export const SERVICE_CONFIG: Record<string, ServiceConfigItem> = {
  // Volcengine
  'volc': {
    name: 'Volcengine',
    enabled: true,
    color: { dot: 'bg-cyan-500', ring: 'bg-cyan-100', text: 'text-cyan-700', border: 'border-cyan-200' }
  },
  'volc_ctx': {
    name: 'Volcengine Context',
    enabled: true,
    color: { dot: 'bg-cyan-600', ring: 'bg-cyan-100', text: 'text-cyan-800', border: 'border-cyan-300' }
  },
  'volc_ctx_rt': {
    name: 'Volcengine Realtime',
    enabled: true,
    color: { dot: 'bg-cyan-600', ring: 'bg-cyan-100', text: 'text-cyan-800', border: 'border-cyan-300' }
  },
  'volc2_ctx': {
    name: 'Volcengine2 with Context',
    enabled: true,
    color: { dot: 'bg-cyan-700', ring: 'bg-cyan-100', text: 'text-cyan-900', border: 'border-cyan-400' }
  },
  'volc2_ctx_rt': {
    name: 'Volcengine2 with Context Realtime',
    enabled: true,
    color: { dot: 'bg-cyan-700', ring: 'bg-cyan-100', text: 'text-cyan-900', border: 'border-cyan-400' }
  },

  // Qwen
  'qwen_ctx_rt': {
    name: 'Qwen with Context Realtime',
    enabled: true,
    color: { dot: 'bg-red-500', ring: 'bg-red-100', text: 'text-red-700', border: 'border-red-200' }
  },

  // iFlyTek
  'ifly': {
    name: 'iFlyTek',
    enabled: true,
    color: { dot: 'bg-orange-500', ring: 'bg-orange-100', text: 'text-orange-700', border: 'border-orange-200' }
  },
  'ifly_mq': {
    name: 'iFlyTek MQ',
    enabled: true,
    color: { dot: 'bg-orange-500', ring: 'bg-orange-100', text: 'text-orange-700', border: 'border-orange-200' }
  },
  'ifly_en': {
    name: 'iFlyTek English',
    enabled: false,
    color: { dot: 'bg-orange-500', ring: 'bg-orange-100', text: 'text-orange-700', border: 'border-orange-200' }
  },
  'iflybatch': {
    name: 'iFlyTek Batch',
    enabled: false,
    color: { dot: 'bg-orange-600', ring: 'bg-orange-100', text: 'text-orange-800', border: 'border-orange-300' }
  },

  // Deepgram
  'dg': {
    name: 'Deepgram',
    enabled: false,
    color: { dot: 'bg-emerald-500', ring: 'bg-emerald-100', text: 'text-emerald-700', border: 'border-emerald-200' }
  },

  // Sonix
  'snx': {
    name: 'Sonix',
    enabled: true,
    color: { dot: 'bg-indigo-500', ring: 'bg-indigo-100', text: 'text-indigo-700', border: 'border-indigo-200' }
  },
  'snxrt': {
    name: 'Sonix Realtime',
    enabled: true,
    color: { dot: 'bg-indigo-600', ring: 'bg-indigo-100', text: 'text-indigo-800', border: 'border-indigo-300' }
  },

  // IST
  'ist_basic': {
    name: 'IST Basic',
    enabled: true,
    color: { dot: 'bg-yellow-500', ring: 'bg-yellow-100', text: 'text-yellow-700', border: 'border-yellow-200' }
  },

  'txt': {
    name: 'Human Transcription',
    enabled: true,
    color: { dot: 'bg-gray-500', ring: 'bg-gray-100', text: 'text-gray-700', border: 'border-gray-200' }
  },

  // Fallback
  'default': {
    name: 'Unknown Service',
    enabled: true,
    color: { dot: 'bg-slate-500', ring: 'bg-slate-100', text: 'text-slate-700', border: 'border-slate-200' }
  }
};

export const getServiceConfig = (id: string): ServiceConfigItem => {
  const key = id.toLowerCase();
  if (SERVICE_CONFIG[key]) {
    return SERVICE_CONFIG[key];
  }

  // Strict fallback behavior: Return uppercase ID with default color
  return {
    name: id.toUpperCase(),
    color: SERVICE_CONFIG.default.color
  };
};
