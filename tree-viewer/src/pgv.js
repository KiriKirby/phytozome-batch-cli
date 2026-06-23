import { jsPDF } from 'jspdf';
import 'svg2pdf.js';

export const PGV_FORMAT = 'phgo-viewer-snapshot';
export const PGV_SCHEMA_VERSION = 3;
export const PGV_MIN_SUPPORTED_SCHEMA_VERSION = 1;
export const PGV_VIEWER_STATE_SCHEMA_VERSION = 3;
export const PGV_REACTREE_STATE_SCHEMA_VERSION = 3;
const DEFAULT_BITMAP_EXPORT_SCALE = 10;
const MAX_BITMAP_EXPORT_PIXELS = 160_000_000;
export const TRANSPARENT_EXPORT_BACKGROUND = 'transparent';

function viewerStateForSnapshot(viewerState) {
  const state = viewerState && typeof viewerState === 'object' && !Array.isArray(viewerState)
    ? { ...viewerState }
    : {};
  state.schema_version = PGV_VIEWER_STATE_SCHEMA_VERSION;
  return state;
}

export function buildViewerSnapshot(payload, viewerState) {
  return {
    format: PGV_FORMAT,
    schema_version: PGV_SCHEMA_VERSION,
    created_at: new Date().toISOString(),
    producer: 'phytozome-go tree viewer',
    payload: payload || {},
    viewer_state: viewerStateForSnapshot(viewerState),
  };
}

export function parseViewerSnapshot(text) {
  let snapshot;
  try {
    snapshot = JSON.parse(String(text || ''));
  } catch {
    throw new Error('PGV snapshot is not valid JSON.');
  }
  if (!snapshot || snapshot.format !== PGV_FORMAT) {
    throw new Error('PGV snapshot format is not recognized.');
  }
  if (
    !Number.isInteger(snapshot.schema_version)
    || snapshot.schema_version < PGV_MIN_SUPPORTED_SCHEMA_VERSION
    || snapshot.schema_version > PGV_SCHEMA_VERSION
  ) {
    throw new Error(`Unsupported PGV schema version: ${snapshot.schema_version}`);
  }
  if (!snapshot.payload || typeof snapshot.payload !== 'object') {
    throw new Error('PGV snapshot is missing the viewer payload.');
  }
  return {
    ...snapshot,
    payload: snapshot.payload || {},
    viewer_state: snapshot.viewer_state && typeof snapshot.viewer_state === 'object' ? snapshot.viewer_state : {},
  };
}

function supportsSavePicker() {
  return typeof window !== 'undefined'
    && typeof window.showSaveFilePicker === 'function'
    && window.isSecureContext;
}

function sanitizeFilename(filename, fallback = 'download') {
  const trimmed = String(filename || '').trim();
  return trimmed || fallback;
}

function sanitizeFilenameBase(value, fallback = 'download') {
  return sanitizeFilename(value, fallback)
    .replace(/[<>:"/\\|?*\x00-\x1F]+/g, '_')
    .replace(/\s+/g, '_')
    .replace(/^_+|_+$/g, '')
    || fallback;
}

export function titleNumberFilenameBase(title, fallback = '') {
  const text = String(title || '').trim();
  const afterPrefix = text.includes(':') ? text.slice(text.indexOf(':') + 1).trim() : text;
  const match = afterPrefix.match(/(?:^|[^\d])(\d+(?:\.\d+)*)(?=$|[^\d.])/u)
    || text.match(/(?:^|[^\d])(\d+(?:\.\d+)*)(?=$|[^\d.])/u);
  return match ? sanitizeFilenameBase(match[1], fallback) : fallback;
}

export function defaultExportBaseName(payload, fallback = 'phgo-viewer') {
  const fromPayload = titleNumberFilenameBase(payload?.title || payload?.metadata?.title || '');
  if (fromPayload) return sanitizeFilenameBase(fromPayload, fallback);
  if (typeof document !== 'undefined') {
    const fromDocument = titleNumberFilenameBase(document.title || '');
    if (fromDocument) return sanitizeFilenameBase(fromDocument, fallback);
  }
  const session = sanitizeFilenameBase(payload?.session_id || '', '');
  return session || fallback;
}

function defaultedFilename(filename, payload, fallbackBase) {
  const raw = String(filename || '').trim();
  const dot = raw.lastIndexOf('.');
  const ext = dot >= 0 ? raw.slice(dot) : '';
  const base = dot >= 0 ? raw.slice(0, dot) : raw;
  const genericNames = new Set(['', 'tree', 'phylotree', 'phylo-tree', 'phgo-tree', 'phgo-viewer', 'snapshot']);
  if (!genericNames.has(base.toLowerCase())) {
    return sanitizeFilename(raw, `${fallbackBase}${ext}`);
  }
  return `${defaultExportBaseName(payload, fallbackBase)}${ext}`;
}

function legacyDownloadBlob(blob, filename) {
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = filename;
  document.body.appendChild(anchor);
  anchor.click();
  document.body.removeChild(anchor);
  setTimeout(() => URL.revokeObjectURL(url), 200);
}

function filePickerTypes({ description, accept, type, extension }) {
  if (accept && Object.keys(accept).length > 0) {
    return [{ description, accept }];
  }
  if (!type || !extension) {
    return [];
  }
  return [{ description, accept: { [type]: [extension] } }];
}

function pickerOptionsFor(filename, blobType, options = {}) {
  const suggestedName = sanitizeFilename(filename);
  const extension = options.extension || (() => {
    const dot = suggestedName.lastIndexOf('.');
    return dot >= 0 ? suggestedName.slice(dot) : '';
  })();
  return {
    suggestedName,
    types: filePickerTypes({
      description: options.description || 'Exported file',
      accept: options.accept,
      type: options.type || blobType || '',
      extension,
    }),
  };
}

export async function saveBlob(blob, filename, options = {}) {
  const safeName = sanitizeFilenameBase(filename, 'download');
  if (supportsSavePicker()) {
    let writable = null;
    try {
      const handle = await window.showSaveFilePicker(pickerOptionsFor(safeName, blob?.type, options));
      writable = await handle.createWritable();
      await writable.write(blob);
      await writable.close();
      writable = null;
      return true;
    } catch (error) {
      if (error?.name === 'AbortError') {
        return false;
      }
      if (!options.fallbackToDownloadOnError) {
        throw error;
      }
    } finally {
      if (writable && typeof writable.close === 'function') {
        try {
          await writable.close();
        } catch {
          // Ignore cleanup failures after the primary picker/write error.
        }
      }
    }
  }
  legacyDownloadBlob(blob, safeName);
  return true;
}

export async function saveTextFile(text, filename, type = 'text/plain', options = {}) {
  return saveBlob(new Blob([text], { type }), filename, {
    type,
    ...options,
  });
}

export async function downloadTextFile(text, filename, type = 'text/plain') {
  return saveTextFile(text, filename, type, { fallbackToDownloadOnError: true });
}

export async function saveURL(url, filename, options = {}) {
  const response = await fetch(url);
  if (!response.ok) {
    throw new Error(`save URL request failed: ${response.status}`);
  }
  const blob = await response.blob();
  return saveBlob(blob, filename, {
    fallbackToDownloadOnError: true,
    ...options,
  });
}

function bitmapExportScale(width, height) {
  if (!(width > 0) || !(height > 0)) {
    return DEFAULT_BITMAP_EXPORT_SCALE;
  }
  const safeScale = Math.sqrt(MAX_BITMAP_EXPORT_PIXELS / (width * height));
  return Math.max(1, Math.min(DEFAULT_BITMAP_EXPORT_SCALE, safeScale));
}

export function normalizeExportBackground(bgColor) {
  const color = String(bgColor || '').trim();
  if (!color || color.toLowerCase() === TRANSPARENT_EXPORT_BACKGROUND) {
    return null;
  }
  return color;
}

async function rasterizeSVGToCanvas(svgString, width, height, bgColor) {
  const scale = bitmapExportScale(width, height);
  const canvas = document.createElement('canvas');
  canvas.width = Math.max(1, Math.round(width * scale));
  canvas.height = Math.max(1, Math.round(height * scale));
  const ctx = canvas.getContext('2d');
  if (!ctx) {
    throw new Error('PNG export failed: 2D canvas context is unavailable.');
  }
  ctx.scale(scale, scale);
  const normalizedBackground = normalizeExportBackground(bgColor);
  if (normalizedBackground) {
    ctx.fillStyle = normalizedBackground;
    ctx.fillRect(0, 0, width, height);
  } else {
    ctx.clearRect(0, 0, width, height);
  }
  await new Promise((resolve, reject) => {
    const img = new Image();
    const svgBlob = new Blob([svgString], { type: 'image/svg+xml;charset=utf-8' });
    const url = URL.createObjectURL(svgBlob);
    img.onload = () => {
      ctx.drawImage(img, 0, 0);
      URL.revokeObjectURL(url);
      resolve();
    };
    img.onerror = () => {
      URL.revokeObjectURL(url);
      reject(new Error('PNG export failed: the SVG could not be rasterized.'));
    };
    img.src = url;
  });
  return canvas;
}

async function exportPNG({ svgString, width, height, bgColor, baseName }) {
  const canvas = await rasterizeSVGToCanvas(svgString, width, height, bgColor);
  const blob = await new Promise((resolve, reject) => {
    canvas.toBlob((value) => {
      if (value) {
        resolve(value);
        return;
      }
      reject(new Error('PNG export failed: the browser did not return a PNG blob.'));
    }, 'image/png');
  });
  return saveBlob(blob, `${baseName}.png`, {
    description: 'PNG image',
    accept: { 'image/png': ['.png'] },
    fallbackToDownloadOnError: true,
  });
}

async function exportSVG({ svgString, baseName }) {
  return saveBlob(
    new Blob([svgString], { type: 'image/svg+xml;charset=utf-8' }),
    `${baseName}.svg`,
    {
      description: 'SVG image',
      accept: { 'image/svg+xml': ['.svg'] },
      fallbackToDownloadOnError: true,
    },
  );
}

async function exportPDF({ svgString, width, height, baseName }) {
  const mmW = width * 0.264583;
  const mmH = height * 0.264583;
  const doc = new jsPDF({
    orientation: width >= height ? 'l' : 'p',
    unit: 'mm',
    format: [mmW, mmH],
    compress: true,
  });
  const host = document.createElement('div');
  host.style.position = 'fixed';
  host.style.left = '-100000px';
  host.style.top = '0';
  host.style.width = `${width}px`;
  host.style.height = `${height}px`;
  host.setAttribute('aria-hidden', 'true');
  const parsed = new DOMParser().parseFromString(svgString, 'image/svg+xml');
  const svg = parsed.documentElement;
  document.body.appendChild(host);
  host.appendChild(svg);
  try {
    await doc.svg(svg, { x: 0, y: 0, width: mmW, height: mmH });
  } finally {
    host.remove();
  }
  return saveBlob(doc.output('blob'), `${baseName}.pdf`, {
    description: 'PDF document',
    accept: { 'application/pdf': ['.pdf'] },
    fallbackToDownloadOnError: true,
  });
}

export async function exportTreeGraphic({ format, svgString, width, height, bgColor, baseName, payload }) {
  const genericNames = new Set(['', 'tree', 'phylotree', 'phylo-tree', 'phgo-tree']);
  const requested = String(baseName || '').trim().replace(/\.[^.]+$/u, '');
  const rawBaseName = genericNames.has(requested.toLowerCase()) ? defaultExportBaseName(payload, 'phylotree') : requested;
  const safeBaseName = sanitizeFilenameBase(rawBaseName, 'phylotree');
  if (format === 'svg') {
    return exportSVG({ svgString, baseName: safeBaseName });
  }
  if (format === 'png') {
    return exportPNG({ svgString, width, height, bgColor, baseName: safeBaseName });
  }
  if (format === 'pdf') {
    return exportPDF({ svgString, width, height, baseName: safeBaseName });
  }
  throw new Error(`Unsupported tree export format: ${format}`);
}

export function installPHGOSaveBridge(onError, defaultContext) {
  const handleError = (error) => {
    if (!error) {
      return;
    }
    if (error?.name === 'AbortError') {
      return;
    }
    onError?.(error instanceof Error ? error.message : String(error));
  };
  const saveBlobBridge = (blob, filename, options) => {
    const context = typeof defaultContext === 'function' ? defaultContext() : {};
    return saveBlob(blob, defaultedFilename(filename, context?.payload, context?.baseName || 'phgo-viewer'), options);
  };
  const saveURLBridge = (url, filename, options) => {
    const context = typeof defaultContext === 'function' ? defaultContext() : {};
    return saveURL(url, defaultedFilename(filename, context?.payload, context?.baseName || 'phgo-viewer'), options);
  };
  const exportTreeBridge = (request) => {
    const context = typeof defaultContext === 'function' ? defaultContext() : {};
    return exportTreeGraphic({
      ...(request || {}),
      payload: request?.payload || context?.payload,
      baseName: request?.baseName || context?.baseName,
    });
  };
  window.__PHGO_SAVE_BLOB__ = saveBlobBridge;
  window.__PHGO_SAVE_URL__ = saveURLBridge;
  window.__PHGO_EXPORT_TREE__ = exportTreeBridge;
  window.__PHGO_REPORT_EXPORT_ERROR__ = handleError;
  return () => {
    if (window.__PHGO_SAVE_BLOB__ === saveBlobBridge) {
      delete window.__PHGO_SAVE_BLOB__;
    }
    if (window.__PHGO_SAVE_URL__ === saveURLBridge) {
      delete window.__PHGO_SAVE_URL__;
    }
    if (window.__PHGO_EXPORT_TREE__ === exportTreeBridge) {
      delete window.__PHGO_EXPORT_TREE__;
    }
    if (window.__PHGO_REPORT_EXPORT_ERROR__ === handleError) {
      delete window.__PHGO_REPORT_EXPORT_ERROR__;
    }
  };
}

export function snapshotFilename(payload) {
  return `${defaultExportBaseName(payload, 'phgo-viewer')}.pgv`;
}
