(function() {
  "use strict";

  function formatValue(value) {
    if (value instanceof Error) return value.message;
    if (typeof value === "string") return value;
    if (value && typeof value === "object") {
      const fields = {};
      for (const key of ["message", "name", "stack", "__CLASS_NAME__", "clazzName"]) {
        if (value[key]) fields[key] = String(value[key]);
      }
      try {
        const className = value.getClass$ && value.getClass$().__CLASS_NAME__;
        if (className) fields.className = String(className);
      } catch (_error) {
        // SwingJS exception objects are not always normal JavaScript Errors.
      }
      for (const method of ["getMessage$", "getLocalizedMessage$", "toString$"]) {
        try {
          if (typeof value[method] === "function") {
            const text = value[method]();
            if (text) fields[method] = String(text);
          }
        } catch (_error) {
          // Keep formatting best-effort for bootstrap diagnostics.
        }
      }
      for (const method of ["getName$", "getCanonicalName$", "getSimpleName$"]) {
        try {
          if (typeof value[method] === "function") {
            const text = value[method]();
            if (text) fields[method] = String(text);
          }
        } catch (_error) {
          // Keep formatting best-effort for bootstrap diagnostics.
        }
      }
      if (value.__CLASS_NAME__) fields.rawClassName = String(value.__CLASS_NAME__);
      try {
        const text = value.toString && value.toString();
        if (text && text !== "[object Object]") fields.text = String(text);
      } catch (_error) {
        // Keep formatting best-effort for bootstrap diagnostics.
      }
      if (Object.keys(fields).length > 0) return JSON.stringify(fields);
    }
    try {
      return JSON.stringify(value);
    } catch (_error) {
      return String(value);
    }
  }

  function decodeBase64URL(value) {
    const base64 = String(value || "").replace(/-/g, "+").replace(/_/g, "/");
    const padded = base64 + "=".repeat((4 - (base64.length % 4)) % 4);
    const binary = atob(padded);
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i += 1) {
      bytes[i] = binary.charCodeAt(i);
    }
    return new TextDecoder().decode(bytes);
  }

  function parseStateFromHash() {
    if (window.__PHGOJalviewInitialState && typeof window.__PHGOJalviewInitialState === "object") {
      return window.__PHGOJalviewInitialState;
    }
    const hash = String(window.location.hash || "").replace(/^#/, "");
    if (hash.startsWith("phgo=")) {
      return JSON.parse(decodeBase64URL(hash.slice("phgo=".length)));
    }
    const params = new URLSearchParams(hash);
    return {
      open: params.get("open") || "",
      title: params.get("title") || ""
    };
  }

  function notify(type, extra) {
    if (window.parent === window) return;
    const payload = Object.assign({ source: "phgo-jalviewjs", type }, extra || {});
    window.parent.postMessage(payload, "*");
  }

  function debug(type, extra) {
    const payload = Object.assign({ type }, extra || {});
    window.__phgoJalviewDebug = window.__phgoJalviewDebug || [];
    window.__phgoJalviewDebug.push(payload);
    if (window.__phgoJalviewDebug.length > 200) window.__phgoJalviewDebug.shift();
    if (window.console && typeof window.console.debug === "function") {
      window.console.debug("[phgo-jalviewjs]", type, payload);
    }
  }

  function toBootstrapRelativePath(value) {
    const target = String(value || "").trim();
    if (!target) return "";
    if (/^https?:\/\//i.test(target)) return target;
    if (target.startsWith("/")) return `${window.location.origin}${target}`;
    return target;
  }

  function elementState(id) {
    const node = document.getElementById(id);
    if (!node) return null;
    const rect = node.getBoundingClientRect();
    return {
      tag: node.tagName,
      childCount: node.childElementCount,
      htmlLength: node.innerHTML.length,
      width: rect.width,
      height: rect.height
    };
  }

  function collectState() {
    const ids = [
      "jalview-desktop-div",
      "jalview-alignment-div",
      "jalview-structureviewer-div",
      "jalview-tree-div",
      "jalview-pca-div",
      "sysoutdiv"
    ];
    const elements = {};
    for (const id of ids) {
      elements[id] = elementState(id);
    }
    return {
      title: document.title,
      bodyChildCount: document.body ? document.body.childElementCount : 0,
      ids: elements,
      jalviewGlobals: {
        SwingJS: !!window.SwingJS,
        JalviewJSEmbedded: !!window.JalviewJSEmbedded,
        JalviewJS: !!window.JalviewJS
      }
    };
  }

  function viewportSize() {
    return {
      width: Math.max(320, Math.floor(window.innerWidth || document.documentElement.clientWidth || 0)),
      height: Math.max(240, Math.floor(window.innerHeight || document.documentElement.clientHeight || 0))
    };
  }

  function normalizeSequenceKind(value) {
    const kind = String(value || "").trim().toLowerCase();
    if (kind === "protein" || kind === "peptide" || kind === "aa" || kind === "aminoacid" || kind === "amino_acid") {
      return "protein";
    }
    if (kind === "dna" || kind === "rna" || kind === "na" || kind === "nucleotide" || kind === "nucleic") {
      return "nucleotide";
    }
    return "";
  }

  function installPHgoState(state) {
    const conversionTarget = normalizeSequenceKind(state.conversionTarget);
    const sequenceKind = conversionTarget || normalizeSequenceKind(state.sequenceKind);
    window.__PHGOJalviewState = {
      session: String(state.session || ""),
      open: String(state.open || ""),
      title: String(state.title || ""),
      sequenceKind,
      conversionTarget,
      alignmentMethod: String(state.alignmentMethod || ""),
      treeMethod: String(state.treeMethod || ""),
      payloadUpdatedAt: String(state.payloadUpdatedAt || "")
    };
    return window.__PHGOJalviewState;
  }

  function callNoArg(target, names) {
    if (!target) return;
    for (const name of names) {
      if (typeof target[name] === "function") {
        try {
          target[name]();
        } catch (error) {
          debug("layout-call-failed", { method: name, message: formatValue(error) });
        }
      }
    }
  }

  function applyMainFrameMode(frame) {
    const phgo = window.__PHGOJalviewState || {};
    const kind = normalizeSequenceKind(phgo.conversionTarget || phgo.sequenceKind);
    if (!frame || !kind) return;
    try {
      const viewport = typeof frame.getViewport$ === "function" ? frame.getViewport$() : frame.viewport;
      const alignment = viewport && typeof viewport.getAlignment$ === "function" ? viewport.getAlignment$() : viewport && viewport.alignment;
      const nucleotide = kind === "nucleotide";
      if (alignment) {
        alignment.nucleotide = nucleotide;
        if (typeof alignment.getDataset$ === "function") {
          const dataset = alignment.getDataset$();
          if (dataset) dataset.nucleotide = nucleotide;
        }
      }
      frame.__phgoSequenceKind = kind;
      callNoArg(frame, ["setGUINucleotide$", "buildColourMenu$", "setMenusForViewport$", "doLayout$", "validate$", "revalidate$", "repaint$"]);
      if (frame.alignPanel) {
        callNoArg(frame.alignPanel, ["doLayout$", "validate$", "revalidate$", "repaint$"]);
        if (typeof frame.alignPanel.paintAlignment$Z$Z === "function") {
          frame.alignPanel.paintAlignment$Z$Z(true, true);
        }
      }
    } catch (error) {
      debug("apply-mode-failed", { kind, message: formatValue(error) });
    }
  }

  function mainAlignmentFrame(desktop) {
    if (window.__PHGOMainAlignmentFrame) return window.__PHGOMainAlignmentFrame;
    if (desktop && desktop.phgoMainAlignmentFrame) return desktop.phgoMainAlignmentFrame;
    if (desktop && desktop.instance && desktop.instance.phgoMainAlignmentFrame) return desktop.instance.phgoMainAlignmentFrame;
    if (desktop && typeof desktop.getAlignFrames$ === "function") {
      try {
        const frames = desktop.getAlignFrames$() || [];
        for (let i = 0; i < frames.length; i += 1) {
          if (frames[i] && frames[i].__phgoMainAlignmentFrame) return frames[i];
        }
      } catch (error) {
        debug("main-frame-lookup-failed", { message: formatValue(error) });
      }
    }
    return null;
  }

  function sequenceName(seq) {
    if (!seq) return "";
    try {
      if (typeof seq.getName$ === "function") return String(seq.getName$() || "");
    } catch (_error) {
      return "";
    }
    return String(seq.name || seq.id || "");
  }

  function normalizeSelectionState(value) {
    const state = String(value || "").trim().toLowerCase();
    if (state === "yellow" || state === "red") return state;
    return "green";
  }

  function nextSelectionState(value) {
    switch (normalizeSelectionState(value)) {
      case "green":
        return "yellow";
      default:
        return "green";
    }
  }

  function ensureToast() {
    let toast = document.querySelector(".phgo-warning-toast");
    if (toast) return toast;
    toast = document.createElement("section");
    toast.className = "phgo-warning-toast";
    toast.setAttribute("role", "status");
    toast.setAttribute("aria-live", "polite");
    const message = document.createElement("p");
    message.className = "phgo-warning-toast-message";
    toast.appendChild(message);
    document.body.appendChild(toast);
    return toast;
  }

  function showToast(message, persistent) {
    const toast = ensureToast();
    const text = toast.querySelector(".phgo-warning-toast-message");
    text.textContent = String(message || "");
    toast.hidden = !text.textContent;
    if (!persistent && text.textContent) {
      window.clearTimeout(showToast.timer);
      showToast.timer = window.setTimeout(() => {
        toast.hidden = true;
      }, 2400);
    }
  }

  async function fetchJSON(path, options) {
    const response = await fetch(path, Object.assign({ cache: "no-store" }, options || {}));
    if (!response.ok) throw new Error(`${path} failed: ${response.status}`);
    return response.json();
  }

  async function putJSON(path, value, options) {
    const response = await fetch(path, Object.assign({
      method: "PUT",
      cache: "no-store",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(value)
    }, options || {}));
    if (!response.ok) throw new Error(`${path} failed: ${response.status}`);
  }

  async function currentPayloadUpdatedAt(session) {
    const payload = await fetchJSON(`/sessions/${encodeURIComponent(session)}/payload`);
    return String(payload && payload.updated_at || "");
  }

  function currentSession() {
    return String((window.__PHGOJalviewState || {}).session || "").trim();
  }

  async function loadMSASelection() {
    const session = currentSession();
    if (!session) return;
    const data = await fetchJSON(`/sessions/${encodeURIComponent(session)}/msa/selection`);
    const rows = Array.isArray(data.rows) ? data.rows : [];
    const selection = new Map();
    const byName = new Map();
    const byIndex = new Map();
    for (const row of rows) {
      const taxonID = String(row.taxon_id || "").trim();
      const displayName = String(row.display_name || "").trim();
      const displayPrefix = String(row.display_prefix || "").trim();
      if (!taxonID) continue;
      const index = Number.isFinite(row.index) ? row.index : Number.parseInt(row.index, 10);
      const entry = {
        taxonID,
        displayName,
        displayPrefix,
        displayLabel: String(row.display_label || "").trim(),
        canvasItemIndex: Number.parseInt(row.canvas_item_index, 10),
        canvasRow: Number.parseInt(row.canvas_row, 10),
        index: Number.isFinite(index) ? index : -1,
        state: normalizeSelectionState(row.state)
      };
      selection.set(taxonID, entry);
      if (displayName) byName.set(displayName, entry);
      if (entry.index >= 0) byIndex.set(entry.index, entry);
    }
    window.__PHGOMSASelection = selection;
    window.__PHGOMSASelectionByName = byName;
    window.__PHGOMSASelectionByIndex = byIndex;
  }

  function javaListToArray(value, limit) {
    const max = Math.max(0, limit || 250);
    if (!value || max === 0) return [];
    if (Array.isArray(value)) return value.slice(0, max);
    const out = [];
    try {
      if (typeof value.size$ === "function" && typeof value.get$I === "function") {
        const size = Math.min(max, Number(value.size$()) || 0);
        for (let i = 0; i < size; i += 1) out.push(value.get$I(i));
        return out;
      }
    } catch (error) {
      debug("java-list-size-failed", { message: formatValue(error) });
    }
    try {
      if (typeof value.iterator$ === "function") {
        const iterator = value.iterator$();
        while (iterator && typeof iterator.hasNext$ === "function" && iterator.hasNext$() && out.length < max) {
          out.push(iterator.next$());
        }
        return out;
      }
    } catch (error) {
      debug("java-list-iterator-failed", { message: formatValue(error) });
    }
    return out;
  }

  function callValue(target, names) {
    if (!target) return undefined;
    for (const name of names) {
      try {
        if (typeof target[name] === "function") return target[name]();
      } catch (_error) {
        // Jalview/SwingJS getters may throw while frames are still initializing.
      }
    }
    return undefined;
  }

  function primitiveValue(value) {
    if (value == null) return undefined;
    if (typeof value === "string" || typeof value === "boolean") return value;
    if (typeof value === "number") return Number.isFinite(value) ? value : undefined;
    try {
      if (typeof value.booleanValue$ === "function") return !!value.booleanValue$();
      if (typeof value.intValue$ === "function") return Number(value.intValue$());
      if (typeof value.floatValue$ === "function") return Number(value.floatValue$());
      if (typeof value.doubleValue$ === "function") return Number(value.doubleValue$());
    } catch (_error) {
      return undefined;
    }
    return undefined;
  }

  function stringValue(value) {
    if (value == null) return "";
    const primitive = primitiveValue(value);
    if (primitive != null) return String(primitive);
    try {
      if (typeof value.toString$ === "function") return String(value.toString$() || "");
      if (typeof value.toString === "function") return String(value.toString() || "");
    } catch (_error) {
      return "";
    }
    return "";
  }

  function javaMapToObject(value, limit) {
    const max = Math.max(0, limit || 250);
    if (!value || max === 0) return {};
    const out = {};
    try {
      if (typeof value.entrySet$ === "function") {
        const entries = value.entrySet$();
        const iterator = entries && typeof entries.iterator$ === "function" ? entries.iterator$() : null;
        let count = 0;
        while (iterator && typeof iterator.hasNext$ === "function" && iterator.hasNext$() && count < max) {
          const entry = iterator.next$();
          const key = stringValue(callValue(entry, ["getKey$"]));
          const rawValue = callValue(entry, ["getValue$"]);
          if (key) {
            if (rawValue && typeof rawValue.entrySet$ === "function") {
              out[key] = javaMapToObject(rawValue, max);
            } else {
              const primitive = primitiveValue(rawValue);
              out[key] = primitive !== undefined ? primitive : stringValue(rawValue);
            }
          }
          count += 1;
        }
      }
    } catch (error) {
      debug("java-map-collect-failed", { message: formatValue(error) });
    }
    return out;
  }

  function setIfPresent(target, key, value) {
    const primitive = primitiveValue(value);
    if (primitive !== undefined) {
      target[key] = primitive;
      return;
    }
    const text = stringValue(value).trim();
    if (text && text !== "[object Object]") target[key] = text;
  }

  function mainViewport(frame) {
    if (!frame) return null;
    try {
      if (typeof frame.getViewport$ === "function") return frame.getViewport$();
    } catch (_error) {
      return null;
    }
    return frame.viewport || frame.av || null;
  }

  function mainAlignment(frame) {
    const viewport = mainViewport(frame);
    if (!viewport) return null;
    try {
      if (typeof viewport.getAlignment$ === "function") return viewport.getAlignment$();
    } catch (_error) {
      return null;
    }
    return viewport.alignment || null;
  }

  function collectSelectionRows() {
    const selection = window.__PHGOMSASelection || new Map();
    return [...selection.values()].map((entry) => ({
      taxon_id: entry.taxonID,
      display_name: entry.displayName || "",
      display_prefix: entry.displayPrefix || "",
      display_label: entry.displayLabel || "",
      canvas_item_index: Number.isFinite(entry.canvasItemIndex) ? entry.canvasItemIndex : undefined,
      canvas_row: Number.isFinite(entry.canvasRow) ? entry.canvasRow : undefined,
      index: entry.index,
      state: normalizeSelectionState(entry.state)
    }));
  }

  function alignmentSequences(frame) {
    const alignment = mainAlignment(frame);
    if (!alignment) return [];
    const raw = callValue(alignment, ["getSequences$"]);
    const fromList = javaListToArray(raw, 5000);
    if (fromList.length > 0) return fromList;
    const height = Number(primitiveValue(callValue(alignment, ["getHeight$"]))) || 0;
    const out = [];
    for (let i = 0; i < height; i += 1) {
      try {
        if (typeof alignment.getSequenceAt$I === "function") {
          const seq = alignment.getSequenceAt$I(i);
          if (seq) out.push(seq);
        }
      } catch (_error) {
        // Keep sequence collection best-effort; Jalview may be mid-refresh.
      }
    }
    return out;
  }

  function sequenceTaxonID(seq) {
    if (!seq) return "";
    return String(seq.phgoTaxonID || "").trim();
  }

  function sequenceDescription(seq) {
    return stringValue(callValue(seq, ["getDescription$"]));
  }

  function sequenceText(seq) {
    return stringValue(callValue(seq, ["getSequenceAsString$"]));
  }

  function collectMSASequences(frame) {
    return alignmentSequences(frame).map((seq, index) => {
      const name = sequenceName(seq);
      const entry = selectionEntryForSequence(sequenceTaxonID(seq), name, index);
      return {
        taxon_id: sequenceTaxonID(seq) || (entry && entry.taxonID) || "",
        display_name: name,
        description: sequenceDescription(seq),
        sequence: sequenceText(seq),
        index
      };
    }).filter((row) => row.taxon_id || row.display_name || row.sequence);
  }

  function collectMSASettings(frame) {
    const viewport = mainViewport(frame);
    const settings = {};
    if (!viewport) return settings;
    const getters = {
      wrap_alignment: ["getWrapAlignment$", "isWrapAlignment$"],
      show_annotations: ["isShowAnnotation$", "getShowAnnotation$"],
      show_boxes: ["getShowBoxes$", "isShowBoxes$"],
      show_text: ["getShowText$", "isShowText$"],
      show_colour_text: ["getShowColourText$", "isShowColourText$"],
      show_sequence_features: ["isShowSequenceFeatures$", "getShowSequenceFeatures$"],
      scale_protein_as_cdna: ["isScaleProteinAsCdna$", "getScaleProteinAsCdna$"],
      residue_font: ["getFont$"],
      colour_scheme: ["getGlobalColourScheme$"]
    };
    for (const [key, names] of Object.entries(getters)) {
      setIfPresent(settings, key, callValue(viewport, names));
    }
    return settings;
  }

  function collectMSAGroups(frame) {
    const alignment = mainAlignment(frame);
    const rawGroups = alignment && callValue(alignment, ["getGroups$"]);
    return javaListToArray(rawGroups, 500).map((group, index) => {
      const out = { index };
      setIfPresent(out, "name", callValue(group, ["getName$"]));
      setIfPresent(out, "start", callValue(group, ["getStartRes$"]));
      setIfPresent(out, "end", callValue(group, ["getEndRes$"]));
      setIfPresent(out, "display_boxes", callValue(group, ["getDisplayBoxes$"]));
      setIfPresent(out, "display_text", callValue(group, ["getDisplayText$"]));
      setIfPresent(out, "colour_text", callValue(group, ["getColourText$"]));
      const colour = callValue(group, ["getColourScheme$"]);
      setIfPresent(out, "colour_scheme", colour);
      const sequences = javaListToArray(callValue(group, ["getSequences$"]), 500)
        .map(sequenceName)
        .filter(Boolean);
      if (sequences.length > 0) out.sequences = sequences;
      return out;
    });
  }

  function collectMSAAnnotations(frame) {
    const alignment = mainAlignment(frame);
    const annotations = alignment && callValue(alignment, ["getAlignmentAnnotation$"]);
    return javaListToArray(annotations, 500).map((annotation, index) => {
      const out = { index };
      setIfPresent(out, "label", callValue(annotation, ["getLabel$"]));
      setIfPresent(out, "description", callValue(annotation, ["getDescription$"]));
      setIfPresent(out, "visible", callValue(annotation, ["isVisible$", "getVisible$"]));
      setIfPresent(out, "below_alignment", callValue(annotation, ["isBelowAlignment$", "getBelowAlignment$"]));
      setIfPresent(out, "graph", callValue(annotation, ["getGraph$"]));
      const sequenceRef = callValue(annotation, ["getSequenceRef$"]);
      const name = sequenceName(sequenceRef);
      if (name) out.sequence = name;
      return out;
    });
  }

  function featureListForSequence(seq) {
    const direct = callValue(seq, ["getSequenceFeatures$"]);
    const fromDirect = javaListToArray(direct, 20000);
    if (fromDirect.length > 0) return fromDirect;
    const featureStore = callValue(seq, ["getFeatures$"]);
    let allFeatures = null;
    try {
      if (featureStore && typeof featureStore.getAllFeatures$SA === "function") {
        allFeatures = featureStore.getAllFeatures$SA([]);
      }
    } catch (_error) {
      allFeatures = null;
    }
    return javaListToArray(allFeatures, 20000);
  }

  function collectFeatureDetails(feature) {
    const out = {};
    setIfPresent(out, "type", callValue(feature, ["getType$"]));
    setIfPresent(out, "description", callValue(feature, ["getDescription$"]));
    setIfPresent(out, "begin", callValue(feature, ["getBegin$"]));
    setIfPresent(out, "end", callValue(feature, ["getEnd$"]));
    setIfPresent(out, "score", callValue(feature, ["getScore$"]));
    setIfPresent(out, "feature_group", callValue(feature, ["getFeatureGroup$"]));
    setIfPresent(out, "status", callValue(feature, ["getStatus$"]));
    setIfPresent(out, "strand", callValue(feature, ["getStrand$"]));
    setIfPresent(out, "phase", callValue(feature, ["getPhase$"]));
    setIfPresent(out, "attributes", callValue(feature, ["getAttributes$"]));
    setIfPresent(out, "ena_location", callValue(feature, ["getEnaLocation$"]));
    if (feature && feature.otherDetails) {
      const other = javaMapToObject(feature.otherDetails, 250);
      if (Object.keys(other).length > 0) out.other_details = other;
    }
    const links = javaListToArray(feature && feature.links, 100).map(stringValue).filter(Boolean);
    if (links.length > 0) out.links = links;
    return out;
  }

  function collectMSAFeatures(frame) {
    const out = [];
    const sequences = alignmentSequences(frame);
    for (let index = 0; index < sequences.length && out.length < 20000; index += 1) {
      const seq = sequences[index];
      const name = sequenceName(seq);
      const entry = selectionEntryForSequence(sequenceTaxonID(seq), name, index);
      const features = featureListForSequence(seq);
      for (let featureIndex = 0; featureIndex < features.length && out.length < 20000; featureIndex += 1) {
        const feature = collectFeatureDetails(features[featureIndex]);
        feature.sequence_index = index;
        feature.feature_index = featureIndex;
        feature.taxon_id = sequenceTaxonID(seq) || (entry && entry.taxonID) || "";
        feature.display_name = name;
        out.push(feature);
      }
    }
    return out;
  }

  function collectMSAState(trigger, options) {
    const frame = mainAlignmentFrame(null);
    const full = !options || options.full !== false;
    const state = {
      schema_version: 2,
      updated_at: new Date().toISOString(),
      rows: collectSelectionRows(),
      viewer_state: {
        trigger: String(trigger || ""),
        title: document.title,
        ready: document.body.classList.contains("phgo-jalview-ready")
      }
    };
    if (full) {
      state.sequences = collectMSASequences(frame);
      state.settings = collectMSASettings(frame);
      state.groups = collectMSAGroups(frame);
      state.annotations = collectMSAAnnotations(frame);
      state.features = collectMSAFeatures(frame);
    }
    return state;
  }

  let msaStateSaveTimer = 0;
  let msaStateSaveChain = Promise.resolve();
  async function saveMSAStateNow(trigger, options, fullState) {
    const session = currentSession();
    if (!session) return;
    const full = fullState !== false;
    const saveTask = msaStateSaveChain.catch(() => undefined).then(async () => {
      try {
        await putJSON(`/sessions/${encodeURIComponent(session)}/msa/state`, collectMSAState(trigger, { full }), options);
      } catch (error) {
        debug("msa-state-save-failed", { trigger, message: formatValue(error) });
      }
    });
    msaStateSaveChain = saveTask;
    await saveTask;
  }

  async function saveMSAStateManual() {
    await saveMSAStateNow("manual-save");
    showToast("MSA state saved.", false);
  }

  function scheduleMSAStateSave(trigger, delay, fullState) {
    if (msaStateSaveTimer) window.clearTimeout(msaStateSaveTimer);
    msaStateSaveTimer = window.setTimeout(() => {
      msaStateSaveTimer = 0;
      saveMSAStateNow(trigger || "debounced", undefined, fullState);
    }, Number.isFinite(delay) ? delay : 900);
  }

  function selectionEntryForSequence(taxonID, name, index) {
    const taxonKey = String(taxonID || "").trim();
    const selection = window.__PHGOMSASelection || new Map();
    if (taxonKey && selection.has(taxonKey)) return selection.get(taxonKey);
    const key = String(name || "").trim();
    const byName = window.__PHGOMSASelectionByName || new Map();
    if (key && byName.has(key)) return byName.get(key);
    if (key && selection.has(key)) return selection.get(key);
    const numericIndex = Number.isFinite(index) ? index : Number.parseInt(index, 10);
    const byIndex = window.__PHGOMSASelectionByIndex || new Map();
    if (Number.isFinite(numericIndex) && byIndex.has(numericIndex)) return byIndex.get(numericIndex);
    return null;
  }

  function selectionStateForSequence(taxonID, name, index) {
    const entry = selectionEntryForSequence(taxonID, name, index);
    return normalizeSelectionState(entry && entry.state);
  }

  function invalidateIdCanvas(idCanvas) {
    if (!idCanvas) return;
    idCanvas.fastPaint = false;
    idCanvas.image = null;
    idCanvas.imgHeight = 0;
  }

  function requestMSARepaint() {
    const frame = mainAlignmentFrame(null);
    if (!frame || !frame.alignPanel) return;
    try {
      const ap = frame.alignPanel;
      const idCanvas = ap.idPanel && ap.idPanel.idCanvas ? ap.idPanel.idCanvas : null;
      invalidateIdCanvas(idCanvas);
      if (ap.av && typeof ap.av.setIdWidth$I === "function") ap.av.setIdWidth$I(-1);
      if (typeof ap.calculateIdWidth$ === "function" && ap.idPanel && ap.idPanel.idCanvas && typeof ap.idPanel.idCanvas.setPreferredSize$java_awt_Dimension === "function") {
        ap.idPanel.idCanvas.setPreferredSize$java_awt_Dimension(ap.calculateIdWidth$());
      }
      if (typeof ap.paintAlignment$Z$Z === "function") {
        ap.paintAlignment$Z$Z(false, false);
      } else if (typeof ap.repaint$ === "function") {
        ap.repaint$();
      } else if (idCanvas && typeof idCanvas.repaint$ === "function") {
        idCanvas.repaint$();
      }
    } catch (error) {
      debug("msa-repaint-failed", { message: formatValue(error) });
    }
  }

  function toggleSelectionForSequence(taxonID, name, index, repaint) {
    const entry = selectionEntryForSequence(taxonID, name, index);
    if (!entry) return "green";
    entry.state = nextSelectionState(entry.state);
    const selection = window.__PHGOMSASelection || new Map();
    selection.set(entry.taxonID, entry);
    if (entry.displayName) {
      const byName = window.__PHGOMSASelectionByName || new Map();
      byName.set(entry.displayName, entry);
      window.__PHGOMSASelectionByName = byName;
    }
    if (entry.index >= 0) {
      const byIndex = window.__PHGOMSASelectionByIndex || new Map();
      byIndex.set(entry.index, entry);
      window.__PHGOMSASelectionByIndex = byIndex;
    }
    window.__PHGOMSASelection = selection;
    if (repaint !== false) requestMSARepaint();
    scheduleMSAStateSave("selection-toggle", 250, false);
    return entry.state;
  }

  async function applyMSASelection() {
    const session = currentSession();
    if (!session) return;
    const selection = window.__PHGOMSASelection || new Map();
    const frame = mainAlignmentFrame(null);
    const sequencesByTaxon = new Map();
    const sequencesByIndex = new Map();
    const sequencesByName = new Map();
    for (const seqRow of collectMSASequences(frame)) {
      if (seqRow.taxon_id) sequencesByTaxon.set(seqRow.taxon_id, seqRow);
      if (Number.isFinite(seqRow.index)) sequencesByIndex.set(seqRow.index, seqRow);
      if (seqRow.display_name) sequencesByName.set(seqRow.display_name, seqRow);
    }
    const rows = [...selection.values()].map((entry) => {
      const seqRow = sequencesByTaxon.get(entry.taxonID) || sequencesByIndex.get(entry.index) || sequencesByName.get(entry.displayName) || {};
      return {
        taxon_id: entry.taxonID,
        name: seqRow.display_name || entry.displayName || "",
        display_name: seqRow.display_name || entry.displayName || "",
        description: seqRow.description || "",
        sequence: seqRow.sequence || "",
        index: entry.index,
        state: normalizeSelectionState(entry.state)
      };
    });
    showToast("Refreshing tree and MSA...", true);
    try {
      await saveMSAStateNow("apply", undefined, true);
      await fetchJSON(`/sessions/${encodeURIComponent(session)}/msa/apply`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ rows })
      });
      window.location.replace(`/sessions/${encodeURIComponent(session)}/msa`);
    } catch (error) {
      showToast(`MSA Apply failed: ${formatValue(error)}`, false);
      throw error;
    }
  }

  function installMSAEvents() {
    const session = currentSession();
    if (session) {
      let reloadScheduled = false;
      try {
        const events = new EventSource(`/events/${encodeURIComponent(session)}`);
        events.addEventListener("update", async () => {
          try {
            const status = await fetchJSON(`/sessions/${encodeURIComponent(session)}/status`);
            if (status && status.refreshing) {
              showToast(status.message || "Refreshing tree and MSA...", true);
            } else if (document.querySelector(".phgo-warning-toast:not([hidden])")) {
              showToast("", true);
            }
          } catch (error) {
            debug("msa-status-failed", { message: formatValue(error) });
          }
          if (reloadScheduled) return;
          try {
            const currentUpdatedAt = await currentPayloadUpdatedAt(session);
            const loadedUpdatedAt = String((window.__PHGOJalviewState || {}).payloadUpdatedAt || "");
            if (currentUpdatedAt && loadedUpdatedAt && currentUpdatedAt !== loadedUpdatedAt) {
              reloadScheduled = true;
              showToast("Reloading MSA...", true);
              window.setTimeout(() => {
                window.location.replace(`/sessions/${encodeURIComponent(session)}/msa`);
              }, 100);
            }
          } catch (error) {
            debug("msa-payload-version-check-failed", { message: formatValue(error) });
          }
        });
      } catch (error) {
        debug("msa-events-failed", { message: formatValue(error) });
      }
    }
  }

  function attachPanelToFrame(frame, panel) {
    if (!frame || !panel) {
      return false;
    }
    if (typeof frame.setContentPane$java_awt_Container === "function") {
      frame.setContentPane$java_awt_Container(panel);
      return true;
    }
    const rootPane = typeof frame.getRootPane$ === "function" ? frame.getRootPane$() : null;
    if (rootPane && typeof rootPane.setContentPane$java_awt_Container === "function") {
      rootPane.setContentPane$java_awt_Container(panel);
      return true;
    }
    const contentPane = typeof frame.getContentPane$ === "function" ? frame.getContentPane$() : null;
    if (contentPane && typeof contentPane.add$java_awt_Component$O === "function") {
      contentPane.add$java_awt_Component$O(panel, "Center");
      return true;
    }
    if (contentPane && typeof contentPane.add$java_awt_Component === "function") {
      contentPane.add$java_awt_Component(panel);
      return true;
    }
    if (typeof frame.add$java_awt_Component$O === "function") {
      frame.add$java_awt_Component$O(panel, "Center");
      return true;
    }
    if (typeof frame.add$java_awt_Component === "function") {
      frame.add$java_awt_Component(panel);
      return true;
    }
    return false;
  }

  function addFrameToJalviewDesktop(desktopClass, frame, title, width, height) {
    if (desktopClass && typeof desktopClass.addInternalFrame$javax_swing_JInternalFrame$S$I$I === "function") {
      desktopClass.addInternalFrame$javax_swing_JInternalFrame$S$I$I(frame, title, width, height);
      return;
    }
    throw new Error("Jalview Desktop.addInternalFrame is required to register the msaexpor child window.");
  }

  function clazzClass(name) {
    if (!window.Clazz || typeof window.Clazz._4Name !== "function") return null;
    return window.Clazz._4Name(name, null, null, true) || window.Clazz._4Name(name);
  }

  function clazzNew(method, args) {
    const ctor = window.Clazz && typeof window.Clazz.new_ === "function"
      ? window.Clazz.new_.bind(window.Clazz)
      : (typeof window.Clazz_new_ === "function" ? window.Clazz_new_ : null);
    if (!ctor || !method) return null;
    return ctor(method, args || []);
  }

  function awtColor(red, green, blue) {
    const colorClass = clazzClass("java.awt.Color");
    if (!colorClass) return null;
    return clazzNew(colorClass.c$$I$I$I, [red, green, blue]);
  }

  function normalizeMSAExportSettings(settings) {
    const raw = settings && typeof settings === "object" ? settings : {};
    const scale = [1, 2, 5, 10].includes(Number(raw.scale)) ? Number(raw.scale) : 2;
    return {
      scale,
      cellWidth: Math.max(1, Math.round(Number(raw.cellWidth) || 9)),
      cellHeight: Math.max(1, Math.round(Number(raw.cellHeight) || 13)),
      showPHgoCoordinates: !!raw.showPHgoCoordinates,
      showLengthRatio: !!raw.showLengthRatio,
      showLengthPercent: !!raw.showLengthPercent,
      showAlignmentColumnNumbers: raw.showAlignmentColumnNumbers !== false,
      columnNumberInterval: Math.max(1, Math.round(Number(raw.columnNumberInterval) || 20)),
      showRightResidueNumbers: !!raw.showRightResidueNumbers || !!raw.useAdvancedLayoutScript,
      showGroups: raw.showGroups !== false,
      showFeatures: raw.showFeatures !== false,
      useAdvancedLayoutScript: !!raw.useAdvancedLayoutScript
    };
  }

  function rowLengthStatsFromSequence(seq) {
    const text = String(sequenceText(seq) || "");
    let last = -1;
    for (let i = text.length - 1; i >= 0; i -= 1) {
      const ch = text.charAt(i);
      if (ch !== "-" && ch !== "." && !/\s/.test(ch)) {
        last = i;
        break;
      }
    }
    let residues = 0;
    let total = 0;
    for (let i = 0; i < text.length; i += 1) {
      const ch = text.charAt(i);
      if (/\s/.test(ch)) continue;
      total += 1;
      if (ch === "-" || ch === ".") continue;
      if (ch === "*" && i === last) continue;
      residues += 1;
    }
    return { residues, total, percent: total ? residues / total * 100 : 0 };
  }

  function exportRowLabel(seq, index, settings) {
    const parts = [];
    const name = sequenceName(seq) || `row ${index + 1}`;
    if (settings.showPHgoCoordinates && window.__PHGOJalviewBridgeAPI && window.__PHGOJalviewBridgeAPI.displayPrefixForSequence) {
      const prefix = window.__PHGOJalviewBridgeAPI.displayPrefixForSequence(sequenceTaxonID(seq), name, index);
      if (prefix) parts.push(prefix);
    }
    parts.push(name);
    if (settings.showLengthRatio || settings.showLengthPercent) {
      const stats = rowLengthStatsFromSequence(seq);
      if (settings.showLengthRatio) parts.push(`${stats.residues}/${stats.total}`);
      if (settings.showLengthPercent) parts.push(`${stats.percent.toFixed(1)}%`);
    }
    return parts.filter(Boolean).join(" ");
  }

  function residueNumberAtExportEnd(seq, startBoundary, endBoundary) {
    if (!seq) return "";
    const text = String(sequenceText(seq) || "");
    const start = Math.max(0, Number(startBoundary) || 0);
    const end = Math.min(text.length, Math.max(start, Number(endBoundary) || 0));
    for (let column = end - 1; column >= start; column -= 1) {
      const ch = text.charAt(column);
      if (ch === "-" || ch === "." || /\s/.test(ch)) continue;
      try {
        if (typeof seq.findPosition$I === "function") return String(seq.findPosition$I(column));
      } catch (_error) {
        break;
      }
      return String(column + 1);
    }
    return "";
  }

  function blockRowsToIndexes(block, alignmentHeight) {
    const rows = Array.isArray(block && block.rows) ? block.rows : [];
    const out = [];
    const seen = new Set();
    for (const row of rows) {
      const index = Number(row && row.index);
      if (!Number.isInteger(index) || index < 0 || index >= alignmentHeight || seen.has(index)) continue;
      seen.add(index);
      out.push(index);
    }
    return out;
  }

  function setComponentSize(component, width, height) {
    if (!component) return;
    try {
      if (typeof component.setSize$I$I === "function") component.setSize$I$I(width, height);
    } catch (_error) {
      // Component size is a drawing hint only; continue with the existing size if SwingJS rejects it.
    }
  }

  function componentSize(component) {
    if (!component) return { width: 0, height: 0 };
    let width = 0;
    let height = 0;
    try {
      if (typeof component.getWidth$ === "function") width = Number(component.getWidth$()) || 0;
      if (typeof component.getHeight$ === "function") height = Number(component.getHeight$()) || 0;
    } catch (_error) {
      return { width: 0, height: 0 };
    }
    return { width, height };
  }

  function renderMSAExportScene(settings, layout) {
    const frame = mainAlignmentFrame(null);
    const alignPanel = frame && frame.alignPanel;
    const viewport = mainViewport(frame);
    const alignment = mainAlignment(frame);
    if (!alignPanel || !viewport || !alignment) {
      throw new Error("Jalview alignment is not ready for MSA export rendering.");
    }
    const seqPanel = typeof alignPanel.getSeqPanel$ === "function" ? alignPanel.getSeqPanel$() : alignPanel.seqPanel;
    const seqCanvas = seqPanel && seqPanel.seqCanvas;
    const idPanel = typeof alignPanel.getIdPanel$ === "function" ? alignPanel.getIdPanel$() : alignPanel.idPanel;
    const idCanvas = idPanel && (typeof idPanel.getIdCanvas$ === "function" ? idPanel.getIdCanvas$() : idPanel.idCanvas);
    if (!seqCanvas || !idCanvas) {
      throw new Error("Jalview sequence and ID canvases are not ready for MSA export rendering.");
    }
    const imageClass = clazzClass("java.awt.image.BufferedImage");
    if (!imageClass) throw new Error("SwingJS BufferedImage is not available for MSA export rendering.");
    const exportSettings = normalizeMSAExportSettings(settings);
    const blocks = Array.isArray(layout && layout.blocks) ? layout.blocks : [];
    if (blocks.length === 0) throw new Error("No layout blocks to render.");
    const alignmentHeight = Number(primitiveValue(callValue(alignment, ["getHeight$"]))) || alignmentSequences(frame).length;
    const renderedBlocks = blocks.map((block) => ({ block, indexes: blockRowsToIndexes(block, alignmentHeight) }));
    if (renderedBlocks.every((entry) => entry.indexes.length === 0)) {
      throw new Error("No renderable MSA rows in the export layout.");
    }
    const charWidthBefore = Number(primitiveValue(callValue(viewport, ["getCharWidth$"]))) || exportSettings.cellWidth;
    const charHeightBefore = Number(primitiveValue(callValue(viewport, ["getCharHeight$"]))) || exportSettings.cellHeight;
    try {
      if (typeof viewport.setCharWidth$I === "function") viewport.setCharWidth$I(exportSettings.cellWidth);
      if (typeof viewport.setCharHeight$I === "function") viewport.setCharHeight$I(exportSettings.cellHeight);
    } catch (error) {
      debug("msaexpor-char-size-failed", { message: formatValue(error) });
    }
    const charWidth = Number(primitiveValue(callValue(viewport, ["getCharWidth$"]))) || exportSettings.cellWidth;
    const charHeight = Number(primitiveValue(callValue(viewport, ["getCharHeight$"]))) || exportSettings.cellHeight;
    const topNumberHeight = exportSettings.showAlignmentColumnNumbers ? Math.max(18, charHeight + 7) : 4;
    const blockGap = 12;
    const margin = 14;
    const rightNumberWidth = exportSettings.showRightResidueNumbers ? 52 : 0;
    let scratch = null;
    let scratchGraphics = null;
    let fontMetrics = null;
    try {
      scratch = clazzNew(imageClass.c$$I$I$I, [16, 16, 1]);
      scratchGraphics = scratch && scratch.getGraphics$();
      if (scratchGraphics) {
        scratchGraphics.setFont$java_awt_Font(viewport.getFont$());
        fontMetrics = scratchGraphics.getFontMetrics$();
      }
    } catch (_error) {
      fontMetrics = null;
    } finally {
      if (scratchGraphics && typeof scratchGraphics.dispose$ === "function") {
        try { scratchGraphics.dispose$(); } catch (_error) {}
      }
    }
    let leftLabelWidth = 96;
    for (const entry of renderedBlocks) {
      for (const index of entry.indexes) {
        const seq = alignment.getSequenceAt$I(index);
        const label = exportRowLabel(seq, index, exportSettings);
        const measured = fontMetrics && typeof fontMetrics.stringWidth$S === "function" ? fontMetrics.stringWidth$S(label) : label.length * 7;
        leftLabelWidth = Math.max(leftLabelWidth, measured + 12);
      }
    }
    leftLabelWidth = Math.min(420, Math.ceil(leftLabelWidth));
    const maxBlockColumns = Math.max(1, ...renderedBlocks.map((entry) => Number(entry.block.alignmentWidthForNumbering || entry.block.visibleColumnCount || 0)));
    const gridWidth = maxBlockColumns * charWidth;
    const width = Math.ceil(margin * 2 + leftLabelWidth + gridWidth + rightNumberWidth);
    const height = Math.ceil(margin * 2 + renderedBlocks.reduce((sum, entry) => {
      return sum + topNumberHeight + entry.indexes.length * charHeight + blockGap;
    }, 0) - blockGap);
    const pixelWidth = Math.ceil(width * exportSettings.scale);
    const pixelHeight = Math.ceil(height * exportSettings.scale);
    const maxPixels = 160000000;
    if (pixelWidth * pixelHeight > maxPixels) {
      throw new Error(`MSA export is too large: ${pixelWidth} x ${pixelHeight} pixels.`);
    }
    const bridge = window.__PHGOJalviewBridgeAPI || {};
    const previousExportSettings = bridge.__msaexporRenderSettings;
    const previousExportActive = window.__PHGO_MSAEXPOR_RENDER_ACTIVE__;
    const previousIdSize = componentSize(idCanvas);
    const previousSeqSize = componentSize(seqCanvas);
    const image = clazzNew(imageClass.c$$I$I$I, [pixelWidth, pixelHeight, 1]);
    if (!image) throw new Error("Unable to allocate SwingJS export image.");
    let graphics = null;
    try {
      bridge.__msaexporRenderSettings = exportSettings;
      window.__PHGO_MSAEXPOR_RENDER_ACTIVE__ = true;
      graphics = image.getGraphics$();
      if (!graphics) throw new Error("Unable to create SwingJS export graphics.");
      if (exportSettings.scale !== 1 && typeof graphics.scale$D$D === "function") {
        graphics.scale$D$D(exportSettings.scale, exportSettings.scale);
      }
      if (viewport.antiAlias && clazzClass("java.awt.RenderingHints")) {
        // SeqCanvas applies the exact Jalview anti-aliasing hints during its own draw call.
      }
      graphics.setFont$java_awt_Font(viewport.getFont$());
      const white = awtColor(255, 255, 255);
      const black = awtColor(0, 0, 0);
      if (white) graphics.setColor$java_awt_Color(white);
      graphics.fillRect$I$I$I$I(0, 0, width, height);
      setComponentSize(idCanvas, leftLabelWidth, height);
      setComponentSize(seqCanvas, gridWidth, height);
      let y = margin;
      for (const entry of renderedBlocks) {
        const block = entry.block;
        const indexes = entry.indexes;
        if (indexes.length === 0) continue;
        const start = Math.max(0, Math.floor(Number(block.columnStartBoundary) || 0));
        const visibleCount = Math.max(0, Math.floor(Number(block.visibleColumnCount) || 0));
        const endExclusive = Math.max(start, Math.floor(Number(block.columnEndBoundary) || (start + visibleCount)));
        const drawEnd = Math.max(start, endExclusive - 1);
        const gridX = margin + leftLabelWidth;
        if (black) graphics.setColor$java_awt_Color(black);
        if (exportSettings.showAlignmentColumnNumbers) {
          const numberingColumns = Math.max(visibleCount, Math.floor(Number(block.alignmentWidthForNumbering) || visibleCount));
          for (let col = start + 1; col <= start + numberingColumns; col += 1) {
            if (col % exportSettings.columnNumberInterval !== 0) continue;
            const text = String(col);
            const textWidth = fontMetrics && typeof fontMetrics.stringWidth$S === "function" ? fontMetrics.stringWidth$S(text) : text.length * 7;
            const x = gridX + Math.round((col - start - 0.5) * charWidth) - Math.round(textWidth / 2);
            graphics.drawString$S$I$I(text, x, y + charHeight);
          }
        }
        const rowStartY = y + topNumberHeight;
        for (let rowIndex = 0; rowIndex < indexes.length; rowIndex += 1) {
          const seqIndex = indexes[rowIndex];
          const rowY = rowStartY + rowIndex * charHeight;
          graphics.translate$I$I(margin, rowY);
          idCanvas.drawIds$java_awt_Graphics2D$jalview_gui_AlignViewport$I$I$java_util_List(graphics, viewport, seqIndex, seqIndex, null);
          graphics.translate$I$I(-margin, -rowY);
          if (endExclusive > start) {
            graphics.translate$I$I(gridX, rowY);
            seqCanvas.drawPanel$java_awt_Graphics$I$I$I$I$I(graphics, start, drawEnd, seqIndex, seqIndex, 0);
            graphics.translate$I$I(-gridX, -rowY);
          }
          if (exportSettings.showRightResidueNumbers) {
            const seq = alignment.getSequenceAt$I(seqIndex);
            const text = residueNumberAtExportEnd(seq, start, endExclusive);
            if (text) {
              if (black) graphics.setColor$java_awt_Color(black);
              const textWidth = fontMetrics && typeof fontMetrics.stringWidth$S === "function" ? fontMetrics.stringWidth$S(text) : text.length * 7;
              const x = gridX + Math.max(visibleCount, Number(block.alignmentWidthForNumbering) || visibleCount) * charWidth + rightNumberWidth - textWidth - 8;
              graphics.drawString$S$I$I(text, Math.round(x), rowY + charHeight - Math.max(2, Math.floor(charHeight / 5)));
            }
          }
        }
        y += topNumberHeight + indexes.length * charHeight + blockGap;
      }
    } finally {
      bridge.__msaexporRenderSettings = previousExportSettings;
      if (previousExportActive) {
        window.__PHGO_MSAEXPOR_RENDER_ACTIVE__ = previousExportActive;
      } else {
        delete window.__PHGO_MSAEXPOR_RENDER_ACTIVE__;
      }
      if (graphics && typeof graphics.dispose$ === "function") {
        try { graphics.dispose$(); } catch (_error) {}
      }
      try {
        if (typeof viewport.setCharWidth$I === "function") viewport.setCharWidth$I(charWidthBefore);
        if (typeof viewport.setCharHeight$I === "function") viewport.setCharHeight$I(charHeightBefore);
        if (previousIdSize.width > 0 && previousIdSize.height > 0) setComponentSize(idCanvas, previousIdSize.width, previousIdSize.height);
        if (previousSeqSize.width > 0 && previousSeqSize.height > 0) setComponentSize(seqCanvas, previousSeqSize.width, previousSeqSize.height);
      } catch (error) {
        debug("msaexpor-char-restore-failed", { message: formatValue(error) });
      }
    }
    const canvas = image._canvas || image._imgNode;
    if (!canvas || typeof canvas.toDataURL !== "function") {
      throw new Error("SwingJS export image did not expose a canvas.");
    }
    const dataURL = canvas.toDataURL("image/png");
    const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}" viewBox="0 0 ${width} ${height}" data-msaexpor="1" data-msaexpor-renderer="jalview-native"><image href="${dataURL}" x="0" y="0" width="${width}" height="${height}" preserveAspectRatio="none"/></svg>`;
    return { svg, width, height, raster: true, source: "jalview-native", blocks: blocks.length };
  }

  function createSwingChildWindow(title, width, height) {
    const frameClass = clazzClass("javax.swing.JInternalFrame");
    const panelClass = clazzClass("javax.swing.JPanel");
    const desktopClass = clazzClass("jalview.gui.Desktop");
    const newJavaObject = window.Clazz && typeof window.Clazz.new_ === "function"
      ? window.Clazz.new_.bind(window.Clazz)
      : (typeof window.Clazz_new_ === "function" ? window.Clazz_new_ : null);
    if (!frameClass || !panelClass || !desktopClass || !newJavaObject) {
      throw new Error("SwingJS internal-frame classes are not ready.");
    }
    const frame = newJavaObject(frameClass.c$$S$Z$Z$Z$Z, [title, true, true, true, true]);
    const panel = newJavaObject(panelClass.c$, []);
    panel.__phgoMSAExportPanel = true;
    frame.__phgoMSAExportFrame = true;
    const attached = attachPanelToFrame(frame, panel);
    if (!attached) {
      throw new Error("SwingJS child-window content pane is not available for msaexpor.");
    }
    addFrameToJalviewDesktop(desktopClass, frame, title, width, height);
    return { frame, panel };
  }

  function frameDOMNodeByTitle(title) {
    const wanted = String(title || "").trim();
    const candidates = Array.from(document.querySelectorAll(".swingjs-window, [role='dialog'], [id*='InternalFrame'], [id*='FrameUI'], [id*='RootPaneUI'], [id*='LayeredPaneUI']"));
    const visible = candidates.filter((node) => {
      const rect = node.getBoundingClientRect();
      const style = window.getComputedStyle(node);
      return rect.width > 0 && rect.height > 0 && style.display !== "none" && style.visibility !== "hidden";
    });
    const titled = visible.find((node) => !wanted || String(node.textContent || "").includes(wanted));
    if (titled) return titled;
    const textNodes = Array.from(document.querySelectorAll("body *")).filter((node) => {
      const rect = node.getBoundingClientRect();
      return rect.width > 0 && rect.height > 0 && String(node.textContent || "").trim() === wanted;
    });
    for (const node of textNodes) {
      let parent = node.parentElement;
      while (parent && parent !== document.body) {
        if (parent.matches(".swingjs-window, [role='dialog'], [id*='InternalFrame'], [id*='FrameUI'], [id*='RootPaneUI'], [id*='LayeredPaneUI']")) {
          return parent;
        }
        parent = parent.parentElement;
      }
    }
    return null;
  }

  function panelDOMNode(panel, frame, title) {
    const candidates = [
      panel && panel._j2sNode,
      panel && panel._j2sObject,
      panel && panel.domNode,
      panel && panel.html5Applet,
      panel && panel.ui && panel.ui.domNode,
      panel && panel.ui && panel.ui.jqNode && panel.ui.jqNode[0],
      frame && frame._j2sNode,
      frame && frame._j2sObject,
      frame && frame.domNode,
      frame && frame.ui && frame.ui.domNode,
      frame && frame.ui && frame.ui.jqNode && frame.ui.jqNode[0],
      frameDOMNodeByTitle(title)
    ];
    for (const candidate of candidates) {
      if (candidate instanceof HTMLElement) return candidate;
    }
    const nodes = Array.from(document.querySelectorAll("[id*='JPanel'], .swingjs-component, [data-component]"));
    return nodes.reverse().find((node) => node instanceof HTMLElement && node.id && node.offsetParent !== null) || null;
  }

  function configureMSAExportFrameWindow(frameWindow, bridge) {
    if (!frameWindow) return;
    frameWindow.__PHGO_MSAEXPOR_PARENT_WINDOW__ = window;
    frameWindow.__PHGOJalviewBridgeAPI = bridge || window.__PHGOJalviewBridgeAPI;
    if (typeof window.__PHGO_SAVE_BLOB__ === "function") {
      frameWindow.__PHGO_SAVE_BLOB__ = (blob, filename, options) => window.__PHGO_SAVE_BLOB__(blob, filename, options);
    } else {
      delete frameWindow.__PHGO_SAVE_BLOB__;
    }
  }

  function writeMSAExportIframeDocument(doc) {
    doc.open();
    doc.write("<!doctype html><html class=\"phgo-msaexpor-doc\"><head><meta charset=\"utf-8\"><link rel=\"stylesheet\" href=\"/assets/msaexpor/style.css\"><script>window.__PHGOmsaexporLoadError='';window.addEventListener('error',function(event){window.__PHGOmsaexporLoadError=(event&&event.message)||'MSA export iframe script error';});window.addEventListener('unhandledrejection',function(event){window.__PHGOmsaexporLoadError=(event&&event.reason&&event.reason.message)||String(event&&event.reason||'MSA export iframe rejected');});</script><script src=\"/assets/msaexpor/pdf.js\" onerror=\"window.__PHGOmsaexporLoadError='Failed to load /assets/msaexpor/pdf.js';\"></script><script src=\"/assets/msaexpor/index.js\" onerror=\"window.__PHGOmsaexporLoadError='Failed to load /assets/msaexpor/index.js';\"></script></head><body><div class=\"phgo-msaexpor-root\"></div></body></html>");
    doc.close();
  }

  function waitForMSAExportFrameRuntime(iframe) {
    return new Promise((resolve, reject) => {
      const started = Date.now();
      const tick = () => {
        const frameWindow = iframe.contentWindow;
        const doc = iframe.contentDocument || frameWindow && frameWindow.document;
        if (frameWindow && frameWindow.PHGOmsaexpor && typeof frameWindow.PHGOmsaexpor.renderApp === "function" && doc && doc.querySelector(".phgo-msaexpor-root")) {
          resolve({ frameWindow, doc });
          return;
        }
        const loadError = frameWindow && frameWindow.__PHGOmsaexporLoadError;
        if (loadError) {
          reject(new Error(loadError));
          return;
        }
        if (Date.now() - started > 8000) {
          reject(new Error("Timed out loading the MSA export iframe runtime."));
          return;
        }
        window.setTimeout(tick, 25);
      };
      tick();
    });
  }

  async function mountMSAExportHost(hostParent, session, bridge) {
    let iframe = hostParent.querySelector("iframe.phgo-msaexpor-frame");
    if (!iframe) {
      hostParent.innerHTML = "";
      hostParent.classList.add("phgo-msaexpor-window-host");
      iframe = document.createElement("iframe");
      iframe.className = "phgo-msaexpor-frame";
      iframe.title = "PHgo MSA export image settings";
      iframe.setAttribute("src", "about:blank");
      hostParent.appendChild(iframe);
    }
    const doc = iframe.contentDocument || iframe.contentWindow && iframe.contentWindow.document;
    if (!doc) throw new Error("MSA export iframe is not ready.");
    configureMSAExportFrameWindow(iframe.contentWindow, bridge);
    if (!doc.body || !doc.querySelector(".phgo-msaexpor-root") || !iframe.contentWindow.PHGOmsaexpor || typeof iframe.contentWindow.PHGOmsaexpor.renderApp !== "function") {
      writeMSAExportIframeDocument(doc);
      configureMSAExportFrameWindow(iframe.contentWindow, bridge);
    }
    const runtime = await waitForMSAExportFrameRuntime(iframe);
    configureMSAExportFrameWindow(runtime.frameWindow, bridge);
    const host = runtime.doc.querySelector(".phgo-msaexpor-root");
    if (!host) throw new Error("MSA export iframe host is not ready.");
    runtime.frameWindow.PHGOmsaexpor.renderApp(host, { bridge, session, parentWindow: window });
    return { iframe, host };
  }

  async function openMSAExportImageWindow() {
    const session = currentSession();
    if (!session) throw new Error("MSA export requires an active PHgo session.");
    closeAllSwingMenus("msaexpor-open");
    if (window.__PHGOMSAExportWindow && window.__PHGOMSAExportWindow.host && document.contains(window.__PHGOMSAExportWindow.host)) {
      if (window.__PHGOMSAExportWindow.frame && typeof window.__PHGOMSAExportWindow.frame.toFront$ === "function") {
        try { window.__PHGOMSAExportWindow.frame.toFront$(); } catch (_error) {}
      }
      const mounted = await mountMSAExportHost(window.__PHGOMSAExportWindow.hostParent, session, window.__PHGOJalviewBridgeAPI);
      window.__PHGOMSAExportWindow.host = mounted.host;
      window.__PHGOMSAExportWindow.iframe = mounted.iframe;
      return window.__PHGOMSAExportWindow;
    }
    let frame;
    let hostParent;
    try {
      const created = createSwingChildWindow("Export image...", 760, 620);
      frame = created.frame;
      hostParent = panelDOMNode(created.panel, frame, "Export image...");
    } catch (error) {
      debug("msaexpor-swing-window-failed", { message: formatValue(error) });
      throw error;
    }
    if (!hostParent) throw new Error("Unable to locate the Jalview/SwingJS child window host required by msaexpor.");
    const mounted = await mountMSAExportHost(hostParent, session, window.__PHGOJalviewBridgeAPI);
    window.__PHGOMSAExportWindow = { frame, host: mounted.host, iframe: mounted.iframe, hostParent };
    return window.__PHGOMSAExportWindow;
  }

  async function openMSAExportImageWindowSafe() {
    try {
      return await openMSAExportImageWindow();
    } catch (error) {
      const message = formatValue(error);
      debug("msaexpor-open-failed", { message });
      showToast(`MSA export window failed: ${message}`, false);
      throw error;
    }
  }

  async function installMSASelectionBridge() {
    try {
      await loadMSASelection();
    } catch (error) {
      debug("msa-selection-load-failed", { message: formatValue(error) });
    }
    installMSAEvents();
    window.setTimeout(requestMSARepaint, 200);
    window.setTimeout(requestMSARepaint, 1000);
    window.setTimeout(() => scheduleMSAStateSave("startup", 200, true), 1200);
  }

  function resizeMainAlignmentFrame() {
    const size = viewportSize();
    const alignment = document.getElementById("jalview-alignment-div");
    const desktopNode = document.getElementById("jalview-desktop-div");
    if (alignment) {
      alignment.style.width = `${size.width}px`;
      alignment.style.height = `${size.height}px`;
    }
    if (desktopNode) {
      desktopNode.style.width = `${size.width}px`;
      desktopNode.style.height = `${size.height}px`;
    }
    if (typeof window.__PHGOJalviewResizeMainAlignment === "function") {
      window.__PHGOJalviewResizeMainAlignment();
    }
  }

  let resizeTimer = 0;
  function scheduleResize() {
    if (typeof window.__PHGOJalviewScheduleResizeMainAlignment === "function") {
      window.__PHGOJalviewScheduleResizeMainAlignment();
      return;
    }
    if (resizeTimer) window.clearTimeout(resizeTimer);
    resizeTimer = window.setTimeout(() => {
      resizeTimer = 0;
      adaptLayout();
    }, 260);
  }

  function adaptLayout() {
    const desktop = document.getElementById("jalview-desktop-div");
    const alignment = document.getElementById("jalview-alignment-div");
    if (!alignment) return false;
    const alignmentReady = alignment.childElementCount > 0 || alignment.innerHTML.length > 1000;

    if (desktop && alignmentReady) {
      desktop.removeAttribute("aria-hidden");
      desktop.classList.add("phgo-window-manager");
    }

    resizeMainAlignmentFrame();

    if (alignmentReady) {
      document.body.classList.add("phgo-jalview-ready");
      if (!adaptLayout.msaStateSaveScheduled) {
        adaptLayout.msaStateSaveScheduled = true;
        scheduleMSAStateSave("layout-ready", 600, true);
      }
    }
    return alignmentReady;
  }

  function closeAllSwingMenus(reason) {
    try {
      if (window.jQuery) {
        window.jQuery(".ui-j2smenu").each(function () {
          const menu = window.jQuery(this).data("ui-j2smenu");
          if (menu && typeof menu.collapseAll === "function") {
            menu.collapseAll({ type: reason || "phgo-close", target: document.body }, true, "phgo");
          }
        });
      }
    } catch (error) {
      debug("hide-menus-failed", { reason, method: "j2smenu.collapseAll", message: formatValue(error) });
    }
    try {
      if (window.J2S && window.J2S.Swing && typeof window.J2S.Swing.hideMenus === "function" && window.J2S.thisApplet) {
        window.J2S.Swing.hideMenus(window.J2S.thisApplet);
      }
    } catch (error) {
      debug("hide-menus-failed", { reason, method: "J2S.Swing.hideMenus", message: formatValue(error) });
    }
    try {
      if (window.Clazz && typeof window.Clazz._4Name === "function") {
        const componentUI = window.Clazz._4Name("swingjs.plaf.JSComponentUI");
        if (componentUI && typeof componentUI.hideMenusAndToolTip$ === "function") componentUI.hideMenusAndToolTip$();
        const popupUI = window.Clazz._4Name("swingjs.plaf.JSPopupMenuUI");
        if (popupUI && typeof popupUI.closeAllMenus$ === "function") popupUI.closeAllMenus$();
      }
    } catch (error) {
      debug("hide-menus-failed", { reason, method: "SwingJS plaf close", message: formatValue(error) });
    }
    try {
      document.querySelectorAll(".swingjsPopupMenu, .swingjs-popup, .swingjs-menu, [role='menu'], [role='j2smenu']").forEach((node) => {
        if (!(node instanceof HTMLElement)) return;
        if (node.closest("[id*='_MenuBarUI']")) return;
        node.style.removeProperty("visibility");
        node.style.display = "none";
      });
      document.querySelectorAll(".ui-j2smenu[aria-hidden='true']").forEach((node) => {
        if (!(node instanceof HTMLElement)) return;
        node.style.removeProperty("visibility");
      });
      document.querySelectorAll(".ui-j2smenu-node.ui-state-active, .ui-j2smenu-node.ui-state-focus").forEach((node) => {
        if (!(node instanceof HTMLElement)) return;
        node.classList.remove("ui-state-active", "ui-state-focus");
      });
    } catch (error) {
      debug("hide-menus-failed", { reason, method: "dom-menu-close-secondary", message: formatValue(error) });
    }
  }

  function hideMenusFromOutsideEvent(event) {
    const target = event && event.target;
    if (!target || !(target instanceof Element)) return;
    if (target.closest(".swingjsPopupMenu, .swingjs-popup, .swingjs-menu, [role='menu'], .ui-j2smenu-node")) {
      return;
    }
    if (target.closest("[id*='_MenuBarUI'], [id*='_MenuUI'], [id*='_MenuItemUI'], [id*='_CheckBoxMenuItemUI'], [id*='_RadioButtonMenuItemUI']")) {
      return;
    }
    closeAllSwingMenus(event.type || "outside");
  }

  function hideMenusOnEscape(event) {
    if (event && event.key === "Escape") closeAllSwingMenus("escape");
  }

  function installLayoutBridge() {
    window.addEventListener("resize", scheduleResize);
    window.addEventListener("load", scheduleResize);
    document.addEventListener("pointerdown", hideMenusFromOutsideEvent, true);
    document.addEventListener("mousedown", hideMenusFromOutsideEvent, true);
    document.addEventListener("touchstart", hideMenusFromOutsideEvent, true);
    document.addEventListener("wheel", hideMenusFromOutsideEvent, true);
    document.addEventListener("keydown", hideMenusOnEscape, true);
    window.addEventListener("message", (event) => {
      const data = event.data || {};
      if (data.source === "phgo-jalview-host" && data.type === "resize") {
        scheduleResize();
      }
    });
    if (window.visualViewport) {
      window.visualViewport.addEventListener("resize", scheduleResize);
    }
    window.setTimeout(adaptLayout, 0);
    window.setTimeout(adaptLayout, 250);
    window.setTimeout(adaptLayout, 1000);
    installMSASelectionBridge();
  }

  function init() {
    let state = {};
    try {
      state = parseStateFromHash();
    } catch (error) {
      notify("error", { message: `Unable to parse PHgo JalviewJS state: ${formatValue(error)}` });
    }
    const phgoState = installPHgoState(state);
    const openTarget = String(state.open || "").trim();
    const argsTarget = toBootstrapRelativePath(openTarget);
    const title = String(state.title || "PHgo JalviewJS").trim() || "PHgo JalviewJS";
    document.title = title;

    const api = {
      state,
      phgoState,
      openTarget,
      argsTarget,
      args: argsTarget ? ["open", argsTarget] : null,
      title,
      notify,
      collectState,
      adaptLayout,
      installLayoutBridge,
      resizeMainAlignmentFrame,
      applyMSASelection,
      selectionStateForSequence,
      toggleSelectionForSequence,
      collectMSAState,
      renderMSAExportScene,
      saveMSAStateNow,
      saveMSAStateManual,
      openMSAExportImageWindow,
      openMSAExportImageWindowSafe,
      scheduleMSAStateSave,
      closeAllSwingMenus,
      invalidateIdCanvas,
      checkboxColumnWidth: () => 16,
      displayPrefixForSequence: (taxonID, name, index) => {
        const entry = selectionEntryForSequence(taxonID, name, index);
        return entry && entry.displayPrefix ? entry.displayPrefix : "";
      },
      formatValue,
      debug
    };
    window.__PHGOJalviewBridgeAPI = api;
    return api;
  }

  window.PHGOJalviewBridge = { init };
})();
