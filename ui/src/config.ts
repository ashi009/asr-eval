interface ColorConfig {
  dot: string;
  ring: string;
  text: string;
  border: string;
}

export interface ASRProviderConfigItem {
  name: string;
  enabled?: boolean;
  color: ColorConfig;
}

export const ASR_PROVIDER_CONFIG: Record<string, ASRProviderConfigItem> = {
  // Volcengine
  'volc': {
    name: 'Volcengine',
    color: { dot: 'bg-cyan-500', ring: 'bg-cyan-100', text: 'text-cyan-700', border: 'border-cyan-200' }
  },
  'volc_ctx': {
    name: 'Volcengine Context',
    color: { dot: 'bg-cyan-600', ring: 'bg-cyan-100', text: 'text-cyan-800', border: 'border-cyan-300' }
  },
  'volc_ctx_rt': {
    name: 'Volcengine Realtime',
    color: { dot: 'bg-cyan-600', ring: 'bg-cyan-100', text: 'text-cyan-800', border: 'border-cyan-300' }
  },
  'volc2_ctx': {
    name: 'Volcengine2 with Context',
    color: { dot: 'bg-cyan-700', ring: 'bg-cyan-100', text: 'text-cyan-900', border: 'border-cyan-400' }
  },
  'volc2_ctx_rt': {
    name: 'Volcengine2 with Context Realtime',
    color: { dot: 'bg-cyan-700', ring: 'bg-cyan-100', text: 'text-cyan-900', border: 'border-cyan-400' }
  },

  // Qwen
  'qwen_ctx_rt': {
    name: 'Qwen with Context Realtime',
    color: { dot: 'bg-red-500', ring: 'bg-red-100', text: 'text-red-700', border: 'border-red-200' }
  },

  // iFlyTek
  'ifly': {
    name: 'iFlyTek',
    color: { dot: 'bg-orange-500', ring: 'bg-orange-100', text: 'text-orange-700', border: 'border-orange-200' }
  },
  'ifly_mq': {
    name: 'iFlyTek MQ',
    color: { dot: 'bg-orange-500', ring: 'bg-orange-100', text: 'text-orange-700', border: 'border-orange-200' }
  },
  'ifly_en': {
    name: 'iFlyTek English',
    color: { dot: 'bg-orange-500', ring: 'bg-orange-100', text: 'text-orange-700', border: 'border-orange-200' }
  },
  'iflybatch': {
    name: 'iFlyTek Batch',
    color: { dot: 'bg-orange-600', ring: 'bg-orange-100', text: 'text-orange-800', border: 'border-orange-300' }
  },

  // Deepgram
  'dg': {
    name: 'Deepgram',
    color: { dot: 'bg-emerald-500', ring: 'bg-emerald-100', text: 'text-emerald-700', border: 'border-emerald-200' }
  },

  // Sonix
  'snx': {
    name: 'Sonix',
    color: { dot: 'bg-indigo-500', ring: 'bg-indigo-100', text: 'text-indigo-700', border: 'border-indigo-200' }
  },
  'snxrt': {
    name: 'Sonix Realtime',
    color: { dot: 'bg-indigo-600', ring: 'bg-indigo-100', text: 'text-indigo-800', border: 'border-indigo-300' }
  },

  // IST
  'ist_basic': {
    name: 'IST Basic',
    color: { dot: 'bg-yellow-500', ring: 'bg-yellow-100', text: 'text-yellow-700', border: 'border-yellow-200' }
  },

  'txt': {
    name: 'Human Transcription',
    color: { dot: 'bg-gray-500', ring: 'bg-gray-100', text: 'text-gray-700', border: 'border-gray-200' }
  },

  // Fallback
  'default': {
    name: 'Unknown Provider',
    color: { dot: 'bg-slate-500', ring: 'bg-slate-100', text: 'text-slate-700', border: 'border-slate-200' }
  }
};

/**
 * Global state for enabled providers, populated from backend config api
 * We wrap this in a config object to allow dynamic updates
 */
let ENABLED_PROVIDERS: Record<string, boolean> = {};

export const setEnabledProviders = (enabled: Record<string, boolean>) => {
  ENABLED_PROVIDERS = enabled;
};

export const isProviderEnabled = (id: string): boolean => {
  const key = id.toLowerCase();
  if (ENABLED_PROVIDERS[key] !== undefined) {
    return ENABLED_PROVIDERS[key];
  }
  return true; // Default to enabled if not specified (e.g. for unknown providers)
};

export const getASRProviderConfig = (id: string): ASRProviderConfigItem => {
  const key = id.toLowerCase();
  if (ASR_PROVIDER_CONFIG[key]) {
    return ASR_PROVIDER_CONFIG[key];
  }

  // Strict fallback behavior: Return uppercase ID with default color
  return {
    name: id.toUpperCase(),
    color: ASR_PROVIDER_CONFIG.default.color
  };
};
