import assert from 'node:assert/strict';
import {
  buildViewerSnapshot,
  normalizeExportBackground,
  parseViewerSnapshot,
  PGV_FORMAT,
  PGV_SCHEMA_VERSION,
  snapshotFilename,
  TRANSPARENT_EXPORT_BACKGROUND,
} from './pgv.js';

const payload = {
  schema_version: 1,
  session_id: 'canvas 2.3/test',
  updated_at: '2026-05-30T12:30:00Z',
  newick: '(PHGOT000001);',
};
const viewerState = {
  schema_version: 1,
  reactree: { layout: 'circular', zoom: 1.25, fontFamily: 'Georgia' },
  phgo: { payload_updated_at: payload.updated_at, split_percent: 42 },
};

const snapshot = buildViewerSnapshot(payload, viewerState);
assert.equal(snapshot.format, PGV_FORMAT);
assert.equal(snapshot.schema_version, PGV_SCHEMA_VERSION);
assert.equal(snapshot.producer, 'phytozome-go tree viewer');
assert.deepEqual(snapshot.payload, payload);
assert.deepEqual(snapshot.viewer_state, viewerState);
assert.match(snapshot.created_at, /^\d{4}-\d{2}-\d{2}T/);

assert.equal(snapshotFilename(payload), 'canvas_2.3_test.pgv');
assert.equal(snapshotFilename({ session_id: '  ' }), 'phgo-viewer.pgv');
assert.equal(normalizeExportBackground(''), null);
assert.equal(normalizeExportBackground('   '), null);
assert.equal(normalizeExportBackground(TRANSPARENT_EXPORT_BACKGROUND), null);
assert.equal(normalizeExportBackground('TrAnSpArEnT'), null);
assert.equal(normalizeExportBackground('#ffffff'), '#ffffff');

const parsed = parseViewerSnapshot(JSON.stringify(snapshot));
assert.deepEqual(parsed.payload, payload);
assert.deepEqual(parsed.viewer_state, viewerState);

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
