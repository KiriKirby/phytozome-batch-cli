import { jsPDF } from 'jspdf';
import 'svg2pdf.js';

async function exportPDFBlob({ svgString, width, height }) {
  const logicalWidth = Math.max(1, Number(width) || 1);
  const logicalHeight = Math.max(1, Number(height) || 1);
  const mmWidth = logicalWidth * 0.264583;
  const mmHeight = logicalHeight * 0.264583;
  const doc = new jsPDF({
    orientation: logicalWidth >= logicalHeight ? 'l' : 'p',
    unit: 'mm',
    format: [mmWidth, mmHeight],
    compress: true,
  });
  const host = document.createElement('div');
  host.style.position = 'fixed';
  host.style.left = '-100000px';
  host.style.top = '0';
  host.style.width = `${logicalWidth}px`;
  host.style.height = `${logicalHeight}px`;
  host.setAttribute('aria-hidden', 'true');
  const parsed = new DOMParser().parseFromString(String(svgString || ''), 'image/svg+xml');
  const parseError = parsed.querySelector('parsererror');
  if (parseError) {
    throw new Error('PDF export failed: generated SVG is not valid XML.');
  }
  const svg = parsed.documentElement;
  if (!svg || String(svg.nodeName || '').toLowerCase() !== 'svg') {
    throw new Error('PDF export failed: generated document is not an SVG.');
  }
  document.body.appendChild(host);
  host.appendChild(svg);
  try {
    await doc.svg(svg, { x: 0, y: 0, width: mmWidth, height: mmHeight });
  } finally {
    host.remove();
  }
  return doc.output('blob');
}

window.PHGOmsaexporPDF = { exportPDFBlob };
