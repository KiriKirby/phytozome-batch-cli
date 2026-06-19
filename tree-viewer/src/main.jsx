import React, { Component, useEffect, useMemo, useRef, useState } from 'react';
import { createRoot } from 'react-dom/client';
import {
  FluentProvider,
  webLightTheme,
} from '@fluentui/react-components';
import { Reactree } from 'reactreejs';
import 'reactreejs/style.css';
import './style.css';
import { relabelFasta, relabelNewick } from './labels.js';
import { megaDefaultDisplayNewick } from './mega-display.js';
import {
  buildViewerSnapshot,
  installPHGOSaveBridge,
  PGV_VIEWER_STATE_SCHEMA_VERSION,
  saveTextFile,
  snapshotFilename,
} from './pgv.js';

const VIEWER_STATE_SCHEMA_VERSION = PGV_VIEWER_STATE_SCHEMA_VERSION;

class ViewerErrorBoundary extends Component {
  constructor(props) {
    super(props);
    this.state = { message: '' };
  }

  static getDerivedStateFromError(error) {
    return { message: error instanceof Error ? error.message : String(error) };
  }

  componentDidCatch(error, info) {
    const message = error instanceof Error ? error.message : String(error);
    window.__phgoViewerLastRenderError = { message, stack: error?.stack || '', componentStack: info?.componentStack || '' };
    console.error('[phgo-viewer] render failed', error, info);
  }

  render() {
    if (!this.state.message) {
      return this.props.children;
    }
    return (
      <FluentProvider theme={webLightTheme} className="phgo-viewer-provider">
        <main className="shell">
          <section className="phgo-warning-toast" role="alert" aria-live="assertive">
            <p className="phgo-warning-toast-message">Tree viewer render failed: {this.state.message}</p>
            <button type="button" className="phgo-warning-toast-close" onClick={() => this.setState({ message: '' })}>
              Close
            </button>
          </section>
        </main>
      </FluentProvider>
    );
  }
}

function sessionIDFromPath() {
  const parts = window.location.pathname.split('/').filter(Boolean);
  const index = parts.indexOf('sessions');
  if (index >= 0 && parts[index + 1]) {
    return decodeURIComponent(parts[index + 1]);
  }
  return 'canvas';
}

async function fetchPayload(sessionID) {
  const response = await fetch(`/sessions/${encodeURIComponent(sessionID)}/payload`, { cache: 'no-store' });
  if (!response.ok) throw new Error(`payload request failed: ${response.status}`);
  return response.json();
}

async function fetchViewerState(sessionID) {
  const response = await fetch(`/sessions/${encodeURIComponent(sessionID)}/state`, { cache: 'no-store' });
  if (!response.ok) throw new Error(`viewer state request failed: ${response.status}`);
  return response.json();
}

async function fetchSessionStatus(sessionID) {
  const response = await fetch(`/sessions/${encodeURIComponent(sessionID)}/status`, { cache: 'no-store' });
  if (!response.ok) throw new Error(`viewer status request failed: ${response.status}`);
  return response.json();
}

async function putViewerState(sessionID, state) {
  const response = await fetch(`/sessions/${encodeURIComponent(sessionID)}/state`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(state || {}),
  });
  if (!response.ok) throw new Error(`viewer state update failed: ${response.status}`);
}

function liveInitialViewerState(state, sessionID) {
  if (!state || typeof state !== 'object') {
    return null;
  }
  if (String(sessionID || '').startsWith('nwk-browser-')) {
    return state;
  }
  const next = { ...state };
  if (next.reactree && typeof next.reactree === 'object') {
    const reactree = { ...next.reactree };
    delete reactree.treeData;
    delete reactree.history;
    delete reactree.currentNewick;
    delete reactree.collapsedNodes;
    delete reactree.collapseLabels;
    delete reactree.cladeEditor;
    delete reactree.nodeInfo;
    reactree.rerootMode = false;
    reactree.flipMode = false;
    reactree.swapMode = false;
    next.reactree = reactree;
  }
  return Object.keys(next).length > 0 ? next : null;
}

function App() {
  const sessionID = useMemo(sessionIDFromPath, []);
  const [payload, setPayload] = useState(null);
  const [initialViewerState, setInitialViewerState] = useState(null);
  const viewerStateRef = useRef({});
  const [sessionStatus, setSessionStatus] = useState({});
  const [error, setError] = useState('');
  const [splitPercent, setSplitPercent] = useState(42);
  const splitPercentRef = useRef(splitPercent);
  const viewerStageRef = useRef(null);
  const [viewerHeight, setViewerHeight] = useState(520);
  const viewerHeightRef = useRef(520);
  const loadedPayloadStampRef = useRef('');
  const hasLoadedPayloadRef = useRef(false);
  const hasTree = Boolean(payload?.newick?.trim());
  const newick = useMemo(() => relabelNewick(megaDefaultDisplayNewick(payload?.newick, payload?.metadata), payload?.metadata), [payload]);
  const fasta = useMemo(() => relabelFasta(payload?.aligned_fasta, payload?.metadata), [payload]);
  const viewerTitle = useMemo(() => {
    const rawTitle = String(payload?.title || payload?.metadata?.title || sessionID || '').trim();
    return rawTitle ? `Phgotreer: ${rawTitle}` : 'Phgotreer';
  }, [payload, sessionID]);
  const preventTextSelection = (event) => {
    event.preventDefault();
  };

  async function reload() {
    try {
      setError('');
      const [nextPayload, nextState, nextStatus] = await Promise.all([
        fetchPayload(sessionID),
        fetchViewerState(sessionID).catch(() => ({})),
        fetchSessionStatus(sessionID).catch(() => ({})),
      ]);
      const statePayloadStamp = nextState?.phgo?.payload_updated_at;
      const payloadStamp = nextPayload?.updated_at || '';
      const stateMatchesPayload = !statePayloadStamp || statePayloadStamp === payloadStamp;
      const usableState = stateMatchesPayload ? liveInitialViewerState(nextState, sessionID) : {};
      const payloadChanged = !hasLoadedPayloadRef.current || loadedPayloadStampRef.current !== payloadStamp;
      setPayload(nextPayload);
      setSessionStatus(nextStatus || {});
      loadedPayloadStampRef.current = payloadStamp;
      hasLoadedPayloadRef.current = true;
      if (payloadChanged) {
        setInitialViewerState(usableState && Object.keys(usableState).length > 0 ? usableState : null);
        viewerStateRef.current = usableState || {};
        if (typeof usableState?.phgo?.split_percent === 'number') {
          setSplitPercent(usableState.phgo.split_percent);
        }
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  function handleViewerStateChange(state) {
    viewerStateRef.current = {
      schema_version: VIEWER_STATE_SCHEMA_VERSION,
      reactree: state || {},
      phgo: {
        split_percent: splitPercentRef.current,
        payload_updated_at: payload?.updated_at || '',
        viewport: {
          inner_width: window.innerWidth,
          inner_height: window.innerHeight,
          device_pixel_ratio: window.devicePixelRatio || 1,
        },
      },
    };
    window.clearTimeout(handleViewerStateChange.timer);
    handleViewerStateChange.timer = window.setTimeout(() => {
      putViewerState(sessionID, viewerStateRef.current).catch((err) => {
        setError(err instanceof Error ? err.message : String(err));
      });
    }, 250);
  }

  async function handleViewerSnapshot(state) {
    try {
      const viewerState = {
        schema_version: VIEWER_STATE_SCHEMA_VERSION,
        reactree: state || viewerStateRef.current?.reactree || {},
        phgo: {
          ...(viewerStateRef.current?.phgo || {}),
          split_percent: splitPercentRef.current,
          payload_updated_at: payload?.updated_at || '',
          viewport: {
            inner_width: window.innerWidth,
            inner_height: window.innerHeight,
            device_pixel_ratio: window.devicePixelRatio || 1,
          },
        },
      };
      const snapshot = buildViewerSnapshot(payload, viewerState);
      await saveTextFile(
        `${JSON.stringify(snapshot, null, 2)}\n`,
        snapshotFilename(payload),
        'application/json',
        {
          description: 'PHgo Viewer Snapshot',
          accept: {
            'application/json': ['.pgv'],
          },
          extension: '.pgv',
          fallbackToDownloadOnError: true,
        },
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  useEffect(() => {
    reload();
    const events = new EventSource(`/events/${encodeURIComponent(sessionID)}`);
    events.addEventListener('update', reload);
    events.onerror = () => setError('Live update stream disconnected; refresh the page if updates stop.');
    return () => events.close();
  }, [sessionID]);

  useEffect(() => installPHGOSaveBridge((message) => setError(message)), []);

  useEffect(() => {
    if (!hasTree) return undefined;
    const stopDoubleClickSearch = (event) => {
      const target = event.target;
      if (target?.closest?.('.Reactree_searchPanel, .Reactree_searchBtn, .phgo-office-search')) {
        return;
      }
      if (target?.closest?.('.viewer-stage .Reactree_container, .viewer-stage svg, .Reactree_taxaList, .Reactree_taxaListItem, .Reactree_leftPanel, .Reactree_sidebar')) {
        event.preventDefault();
        event.stopImmediatePropagation?.();
        event.stopPropagation();
      }
    };
    document.addEventListener('dblclick', stopDoubleClickSearch, true);
    return () => document.removeEventListener('dblclick', stopDoubleClickSearch, true);
  }, [hasTree]);

  useEffect(() => {
    document.title = viewerTitle;
  }, [viewerTitle]);

  useEffect(() => {
    const stage = viewerStageRef.current;
    if (!stage) return undefined;

    const updateHeight = () => {
      const nextHeight = Math.max(360, Math.floor(stage.getBoundingClientRect().height || 0));
      if (nextHeight === viewerHeightRef.current) {
        return;
      }
      viewerHeightRef.current = nextHeight;
      setViewerHeight(nextHeight);
    };

    updateHeight();
    const observer = new ResizeObserver(() => updateHeight());
    observer.observe(stage);
    return () => {
      observer.disconnect();
    };
  }, [hasTree]);

  useEffect(() => {
    splitPercentRef.current = splitPercent;
    document
      .querySelectorAll('.viewer-stage .Reactree_bodyRowWithAln')
      .forEach((body) => body.style.setProperty('--phgo-aln-width', `${splitPercent}%`));
    if (viewerStateRef.current?.reactree) {
      viewerStateRef.current = {
        ...viewerStateRef.current,
        phgo: {
          ...(viewerStateRef.current.phgo || {}),
          split_percent: splitPercent,
          payload_updated_at: payload?.updated_at || '',
        },
      };
      window.clearTimeout(handleViewerStateChange.timer);
      handleViewerStateChange.timer = window.setTimeout(() => {
        putViewerState(sessionID, viewerStateRef.current).catch((err) => {
          setError(err instanceof Error ? err.message : String(err));
        });
      }, 250);
    }
  }, [splitPercent]);

  useEffect(() => {
    if (!hasTree) return undefined;

    let dragging = false;
    let activeBody = null;

    const clampSplit = (value) => Math.min(70, Math.max(22, value));

    const applySplit = (clientX) => {
      if (!activeBody) return;
      const rect = activeBody.getBoundingClientRect();
      if (rect.width <= 0) return;
      const alignmentWidth = ((rect.right - clientX) / rect.width) * 100;
      setSplitPercent(clampSplit(alignmentWidth));
    };

    const onPointerMove = (event) => {
      if (!dragging) return;
      event.preventDefault();
      applySplit(event.clientX);
    };

    const onPointerUp = () => {
      dragging = false;
      activeBody = null;
      document.body.classList.remove('phgo-resizing-alignment');
      window.removeEventListener('pointermove', onPointerMove);
      window.removeEventListener('pointerup', onPointerUp);
      window.dispatchEvent(new Event('phgo-alignment-resize'));
    };

    const attachSplitter = () => {
      const body = document.querySelector('.viewer-stage .Reactree_bodyRowWithAln');
      if (!body) return;
      body.style.setProperty('--phgo-aln-width', `${splitPercentRef.current}%`);
      const container = body.querySelector('.Reactree_container');
      const alignment = body.querySelector('.Reactree_alnPanel');
      if (!container || !alignment || body.querySelector('.phgo-aln-splitter')) return;

      const splitter = document.createElement('div');
      splitter.className = 'phgo-aln-splitter';
      splitter.setAttribute('role', 'separator');
      splitter.setAttribute('aria-orientation', 'vertical');
      splitter.setAttribute('title', 'Drag to resize tree and alignment panels');
      splitter.addEventListener('pointerdown', (event) => {
        dragging = true;
        activeBody = body;
        splitter.setPointerCapture?.(event.pointerId);
        document.body.classList.add('phgo-resizing-alignment');
        applySplit(event.clientX);
        window.addEventListener('pointermove', onPointerMove);
        window.addEventListener('pointerup', onPointerUp);
      });
      body.insertBefore(splitter, alignment);
    };

    attachSplitter();
    const observer = new MutationObserver(attachSplitter);
    observer.observe(viewerStageRef.current ?? document.getElementById('root'), { childList: true, subtree: true });
    return () => {
      observer.disconnect();
      onPointerUp();
    };
  }, [hasTree]);

  return (
    <FluentProvider theme={webLightTheme} className="phgo-viewer-provider">
      <main className="shell" onSelectStart={preventTextSelection} onDragStart={preventTextSelection}>
        {error && (
          <section className="phgo-warning-toast" role="status" aria-live="polite">
            <p className="phgo-warning-toast-message">{error}</p>
            <button type="button" className="phgo-warning-toast-close" onClick={() => setError('')}>
              Close
            </button>
          </section>
        )}
        {sessionStatus?.refreshing && (
          <section className="phgo-warning-toast phgo-status-toast" role="status" aria-live="polite">
            <p className="phgo-warning-toast-message">{sessionStatus.message || 'Refreshing tree and MSA...'}</p>
          </section>
        )}
        {hasTree && (
          <div className="viewer-stage" ref={viewerStageRef}>
            <Reactree
              newick={newick}
              fasta={fasta}
              defaultHeight={viewerHeight}
              initialState={initialViewerState?.reactree}
              onStateChange={handleViewerStateChange}
              onViewerSnapshot={handleViewerSnapshot}
            />
          </div>
        )}
      </main>
    </FluentProvider>
  );
}

createRoot(document.getElementById('root')).render(
  <ViewerErrorBoundary>
    <App />
  </ViewerErrorBoundary>,
);
