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
      debug("hide-menus-failed", { reason, method: "dom-menu-close-fallback", message: formatValue(error) });
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
      saveMSAStateNow,
      saveMSAStateManual,
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
