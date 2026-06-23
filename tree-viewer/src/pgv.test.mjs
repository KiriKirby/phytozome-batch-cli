import assert from 'node:assert/strict';
import {
  buildViewerSnapshot,
  defaultExportBaseName,
  normalizeExportBackground,
  parseViewerSnapshot,
  PGV_FORMAT,
  PGV_REACTREE_STATE_SCHEMA_VERSION,
  PGV_SCHEMA_VERSION,
  PGV_VIEWER_STATE_SCHEMA_VERSION,
  snapshotFilename,
  titleNumberFilenameBase,
  TRANSPARENT_EXPORT_BACKGROUND,
} from './pgv.js';

const payload = {
  schema_version: 1,
  session_id: 'canvas 2.3/test',
  title: '1.1 Brassica test tree',
  updated_at: '2026-05-30T12:30:00Z',
  newick: '(PHGOT000001);',
};
const viewerState = {
  schema_version: 1,
  reactree: {
    schema_version: PGV_REACTREE_STATE_SCHEMA_VERSION,
    layout: 'circular',
    renderStyle: 'mega',
    hScale: 0,
    vScale: 0,
    fontFamily: 'Georgia',
    exportLongEdge: 8192,
    activeRibbonTab: 'format',
    searchOpen: true,
    searchQuery: 'PAL',
    transform: { x: 10, y: 20, k: 1.25 },
  },
  phgo: {
    payload_updated_at: payload.updated_at,
    split_percent: 42,
    viewport: { inner_width: 1200, inner_height: 800, device_pixel_ratio: 1 },
  },
};

const snapshot = buildViewerSnapshot(payload, viewerState);
assert.equal(snapshot.format, PGV_FORMAT);
assert.equal(snapshot.schema_version, PGV_SCHEMA_VERSION);
assert.equal(snapshot.viewer_state.schema_version, PGV_VIEWER_STATE_SCHEMA_VERSION);
assert.equal(snapshot.producer, 'phytozome-go tree viewer');
assert.deepEqual(snapshot.payload, payload);
assert.deepEqual(snapshot.viewer_state, {
  ...viewerState,
  schema_version: PGV_VIEWER_STATE_SCHEMA_VERSION,
});
assert.match(snapshot.created_at, /^\d{4}-\d{2}-\d{2}T/);

assert.equal(titleNumberFilenameBase('Phgotreer: 1.1 Brassica test tree'), '1.1');
assert.equal(titleNumberFilenameBase('Phgomsar: 1 MSA export'), '1');
assert.equal(titleNumberFilenameBase('Phgotreer: 1/2 invalid chars'), '1');
assert.equal(titleNumberFilenameBase('No numeric title', 'fallback'), 'fallback');
assert.equal(defaultExportBaseName(payload), '1.1');
assert.equal(defaultExportBaseName({ title: '', session_id: 'bad/session:name' }), 'bad_session_name');
assert.equal(snapshotFilename(payload), '1.1.pgv');
assert.equal(snapshotFilename({ session_id: '  ' }), 'phgo-viewer.pgv');
assert.equal(normalizeExportBackground(''), null);
assert.equal(normalizeExportBackground('   '), null);
assert.equal(normalizeExportBackground(TRANSPARENT_EXPORT_BACKGROUND), null);
assert.equal(normalizeExportBackground('TrAnSpArEnT'), null);
assert.equal(normalizeExportBackground('#ffffff'), '#ffffff');

const parsed = parseViewerSnapshot(JSON.stringify(snapshot));
assert.deepEqual(parsed.payload, payload);
assert.equal(parsed.viewer_state.reactree.hScale, 0);
assert.equal(parsed.viewer_state.reactree.vScale, 0);
assert.equal(parsed.viewer_state.reactree.exportLongEdge, 8192);
assert.equal(parsed.viewer_state.reactree.renderStyle, 'mega');
assert.equal(parsed.viewer_state.phgo.split_percent, 42);

const parsedV1 = parseViewerSnapshot(JSON.stringify({ ...snapshot, schema_version: 1 }));
assert.equal(parsedV1.schema_version, 1);

assert.throws(
  () => parseViewerSnapshot('{"format":"other","schema_version":1}'),
  /format is not recognized/,
);

assert.throws(
  () => parseViewerSnapshot(JSON.stringify({ format: PGV_FORMAT, schema_version: 999, payload: {} })),
  /Unsupported PGV schema version/,
);
