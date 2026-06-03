import { readFileSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const scriptDir = dirname(fileURLToPath(import.meta.url));
const packageRoot = join(scriptDir, '..', 'node_modules', 'reactreejs');
const files = [join(packageRoot, 'dist', 'index.mjs')];

const dprAnchor = 'const AXIS_H = 24;';
const dprPatch = `${dprAnchor}
    const PHGO_DPR = Math.max(1, Math.min(3, window.devicePixelRatio || 1));`;

const labelOriginal = `      if (lblCanvas.width !== LBL_W) lblCanvas.width = LBL_W;
      if (lblCanvas.height !== H) lblCanvas.height = H;
      const lc = lblCanvas.getContext("2d");
      if (lc) {`;

const labelPatched = `      const lblPixelW = Math.max(1, Math.round(LBL_W * PHGO_DPR));
      const lblPixelH = Math.max(1, Math.round(H * PHGO_DPR));
      if (lblCanvas.width !== lblPixelW) lblCanvas.width = lblPixelW;
      if (lblCanvas.height !== lblPixelH) lblCanvas.height = lblPixelH;
      lblCanvas.style.width = \`\${LBL_W}px\`;
      lblCanvas.style.height = \`\${H}px\`;
      const lc = lblCanvas.getContext("2d");
      if (lc) {
        lc.setTransform(PHGO_DPR, 0, 0, PHGO_DPR, 0, 0);`;

const seqOriginal = `    if (seqCanvas.width !== totalW) seqCanvas.width = totalW;
    if (seqCanvas.height !== H) seqCanvas.height = H;
    const sc = seqCanvas.getContext("2d");
    if (!sc) return;`;

const seqPatched = `    const seqPixelW = Math.max(1, Math.round(totalW * PHGO_DPR));
    const seqPixelH = Math.max(1, Math.round(H * PHGO_DPR));
    if (seqCanvas.width !== seqPixelW) seqCanvas.width = seqPixelW;
    if (seqCanvas.height !== seqPixelH) seqCanvas.height = seqPixelH;
    seqCanvas.style.width = \`\${totalW}px\`;
    seqCanvas.style.height = \`\${H}px\`;
    const sc = seqCanvas.getContext("2d");
    if (!sc) return;
    sc.setTransform(PHGO_DPR, 0, 0, PHGO_DPR, 0, 0);`;

const parseOriginal = `function parseNewick(newick) {
  let index = 0;
  function parseSubtree() {
    const node = {};
    if (newick[index] === "(") {
      index++;
      node.children = [parseSubtree()];
      while (newick[index] === ",") {
        index++;
        node.children.push(parseSubtree());
      }
      if (newick[index] === ")") index++;
    }
    let name = "";
    while (index < newick.length && ![":", ",", ")", "("].includes(newick[index])) {
      name += newick[index++];
    }
    node.name = name.trim();
    if (newick[index] === ":") {
      index++;
      let lengthStr = "";
      while (index < newick.length && ![":", ",", ")", "("].includes(newick[index])) {
        lengthStr += newick[index++];
      }
      node.length = parseFloat(lengthStr);
    }
    return node;
  }
  const tree = parseSubtree();
  return tree;
}`;

const parsePatched = `function parseNewick(newick) {
  let index = 0;
  const DELIMS = [":", ",", ")", "(", ";"];
  function skipWhitespace() {
    while (index < newick.length && /\\s/.test(newick[index])) index++;
  }
  function readQuotedName() {
    let name = "";
    index++;
    while (index < newick.length) {
      const ch = newick[index];
      if (ch === "'") {
        if (newick[index + 1] === "'") {
          name += "'";
          index += 2;
          continue;
        }
        index++;
        break;
      }
      name += ch;
      index++;
    }
    return name;
  }
  function readName() {
    skipWhitespace();
    if (newick[index] === "'") return readQuotedName();
    let name = "";
    while (index < newick.length && !DELIMS.includes(newick[index])) {
      name += newick[index++];
    }
    return name.trim();
  }
  function readLength() {
    skipWhitespace();
    let lengthStr = "";
    while (index < newick.length && !DELIMS.includes(newick[index])) {
      lengthStr += newick[index++];
    }
    return parseFloat(lengthStr.trim());
  }
  function parseSubtree() {
    skipWhitespace();
    const node = {};
    if (newick[index] === "(") {
      index++;
      node.children = [parseSubtree()];
      skipWhitespace();
      while (newick[index] === ",") {
        index++;
        node.children.push(parseSubtree());
        skipWhitespace();
      }
      if (newick[index] === ")") index++;
    }
    skipWhitespace();
    node.name = readName();
    skipWhitespace();
    if (newick[index] === ":") {
      index++;
      node.length = readLength();
    }
    skipWhitespace();
    return node;
  }
  const tree = parseSubtree();
  skipWhitespace();
  if (newick[index] === ";") index++;
  return tree;
}`;

const toNewickOriginal = `function toNewick(node) {
  if (!node.children || !node.children.length) {
    const name2 = (node.name || "").replace(/ /g, "_");
    const len2 = node.length != null && !isNaN(node.length) ? \`:\${node.length}\` : "";
    return \`\${name2}\${len2}\`;
  }
  const inner = node.children.map(toNewick).join(",");
  const name = node.name || "";
  const len = node.length != null && !isNaN(node.length) ? \`:\${node.length}\` : "";
  return \`(\${inner})\${name}\${len}\`;
}`;

const toNewickPatched = `function formatNewickName(name) {
  const value = String(name || "");
  if (!value) return "";
  if (/^[^\\s\\(\\)\\[\\]'":;,]+$/.test(value)) return value;
  return \`'\${value.replace(/'/g, "''")}'\`;
}
function toNewick(node) {
  if (!node.children || !node.children.length) {
    const name2 = formatNewickName(node.name || "");
    const len2 = node.length != null && !isNaN(node.length) ? \`:\${node.length}\` : "";
    return \`\${name2}\${len2}\`;
  }
  const inner = node.children.map(toNewick).join(",");
  const name = formatNewickName(node.name || "");
  const len = node.length != null && !isNaN(node.length) ? \`:\${node.length}\` : "";
  return \`(\${inner})\${name}\${len}\`;
}`;

const displayNameAnchor = `function triggerDownload(url, filename) {
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  setTimeout(() => URL.revokeObjectURL(url), 200);
}`;

const displayNamePatched = `function displayTreeName(name) {
  return String(name || "");
}
function triggerDownload(url, filename) {
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  setTimeout(() => URL.revokeObjectURL(url), 200);
}`;

const triggerDownloadPatched = `function triggerDownload(url, filename) {
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  setTimeout(() => URL.revokeObjectURL(url), 200);
}`;

const triggerDownloadBridgePatched = `async function triggerDownload(url, filename, options) {
  const saveURL = typeof window !== "undefined" ? window.__PHGO_SAVE_URL__ : null;
  if (typeof saveURL === "function") {
    try {
      const saved = await saveURL(url, filename, options || {});
      if (saved === false || saved) {
        setTimeout(() => URL.revokeObjectURL(url), 200);
        return;
      }
    } catch (error) {
      const reportError = typeof window !== "undefined" ? window.__PHGO_REPORT_EXPORT_ERROR__ : null;
      if (typeof reportError === "function") {
        reportError(error);
        return;
      }
      throw error;
    }
  }
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  setTimeout(() => URL.revokeObjectURL(url), 200);
}`;

const handleDownloadOriginal = `  const handleDownload = useCallback(async (format) => {
    if (!svgRef.current) return;
    const svgEl = svgRef.current;
    const width = svgEl.width?.baseVal?.value || svgEl.clientWidth || 800;
    const height = svgEl.height?.baseVal?.value || svgEl.clientHeight || 600;
    const clone = svgEl.cloneNode(true);
    clone.setAttribute("xmlns", "http://www.w3.org/2000/svg");
    clone.setAttribute("width", String(width));
    clone.setAttribute("height", String(height));
    const isDark = document.documentElement.getAttribute("data-theme") === "dark";
    const bgColor = isDark ? "#1e293b" : "#ffffff";
    const bgRect = document.createElementNS("http://www.w3.org/2000/svg", "rect");
    bgRect.setAttribute("width", String(width));
    bgRect.setAttribute("height", String(height));
    bgRect.setAttribute("fill", bgColor);
    clone.insertBefore(bgRect, clone.firstChild);
    const svgString = new XMLSerializer().serializeToString(clone);
    if (format === "svg") {
      const blob = new Blob([svgString], { type: "image/svg+xml;charset=utf-8" });
      triggerDownload(URL.createObjectURL(blob), "phylotree.svg");
      return;
    }
    const scale = 2;
    const canvas = document.createElement("canvas");
    canvas.width = width * scale;
    canvas.height = height * scale;
    const ctx = canvas.getContext("2d");
    ctx.scale(scale, scale);
    ctx.fillStyle = bgColor;
    ctx.fillRect(0, 0, width, height);
    await new Promise((resolve) => {
      const img = new Image();
      const svgBlob = new Blob([svgString], { type: "image/svg+xml;charset=utf-8" });
      const url = URL.createObjectURL(svgBlob);
      img.onload = () => {
        ctx.drawImage(img, 0, 0);
        URL.revokeObjectURL(url);
        resolve();
      };
      img.onerror = () => {
        URL.revokeObjectURL(url);
        resolve();
      };
      img.src = url;
    });
    if (format === "png") {
      canvas.toBlob((blob) => {
        if (blob) triggerDownload(URL.createObjectURL(blob), "phylotree.png");
      }, "image/png");
      return;
    }
    const { jsPDF } = await import("jspdf");
    const mmW = width * 0.264583;
    const mmH = height * 0.264583;
    const doc = new jsPDF({
      orientation: width >= height ? "l" : "p",
      unit: "mm",
      format: [mmW, mmH]
    });
    doc.addImage(canvas.toDataURL("image/png"), "PNG", 0, 0, mmW, mmH);
    doc.save("phylotree.pdf");
  }, []);`;

const handleDownloadPatchedLegacy = `  const handleDownload = useCallback(async (format) => {
    if (!svgRef.current) return;
    const svgEl = svgRef.current;
    const width = svgEl.width?.baseVal?.value || svgEl.clientWidth || 800;
    const height = svgEl.height?.baseVal?.value || svgEl.clientHeight || 600;
    const clone = svgEl.cloneNode(true);
    clone.setAttribute("xmlns", "http://www.w3.org/2000/svg");
    clone.setAttribute("width", String(width));
    clone.setAttribute("height", String(height));
    const isDark = document.documentElement.getAttribute("data-theme") === "dark";
    const bgColor = isDark ? "#1e293b" : "#ffffff";
    const bgRect = document.createElementNS("http://www.w3.org/2000/svg", "rect");
    bgRect.setAttribute("width", String(width));
    bgRect.setAttribute("height", String(height));
    bgRect.setAttribute("fill", bgColor);
    clone.insertBefore(bgRect, clone.firstChild);
    const svgString = new XMLSerializer().serializeToString(clone);
    const exportTree = typeof window !== "undefined" ? window.__PHGO_EXPORT_TREE__ : null;
    if (typeof exportTree === "function") {
      try {
        await exportTree({ format, svgString, width, height, bgColor, baseName: "phylotree" });
        return;
      } catch (error) {
        const reportError = typeof window !== "undefined" ? window.__PHGO_REPORT_EXPORT_ERROR__ : null;
        if (typeof reportError === "function") {
          reportError(error);
          return;
        }
        throw error;
      }
    }
    if (format === "svg") {
      const blob = new Blob([svgString], { type: "image/svg+xml;charset=utf-8" });
      triggerDownload(URL.createObjectURL(blob), "phylotree.svg");
      return;
    }
    const scale = 10;
    const canvas = document.createElement("canvas");
    canvas.width = width * scale;
    canvas.height = height * scale;
    const ctx = canvas.getContext("2d");
    ctx.scale(scale, scale);
    ctx.fillStyle = bgColor;
    ctx.fillRect(0, 0, width, height);
    await new Promise((resolve) => {
      const img = new Image();
      const svgBlob = new Blob([svgString], { type: "image/svg+xml;charset=utf-8" });
      const url = URL.createObjectURL(svgBlob);
      img.onload = () => {
        ctx.drawImage(img, 0, 0);
        URL.revokeObjectURL(url);
        resolve();
      };
      img.onerror = () => {
        URL.revokeObjectURL(url);
        resolve();
      };
      img.src = url;
    });
    if (format === "png") {
      canvas.toBlob((blob) => {
        if (blob) triggerDownload(URL.createObjectURL(blob), "phylotree.png");
      }, "image/png");
      return;
    }
    const { jsPDF } = await import("jspdf");
    const mmW = width * 0.264583;
    const mmH = height * 0.264583;
    const doc = new jsPDF({
      orientation: width >= height ? "l" : "p",
      unit: "mm",
      format: [mmW, mmH]
    });
    doc.addImage(canvas.toDataURL("image/png"), "PNG", 0, 0, mmW, mmH);
    doc.save("phylotree.pdf");
  }, []);`;

const handleDownloadPatched = `  const handleDownload = useCallback(async (format) => {
    if (!svgRef.current) return;
    const svgEl = svgRef.current;
    const width = svgEl.width?.baseVal?.value || svgEl.clientWidth || 800;
    const height = svgEl.height?.baseVal?.value || svgEl.clientHeight || 600;
    const clone = svgEl.cloneNode(true);
    clone.setAttribute("xmlns", "http://www.w3.org/2000/svg");
    clone.setAttribute("width", String(width));
    clone.setAttribute("height", String(height));
    const svgString = new XMLSerializer().serializeToString(clone);
    const exportTree = typeof window !== "undefined" ? window.__PHGO_EXPORT_TREE__ : null;
    if (typeof exportTree === "function") {
      try {
        await exportTree({ format, svgString, width, height, baseName: "phylotree" });
        return;
      } catch (error) {
        const reportError = typeof window !== "undefined" ? window.__PHGO_REPORT_EXPORT_ERROR__ : null;
        if (typeof reportError === "function") {
          reportError(error);
          return;
        }
        throw error;
      }
    }
    if (format === "svg") {
      const blob = new Blob([svgString], { type: "image/svg+xml;charset=utf-8" });
      triggerDownload(URL.createObjectURL(blob), "phylotree.svg");
      return;
    }
    const scale = 10;
    const canvas = document.createElement("canvas");
    canvas.width = width * scale;
    canvas.height = height * scale;
    const ctx = canvas.getContext("2d");
    ctx.scale(scale, scale);
    ctx.clearRect(0, 0, width, height);
    await new Promise((resolve) => {
      const img = new Image();
      const svgBlob = new Blob([svgString], { type: "image/svg+xml;charset=utf-8" });
      const url = URL.createObjectURL(svgBlob);
      img.onload = () => {
        ctx.drawImage(img, 0, 0);
        URL.revokeObjectURL(url);
        resolve();
      };
      img.onerror = () => {
        URL.revokeObjectURL(url);
        resolve();
      };
      img.src = url;
    });
    if (format === "png") {
      canvas.toBlob((blob) => {
        if (blob) triggerDownload(URL.createObjectURL(blob), "phylotree.png");
      }, "image/png");
      return;
    }
    const { jsPDF } = await import("jspdf");
    const mmW = width * 0.264583;
    const mmH = height * 0.264583;
    const doc = new jsPDF({
      orientation: width >= height ? "l" : "p",
      unit: "mm",
      format: [mmW, mmH]
    });
    doc.addImage(canvas.toDataURL("image/png"), "PNG", 0, 0, mmW, mmH);
    doc.save("phylotree.pdf");
  }, []);`;

const handleDownloadOriginalCJS = handleDownloadOriginal.replace(
  '  const handleDownload = useCallback(async (format) => {',
  '  const handleDownload = (0, import_react.useCallback)(async (format) => {',
);

const handleDownloadPatchedLegacyCJS = handleDownloadPatchedLegacy.replace(
  '  const handleDownload = useCallback(async (format) => {',
  '  const handleDownload = (0, import_react.useCallback)(async (format) => {',
);

const handleDownloadPatchedCJS = handleDownloadPatched.replace(
  '  const handleDownload = useCallback(async (format) => {',
  '  const handleDownload = (0, import_react.useCallback)(async (format) => {',
);

const labelModeGuardOriginalMJS = `  useEffect(() => {
    if (labelMode === "bootstrap" && !hasBootstraps) setLabelMode("none");
    if (labelMode === "branchlength" && !hasBranchLengths) setLabelMode("none");
  }, [hasBootstraps, hasBranchLengths, labelMode]);`;

const labelModeGuardPatchedMJS = `  useLayoutEffect(() => {
    const nextLabelMode = sanitizeLabelMode(labelMode, hasBootstraps, hasBranchLengths);
    if (nextLabelMode !== labelMode) setLabelMode(nextLabelMode);
  }, [hasBootstraps, hasBranchLengths, labelMode]);`;

const labelModeGuardOriginalCJS = `  (0, import_react.useEffect)(() => {
    if (labelMode === "bootstrap" && !hasBootstraps) setLabelMode("none");
    if (labelMode === "branchlength" && !hasBranchLengths) setLabelMode("none");
  }, [hasBootstraps, hasBranchLengths, labelMode]);`;

const labelModeGuardPatchedCJS = `  (0, import_react.useLayoutEffect)(() => {
    const nextLabelMode = sanitizeLabelMode(labelMode, hasBootstraps, hasBranchLengths);
    if (nextLabelMode !== labelMode) setLabelMode(nextLabelMode);
  }, [hasBootstraps, hasBranchLengths, labelMode]);`;

const truncateStateOriginalMJS = '  const [truncateNames, setTruncateNames] = useState(true);';
const truncateStateReplacementMJS = '  const [truncateNames, setTruncateNames] = useState(false);';

const truncateStateOriginalCJS = '  const [truncateNames, setTruncateNames] = (0, import_react.useState)(true);';
const truncateStateReplacementCJS = '  const [truncateNames, setTruncateNames] = (0, import_react.useState)(false);';
const labelModeInitialPatched = '  const [labelMode, setLabelMode] = useState(stringOr(initialSnapshot.labelMode, "bootstrap"));';
const labelModeInitialRootFixed = '  const [labelMode, setLabelMode] = useState(() => sanitizeLabelMode(stringOr(initialSnapshot.labelMode, "bootstrap"), treeHasBootstraps(treeData), treeHasBranchLengths(treeData)));';
const restoreTreeDataPatched = '    dispatch({ type: "RESET", data: snapshot.treeData ? cloneTreeData(snapshot.treeData) : parseNewick(newick) });';
const restoreTreeDataRootFixed = `    const restoredTreeData = snapshot.treeData ? cloneTreeData(snapshot.treeData) : parseNewick(newick);
    const restoredHasBootstraps = treeHasBootstraps(restoredTreeData);
    const restoredHasBranchLengths = treeHasBranchLengths(restoredTreeData);
    dispatch({ type: "RESET", data: restoredTreeData });`;
const restoreLabelModePatched = '    setLabelMode(stringOr(snapshot.labelMode, "bootstrap"));';
const restoreLabelModeRootFixed = '    setLabelMode(sanitizeLabelMode(stringOr(snapshot.labelMode, "bootstrap"), restoredHasBootstraps, restoredHasBranchLengths));';
const restoreEffectDepsPatched = '  }, [newick, initialState, defaultHeight]);';
const restoreEffectDepsRootFixed = '  }, [newick, initialState]);';
const viewportScaleStateAnchor = `  const [strokeWidth, setStrokeWidth] = useState(numberOr(initialSnapshot.strokeWidth, 1.5));
  const vScaleRef = useRef(1);`;
const viewportScaleStatePatched = `  const [strokeWidth, setStrokeWidth] = useState(numberOr(initialSnapshot.strokeWidth, 1.5));
  const [viewportScale, setViewportScale] = useState(numberOr(initialSnapshot.transform?.k, 1));
  const vScaleRef = useRef(1);`;
const viewportScaleRestoreAnchor = '    setStrokeWidth(numberOr(snapshot.strokeWidth, 1.5));';
const viewportScaleRestorePatched = `    setStrokeWidth(numberOr(snapshot.strokeWidth, 1.5));
    setViewportScale(numberOr(snapshot.transform?.k, 1));`;
const zoomEventAnchor = `      transformRef.current = ev.transform;
      content.attr("transform", ev.transform.toString());
      drawAlnCanvasRef.current();`;
const zoomEventPatched = `      transformRef.current = ev.transform;
      setViewportScale(ev.transform.k);
      content.attr("transform", ev.transform.toString());
      drawAlnCanvasRef.current();`;
const zoomEventStatePersistPrevious = `      transformRef.current = ev.transform;
      setViewportScale(ev.transform.k);
      scheduleStateChange();
      content.attr("transform", ev.transform.toString());
      drawAlnCanvasRef.current();`;
const zoomEventStatePersistPatched = `      transformRef.current = ev.transform;
      scheduleViewportScaleChange(ev.transform.k);
      scheduleStateChange();
      content.attr("transform", ev.transform.toString());
      scheduleAlnCanvasDraw();`;
const zoomFilterPatched = '    const zoom2 = d32.zoom().scaleExtent([0.05, 14]).filter((ev) => ev.type !== "dblclick").on("zoom", (ev) => {';
const zoomFilterRootFixed = '    const zoom2 = d32.zoom().scaleExtent([0.05, 14]).filter((ev) => ev.type !== "dblclick" && ev.type !== "wheel").on("zoom", (ev) => {';
const zoomWheelAnchor = `    svg.call(zoom2);
    zoomRef.current = zoom2;
    svg.on("click.nodeInfo", () => setNodeInfoRef.current(null));`;
const zoomWheelPrevious = `    svg.call(zoom2);
    zoomRef.current = zoom2;
    svg.on("wheel.phgoViewport", (event) => {
      event.preventDefault();
      const currentTransform = transformRef.current ?? d32.zoomIdentity;
      if (event.ctrlKey || event.metaKey) {
        const rect = svgRef.current?.getBoundingClientRect();
        if (!rect) return;
        const px = event.clientX - rect.left;
        const py = event.clientY - rect.top;
        const nextScale = Math.max(0.05, Math.min(14, currentTransform.k * Math.exp(-event.deltaY * 0.0025)));
        const worldX = (px - currentTransform.x) / currentTransform.k;
        const worldY = (py - currentTransform.y) / currentTransform.k;
        const nextTransform = d32.zoomIdentity.translate(px - worldX * nextScale, py - worldY * nextScale).scale(nextScale);
        svg.call(zoom2.transform, nextTransform);
        return;
      }
      const nextTransform = d32.zoomIdentity.translate(currentTransform.x - event.deltaX, currentTransform.y - event.deltaY).scale(currentTransform.k);
      svg.call(zoom2.transform, nextTransform);
    }, { passive: false });
    svg.on("click.nodeInfo", () => setNodeInfoRef.current(null));`;
const zoomWheelPatched = `    svg.call(zoom2);
    zoomRef.current = zoom2;
    svg.on("wheel.phgoViewport", (event) => {
      event.preventDefault();
      const currentTransform = transformRef.current ?? d32.zoomIdentity;
      if (event.ctrlKey || event.metaKey) {
        const rect = svgRef.current?.getBoundingClientRect();
        if (!rect) return;
        const px = event.clientX - rect.left;
        const py = event.clientY - rect.top;
        const unit = event.deltaMode === 1 ? 0.05 : event.deltaMode ? 1 : 0.002;
        const nextScale = Math.max(0.05, Math.min(14, currentTransform.k * Math.pow(2, -event.deltaY * unit * 10)));
        const worldX = (px - currentTransform.x) / currentTransform.k;
        const worldY = (py - currentTransform.y) / currentTransform.k;
        const nextTransform = d32.zoomIdentity.translate(px - worldX * nextScale, py - worldY * nextScale).scale(nextScale);
        svg.call(zoom2.transform, nextTransform);
        return;
      }
      const nextTransform = d32.zoomIdentity.translate(currentTransform.x - event.deltaX, currentTransform.y - event.deltaY).scale(currentTransform.k);
      svg.call(zoom2.transform, nextTransform);
    }, { passive: false });
    svg.on("click.nodeInfo", () => setNodeInfoRef.current(null));`;
const zoomScaleHandlerAnchor = `  const handleResizeDown = useCallback((e) => {
    const startY = e.clientY, startH = containerHRef.current;
    document.body.style.cursor = "ns-resize";
    document.body.style.userSelect = "none";
    const onMove = (ev) => setContainerH(Math.max(200, Math.min(1400, startH + ev.clientY - startY)));
    const onUp = () => {
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
    };
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
    e.preventDefault();
  }, []);
  const drawAlnCanvas = useCallback(() => {`;
const zoomScaleHandlerPatched = `  const handleResizeDown = useCallback((e) => {
    const startY = e.clientY, startH = containerHRef.current;
    document.body.style.cursor = "ns-resize";
    document.body.style.userSelect = "none";
    const onMove = (ev) => setContainerH(Math.max(200, Math.min(1400, startH + ev.clientY - startY)));
    const onUp = () => {
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
    };
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
    e.preventDefault();
  }, []);
  const handleViewportScaleChange = useCallback((nextScale) => {
    if (!svgRef.current || !zoomRef.current || !wrapperRef.current) return;
    const clampedScale = Math.max(0.05, Math.min(14, nextScale));
    const currentTransform = transformRef.current ?? d32.zoomIdentity;
    const wrapperRect = wrapperRef.current.getBoundingClientRect();
    const centerX = wrapperRect.width / 2;
    const centerY = wrapperRect.height / 2;
    const worldX = (centerX - currentTransform.x) / currentTransform.k;
    const worldY = (centerY - currentTransform.y) / currentTransform.k;
    const nextTransform = d32.zoomIdentity.translate(centerX - worldX * clampedScale, centerY - worldY * clampedScale).scale(clampedScale);
    d32.select(svgRef.current).call(zoomRef.current.transform, nextTransform);
  }, []);
  const drawAlnCanvas = useCallback(() => {`;
const alignmentWheelOriginal = `  useEffect(() => {
    const els = [alnScrollWrapRef.current, alnLegendRef.current];
    const handler = (e) => {
      const el = e.currentTarget;
      if (Math.abs(e.deltaX) > Math.abs(e.deltaY)) return;
      e.preventDefault();
      el.scrollLeft += e.deltaY;
    };
    els.forEach((el) => el?.addEventListener("wheel", handler, { passive: false }));
    return () => els.forEach((el) => el?.removeEventListener("wheel", handler));
  }, [showAlignment]);`;
const alignmentWheelPatched = alignmentWheelOriginal;
const zoomPctAnchor = `  const hPct = (hScale - 0.3) / 3.7 * 100;
  const vPct = (vScale - 0.3) / 3.7 * 100;
  const fPct = (fontScale - 0.5) / 2 * 100;
  const swPct = (strokeWidth - 0.5) / 3.5 * 100;`;
const zoomPctPatched = `  const hPct = (hScale - 0.3) / 3.7 * 100;
  const vPct = (vScale - 0.3) / 3.7 * 100;
  const fPct = (fontScale - 0.5) / 2 * 100;
  const swPct = (strokeWidth - 0.5) / 3.5 * 100;
  const zoomPct = (Math.max(0.05, Math.min(14, viewportScale)) - 0.05) / 13.95 * 100;`;
const zoomSliderAnchor = [
  '        )',
  '      ] }),',
  '      collapsedNodes.size > 0 && /* @__PURE__ */ jsxs2(Fragment, { children: [',
].join('\n');
const zoomSliderBlock = `      /* @__PURE__ */ jsx2("div", { className: Reactree_default.divider }),
      /* @__PURE__ */ jsxs2("div", { className: Reactree_default.sliderGroup, style: { minWidth: 172 }, children: [
        /* @__PURE__ */ jsx2("span", { className: Reactree_default.sliderIcon, children: /* @__PURE__ */ jsxs2("svg", { width: "14", height: "14", viewBox: "0 0 14 14", fill: "none", children: [
          /* @__PURE__ */ jsx2("circle", { cx: "6", cy: "6", r: "4", stroke: "currentColor", strokeWidth: "1.4" }),
          /* @__PURE__ */ jsx2("path", { d: "M9 9l3 3", stroke: "currentColor", strokeWidth: "1.4", strokeLinecap: "round" }),
          /* @__PURE__ */ jsx2("path", { d: "M6 4v4M4 6h4", stroke: "currentColor", strokeWidth: "1.2", strokeLinecap: "round" })
        ] }) }),
        /* @__PURE__ */ jsx2("span", { className: Reactree_default.sliderLabel, children: "Zoom" }),
        /* @__PURE__ */ jsx2(
          "input",
          {
            className: Reactree_default.slider,
            type: "range",
            min: 0.05,
            max: 14,
            step: 0.01,
            value: viewportScale,
            style: { background: \`linear-gradient(to right, var(--clr-primary-a0) \${zoomPct}%, var(--clr-surface-a30) \${zoomPct}%)\` },
            onChange: (e) => handleViewportScaleChange(Number(e.target.value)),
            onDoubleClick: () => handleViewportScaleChange(1)
          }
        ),
        /* @__PURE__ */ jsxs2("button", { className: Reactree_default.sliderVal, onClick: () => handleViewportScaleChange(1), children: [
          viewportScale >= 10 ? viewportScale.toFixed(1) : viewportScale.toFixed(2),
          "\xD7"
        ] })
      ] })`;
const fontPickerBlock = `      /* @__PURE__ */ jsxs2("div", { className: "phgo-fontPicker", children: [
        /* @__PURE__ */ jsx2("span", { className: "phgo-fontPickerLabel", children: "Font" }),
        /* @__PURE__ */ jsx2("select", { className: "phgo-fontSelect", value: fontFamily, onChange: (e) => setFontFamily(e.target.value), title: "Choose label font", children: fontOptions.map((family) => /* @__PURE__ */ jsx2("option", { value: family, children: family === DEFAULT_TREE_FONT_FAMILY ? "Default (system-ui)" : family }, family)) })
      ] })`;
const zoomSliderInsertBlock = `${fontPickerBlock},
${zoomSliderBlock},`;
const zoomSliderPatched = [
  '        )',
  '      ] }),',
  zoomSliderInsertBlock,
  '      collapsedNodes.size > 0 && /* @__PURE__ */ jsxs2(Fragment, { children: [',
].join('\n');
const panHintOriginal = '            /* @__PURE__ */ jsx2("div", { className: Reactree_default.hint, children: "Scroll to zoom \\xB7 Drag to pan" }),';
const panHintPatched = '            /* @__PURE__ */ jsx2("div", { className: Reactree_default.hint, children: "Scroll to pan \\xB7 Pinch to zoom \\xB7 Drag to pan" }),';
const snapshotStateEffectAnchor = `  }, [treeData, history, layout, treeType, labelMode, alignLabels, truncateNames, showAlignment, containerH, hScale, vScale, fontScale, strokeWidth, colorOverrides, collapsedNodes, collapseLabels, activeColor, colorMode, rerootMode, flipMode, swapMode, searchOpen, searchQuery, searchMatchIdx, downloadOpen, downloadMenuPos, palettePos, cladeEditor, nodeInfo, theme]);
  useEffect(() => {
    onStateChange?.(snapshotState());
  }, [onStateChange, snapshotState]);`;
const snapshotStateEffectPatched = `  }, [treeData, history, layout, treeType, labelMode, alignLabels, truncateNames, showAlignment, containerH, hScale, vScale, fontScale, strokeWidth, colorOverrides, collapsedNodes, collapseLabels, activeColor, colorMode, rerootMode, flipMode, swapMode, searchOpen, searchQuery, searchMatchIdx, downloadOpen, downloadMenuPos, palettePos, cladeEditor, nodeInfo, theme]);
  const snapshotStateRef = useRef(snapshotState);
  snapshotStateRef.current = snapshotState;
  const onStateChangeRef = useRef(onStateChange);
  onStateChangeRef.current = onStateChange;
  const pendingViewportScaleRef = useRef(viewportScale);
  const viewportScaleFrameRef = useRef(null);
  const scheduleViewportScaleChange = useCallback((nextScale) => {
    pendingViewportScaleRef.current = nextScale;
    if (viewportScaleFrameRef.current != null) return;
    viewportScaleFrameRef.current = requestAnimationFrame(() => {
      viewportScaleFrameRef.current = null;
      const pendingScale = pendingViewportScaleRef.current;
      setViewportScale((currentScale) => Math.abs(currentScale - pendingScale) < 1e-4 ? currentScale : pendingScale);
    });
  }, []);
  const alnDrawFrameRef = useRef(null);
  const scheduleAlnCanvasDraw = useCallback(() => {
    if (alnDrawFrameRef.current != null) return;
    alnDrawFrameRef.current = requestAnimationFrame(() => {
      alnDrawFrameRef.current = null;
      drawAlnCanvasRef.current();
    });
  }, []);
  const stateChangeFrameRef = useRef(null);
  const scheduleStateChange = useCallback(() => {
    if (!onStateChangeRef.current) return;
    if (stateChangeFrameRef.current != null) cancelAnimationFrame(stateChangeFrameRef.current);
    stateChangeFrameRef.current = requestAnimationFrame(() => {
      stateChangeFrameRef.current = null;
      onStateChangeRef.current?.(snapshotStateRef.current());
    });
  }, []);
  useEffect(() => {
    return () => {
      if (viewportScaleFrameRef.current != null) cancelAnimationFrame(viewportScaleFrameRef.current);
      if (alnDrawFrameRef.current != null) cancelAnimationFrame(alnDrawFrameRef.current);
      if (stateChangeFrameRef.current != null) cancelAnimationFrame(stateChangeFrameRef.current);
    };
  }, []);
  useEffect(() => {
    onStateChange?.(snapshotState());
  }, [onStateChange, snapshotState]);`;
const snapshotStateBridgePrevious = `  const snapshotStateRef = useRef(snapshotState);
  snapshotStateRef.current = snapshotState;
  const onStateChangeRef = useRef(onStateChange);
  onStateChangeRef.current = onStateChange;
  const stateChangeFrameRef = useRef(null);
  const scheduleStateChange = useCallback(() => {
    if (!onStateChangeRef.current) return;
    if (stateChangeFrameRef.current != null) cancelAnimationFrame(stateChangeFrameRef.current);
    stateChangeFrameRef.current = requestAnimationFrame(() => {
      stateChangeFrameRef.current = null;
      onStateChangeRef.current?.(snapshotStateRef.current());
    });
  }, []);
  useEffect(() => {
    return () => {
      if (stateChangeFrameRef.current != null) cancelAnimationFrame(stateChangeFrameRef.current);
    };
  }, []);
  useEffect(() => {
    onStateChange?.(snapshotState());
  }, [onStateChange, snapshotState]);`;
const snapshotStateBridgePatched = `  const snapshotStateRef = useRef(snapshotState);
  snapshotStateRef.current = snapshotState;
  const onStateChangeRef = useRef(onStateChange);
  onStateChangeRef.current = onStateChange;
  const pendingViewportScaleRef = useRef(viewportScale);
  const viewportScaleFrameRef = useRef(null);
  const scheduleViewportScaleChange = useCallback((nextScale) => {
    pendingViewportScaleRef.current = nextScale;
    if (viewportScaleFrameRef.current != null) return;
    viewportScaleFrameRef.current = requestAnimationFrame(() => {
      viewportScaleFrameRef.current = null;
      const pendingScale = pendingViewportScaleRef.current;
      setViewportScale((currentScale) => Math.abs(currentScale - pendingScale) < 1e-4 ? currentScale : pendingScale);
    });
  }, []);
  const alnDrawFrameRef = useRef(null);
  const scheduleAlnCanvasDraw = useCallback(() => {
    if (alnDrawFrameRef.current != null) return;
    alnDrawFrameRef.current = requestAnimationFrame(() => {
      alnDrawFrameRef.current = null;
      drawAlnCanvasRef.current();
    });
  }, []);
  const stateChangeFrameRef = useRef(null);
  const scheduleStateChange = useCallback(() => {
    if (!onStateChangeRef.current) return;
    if (stateChangeFrameRef.current != null) cancelAnimationFrame(stateChangeFrameRef.current);
    stateChangeFrameRef.current = requestAnimationFrame(() => {
      stateChangeFrameRef.current = null;
      onStateChangeRef.current?.(snapshotStateRef.current());
    });
  }, []);
  useEffect(() => {
    return () => {
      if (viewportScaleFrameRef.current != null) cancelAnimationFrame(viewportScaleFrameRef.current);
      if (alnDrawFrameRef.current != null) cancelAnimationFrame(alnDrawFrameRef.current);
      if (stateChangeFrameRef.current != null) cancelAnimationFrame(stateChangeFrameRef.current);
    };
  }, []);
  useEffect(() => {
    onStateChange?.(snapshotState());
  }, [onStateChange, snapshotState]);`;

const alignmentTruncateOriginal = `            let text = displayTreeName(name);
            while (lc.measureText(text).width > maxW && text.length > 3)
              text = text.slice(0, -4) + "\\u2026";
            lc.fillText(text, NAME_X, top + cellH / 2 + 0.5);`;

const alignmentTruncateReplacement = `            const text = displayTreeName(name);
            lc.fillText(text, NAME_X, top + cellH / 2 + 0.5);`;

const treeLabelTruncateOriginal = `        const n = displayTreeName(d.data.name);
        return truncateNames && n.length > 30 ? n.slice(0, 28) + "\\u2026" : n;`;

const treeLabelTruncateReplacement = `        const n = displayTreeName(d.data.name);
        return n;`;

const truncateButtonOriginalCJS = `      hasTruncatableNames && /* @__PURE__ */ (0, import_jsx_runtime2.jsxs)(import_jsx_runtime2.Fragment, { children: [
        /* @__PURE__ */ (0, import_jsx_runtime2.jsx)("div", { className: Reactree_default.divider }),
        /* @__PURE__ */ (0, import_jsx_runtime2.jsxs)(
          "button",
          {
            className: \`\${Reactree_default.alignLabelsBtn} \${truncateNames ? Reactree_default.alignLabelsBtnActive : ""}\`,
            onClick: () => setTruncateNames((t) => !t),
            title: truncateNames ? "Names truncated \\u2014 click to show full names" : "Full names shown \\u2014 click to truncate",
            children: [
              /* @__PURE__ */ (0, import_jsx_runtime2.jsxs)("svg", { width: "13", height: "13", viewBox: "0 0 14 14", fill: "none", children: [
                /* @__PURE__ */ (0, import_jsx_runtime2.jsx)("path", { d: "M1 3.5h12", stroke: "currentColor", strokeWidth: "1.4", strokeLinecap: "round" }),
                /* @__PURE__ */ (0, import_jsx_runtime2.jsx)("path", { d: "M1 7h7", stroke: "currentColor", strokeWidth: "1.4", strokeLinecap: "round" }),
                /* @__PURE__ */ (0, import_jsx_runtime2.jsx)("circle", { cx: "9.5", cy: "7", r: "0.8", fill: "currentColor" }),
                /* @__PURE__ */ (0, import_jsx_runtime2.jsx)("circle", { cx: "11.5", cy: "7", r: "0.8", fill: "currentColor" }),
                /* @__PURE__ */ (0, import_jsx_runtime2.jsx)("circle", { cx: "13.5", cy: "7", r: "0.8", fill: "currentColor" }),
                /* @__PURE__ */ (0, import_jsx_runtime2.jsx)("path", { d: "M1 10.5h12", stroke: "currentColor", strokeWidth: "1.4", strokeLinecap: "round" })
              ] }),
              truncateNames ? "Truncate" : "Full names"
            ]
          }
        )
      ] }),`;

const truncateButtonOriginalMJS = `      hasTruncatableNames && /* @__PURE__ */ jsxs2(Fragment, { children: [
        /* @__PURE__ */ jsx2("div", { className: Reactree_default.divider }),
        /* @__PURE__ */ jsxs2(
          "button",
          {
            className: \`\${Reactree_default.alignLabelsBtn} \${truncateNames ? Reactree_default.alignLabelsBtnActive : ""}\`,
            onClick: () => setTruncateNames((t) => !t),
            title: truncateNames ? "Names truncated \\u2014 click to show full names" : "Full names shown \\u2014 click to truncate",
            children: [
              /* @__PURE__ */ jsxs2("svg", { width: "13", height: "13", viewBox: "0 0 14 14", fill: "none", children: [
                /* @__PURE__ */ jsx2("path", { d: "M1 3.5h12", stroke: "currentColor", strokeWidth: "1.4", strokeLinecap: "round" }),
                /* @__PURE__ */ jsx2("path", { d: "M1 7h7", stroke: "currentColor", strokeWidth: "1.4", strokeLinecap: "round" }),
                /* @__PURE__ */ jsx2("circle", { cx: "9.5", cy: "7", r: "0.8", fill: "currentColor" }),
                /* @__PURE__ */ jsx2("circle", { cx: "11.5", cy: "7", r: "0.8", fill: "currentColor" }),
                /* @__PURE__ */ jsx2("circle", { cx: "13.5", cy: "7", r: "0.8", fill: "currentColor" }),
                /* @__PURE__ */ jsx2("path", { d: "M1 10.5h12", stroke: "currentColor", strokeWidth: "1.4", strokeLinecap: "round" })
              ] }),
              truncateNames ? "Truncate" : "Full names"
            ]
          }
        )
      ] }),`;

function replaceRequired(text, original, replacement, description, file) {
  if (text.includes(replacement)) {
    console.log(`${description} already present: ${file}`);
    return text;
  }
  if (!text.includes(original)) {
    throw new Error(`${description} anchor not found in ${file}`);
  }
  console.log(`Patched ${description}: ${file}`);
  return text.replace(original, replacement);
}

function replaceAllRequired(text, original, replacement, description, file) {
  if (
    description === 'Reactree full-name default' &&
    text.includes('initialSnapshot.truncateNames')
  ) {
    console.log(`${description} already present: ${file}`);
    return text;
  }
  if (description === 'Reactree viewport-scale restore') {
    const single = '    setViewportScale(numberOr(snapshot.transform?.k, 1));';
    const doubled = `${single}\n${single}`;
    if (text.includes(doubled)) {
      console.log(`${description} normalized duplicate: ${file}`);
      return text.replace(doubled, single);
    }
    if (text.includes(single)) {
      console.log(`${description} already present: ${file}`);
      return text;
    }
  }
  if (description === 'Reactree viewport zoom percent') {
    const single = '  const zoomPct = (Math.max(0.05, Math.min(14, viewportScale)) - 0.05) / 13.95 * 100;';
    const doubled = `${single}\n${single}`;
    if (text.includes(doubled)) {
      console.log(`${description} normalized duplicate: ${file}`);
      return text.replace(doubled, single);
    }
    if (text.includes(single)) {
      console.log(`${description} already present: ${file}`);
      return text;
    }
  }
  if (description === 'Reactree zoom state sync') {
    if (text.includes(zoomEventStatePersistPatched) || text.includes(zoomEventStatePersistPrevious) || text.includes(zoomEventPatched)) {
      console.log(`${description} already present: ${file}`);
      return text;
    }
  }
  if (description === 'Reactree transform persistence') {
    if (text.includes(zoomEventStatePersistPatched)) {
      console.log(`${description} already present: ${file}`);
      return text;
    }
    if (text.includes(zoomEventStatePersistPrevious)) {
      console.log(`${description} upgraded old transform persistence: ${file}`);
      return text.split(zoomEventStatePersistPrevious).join(zoomEventStatePersistPatched);
    }
  }
  if (description === 'Reactree transform state bridge') {
    if (text.includes('const scheduleViewportScaleChange = useCallback((nextScale) => {')) {
      console.log(`${description} already present: ${file}`);
      return text;
    }
    if (text.includes(snapshotStateBridgePrevious)) {
      console.log(`${description} upgraded old bridge: ${file}`);
      return text.replace(snapshotStateBridgePrevious, snapshotStateBridgePatched);
    }
    if (text.includes(snapshotStateEffectAnchor)) {
      console.log(`${description} upgraded old bridge: ${file}`);
      return text.replace(snapshotStateEffectAnchor, snapshotStateEffectPatched);
    }
  }
  if (description === 'Reactree trackpad pan handler') {
    if (text.includes(zoomWheelPatched)) {
      console.log(`${description} already present: ${file}`);
      return text;
    }
    if (text.includes(zoomWheelPrevious)) {
      console.log(`${description} upgraded old wheel handler: ${file}`);
      return text.split(zoomWheelPrevious).join(zoomWheelPatched);
    }
  }
  if (description === 'Reactree viewport zoom slider') {
    const existingCount = text.split(zoomSliderBlock).length - 1;
    if (text.includes(zoomSliderInsertBlock)) {
      text = text.split(zoomSliderInsertBlock).join('');
    }
    if (existingCount > 0) {
      text = text.split(zoomSliderBlock).join('');
      text = text.replace(/^\s*,\s*$/gm, '');
      text = text.replace(/\n{3,}/g, '\n\n');
      console.log(
        existingCount > 1
          ? `${description} normalized duplicate: ${file}`
          : `${description} repositioning existing block: ${file}`,
      );
    }
    const collapsedAnchor = /\n(\s*collapsedNodes\.size > 0 && \/\* @__PURE__ \*\/ jsxs2\(Fragment, \{ children: \[)/;
    if (!collapsedAnchor.test(text)) {
      throw new Error(`${description} anchor not found in ${file}`);
    }
    console.log(`Patched ${description}: ${file}`);
    return text.replace(collapsedAnchor, `\n${zoomSliderInsertBlock}\n$1`);
  }
  if (!text.includes(original)) {
    if (text.includes(replacement)) {
      console.log(`${description} already present: ${file}`);
      return text;
    }
    throw new Error(`${description} anchor not found in ${file}`);
  }
  console.log(`Patched ${description}: ${file}`);
  return text.split(original).join(replacement);
}

for (const file of files) {
  let text = readFileSync(file, 'utf8').replace(/\r\n/g, '\n');
  if (!text.includes('PHGO_DPR')) {
    if (!text.includes(dprAnchor) || !text.includes(labelOriginal) || !text.includes(seqOriginal)) {
      throw new Error(`reactreejs alignment canvas patch anchors not found in ${file}`);
    }
    text = text
      .replace(dprAnchor, dprPatch)
      .replace(labelOriginal, labelPatched)
      .replace(seqOriginal, seqPatched);
    writeFileSync(file, text);
    console.log(`Patched Reactree alignment canvas DPI scaling: ${file}`);
  } else {
    console.log(`Reactree alignment canvas DPI patch already present: ${file}`);
  }

  text = readFileSync(file, 'utf8').replace(/\r\n/g, '\n');
  const heightPatchNeedles = [
    'useEffect(() => {\n    setContainerH(defaultHeight);\n  }, [defaultHeight]);',
    '(0, import_react.useEffect)(() => {\n    setContainerH(defaultHeight);\n  }, [defaultHeight]);',
    'useEffect(() => {\n    if (initialSnapshot.containerH == null) setContainerH(defaultHeight);\n  }, [defaultHeight, initialSnapshot.containerH]);',
    '(0, import_react.useEffect)(() => {\n    if (initialSnapshot.containerH == null) setContainerH(defaultHeight);\n  }, [defaultHeight, initialSnapshot.containerH]);'
  ];
  if (!heightPatchNeedles.some((needle) => text.includes(needle))) {
    const heightAnchors = [
      '  const [containerH, setContainerH] = useState(defaultHeight);',
      '  const [containerH, setContainerH] = (0, import_react.useState)(defaultHeight);'
    ];
    const heightAnchor = heightAnchors.find((anchor) => text.includes(anchor));
    if (!heightAnchor) {
      throw new Error(`reactreejs height sync anchor not found in ${file}`);
    }
    const heightPatch = `${heightAnchor}\n  useEffect(() => {\n    setContainerH(defaultHeight);\n  }, [defaultHeight]);`;
    const heightPatchCjs = `${heightAnchor}\n  (0, import_react.useEffect)(() => {\n    setContainerH(defaultHeight);\n  }, [defaultHeight]);`;
    text = text.replace(heightAnchor, file.endsWith('.js') ? heightPatchCjs : heightPatch);
    writeFileSync(file, text);
    console.log(`Patched Reactree defaultHeight sync: ${file}`);
  } else {
    console.log(`Reactree defaultHeight sync already present: ${file}`);
  }

  text = readFileSync(file, 'utf8');
  text = replaceRequired(text, parseOriginal, parsePatched, 'Reactree quoted Newick parser', file);
  text = replaceRequired(text, toNewickOriginal, toNewickPatched, 'Reactree quoted Newick exporter', file);
  if (text.includes('function displayTreeName(name)')) {
    console.log(`Reactree display-name helper already present: ${file}`);
  } else {
    text = replaceRequired(text, displayNameAnchor, displayNamePatched, 'Reactree display-name helper', file);
  }
  text = replaceAllRequired(text, triggerDownloadPatched, triggerDownloadBridgePatched, 'Reactree save-picker bridge', file);
  const isCJS = file.endsWith('.js');
  text = replaceAllRequired(
    text,
    isCJS ? labelModeGuardOriginalCJS : labelModeGuardOriginalMJS,
    isCJS ? labelModeGuardPatchedCJS : labelModeGuardPatchedMJS,
    'Reactree label-mode guard',
    file,
  );
  text = replaceAllRequired(
    text,
    isCJS
      ? '(0, import_react.useEffect)(() => {\n    if (initialSnapshot.containerH == null) setContainerH(defaultHeight);\n  }, [defaultHeight, initialSnapshot.containerH]);'
      : 'useEffect(() => {\n    if (initialSnapshot.containerH == null) setContainerH(defaultHeight);\n  }, [defaultHeight, initialSnapshot.containerH]);',
    isCJS
      ? '(0, import_react.useEffect)(() => {\n    setContainerH(defaultHeight);\n  }, [defaultHeight]);'
      : 'useEffect(() => {\n    setContainerH(defaultHeight);\n  }, [defaultHeight]);',
    'Reactree viewport height sync',
    file,
  );
  text = replaceAllRequired(
    text,
    isCJS ? truncateStateOriginalCJS : truncateStateOriginalMJS,
    isCJS ? truncateStateReplacementCJS : truncateStateReplacementMJS,
    'Reactree full-name default',
    file,
  );
  text = replaceRequired(text, alignmentTruncateOriginal, alignmentTruncateReplacement, 'Reactree full alignment labels', file);
  text = replaceRequired(text, treeLabelTruncateOriginal, treeLabelTruncateReplacement, 'Reactree full tree labels', file);
  text = replaceAllRequired(text, isCJS ? truncateButtonOriginalCJS : truncateButtonOriginalMJS, '', 'Reactree truncate toggle removal', file);
  text = replaceAllRequired(text, '(d.data.name || "").replace(/_/g, " ")', 'displayTreeName(d.data.name)', 'Reactree display label preservation', file);
  text = replaceAllRequired(text, '(d.target.data.name || "").replace(/_/g, " ")', 'displayTreeName(d.target.data.name)', 'Reactree target label preservation', file);
  text = replaceAllRequired(text, '(node.name || "").replace(/_/g, " ")', 'displayTreeName(node.name)', 'Reactree search label preservation', file);
  const exportBridgeOriginal = isCJS ? handleDownloadOriginalCJS : handleDownloadOriginal;
  const exportBridgeLegacy = isCJS ? handleDownloadPatchedLegacyCJS : handleDownloadPatchedLegacy;
  const exportBridgePatched = isCJS ? handleDownloadPatchedCJS : handleDownloadPatched;
  if (text.includes(exportBridgePatched)) {
    console.log(`Reactree export bridge already present: ${file}`);
  } else if (text.includes(exportBridgeLegacy)) {
    console.log(`Patched Reactree export bridge: ${file}`);
    text = text.split(exportBridgeLegacy).join(exportBridgePatched);
  } else {
    text = replaceAllRequired(
      text,
      exportBridgeOriginal,
      exportBridgePatched,
      'Reactree export bridge',
      file,
    );
  }
  text = replaceAllRequired(
    text,
    labelModeInitialPatched,
    labelModeInitialRootFixed,
    'Reactree label-mode root init',
    file,
  );
  text = replaceAllRequired(
    text,
    restoreTreeDataPatched,
    restoreTreeDataRootFixed,
    'Reactree restore tree-data root fix',
    file,
  );
  text = replaceAllRequired(
    text,
    restoreLabelModePatched,
    restoreLabelModeRootFixed,
    'Reactree restore label-mode root fix',
    file,
  );
  text = replaceAllRequired(
    text,
    restoreEffectDepsPatched,
    restoreEffectDepsRootFixed,
    'Reactree restore deps resize fix',
    file,
  );
  text = replaceAllRequired(
    text,
    viewportScaleStateAnchor,
    viewportScaleStatePatched,
    'Reactree viewport-scale state',
    file,
  );
  text = replaceAllRequired(
    text,
    viewportScaleRestoreAnchor,
    viewportScaleRestorePatched,
    'Reactree viewport-scale restore',
    file,
  );
  text = replaceAllRequired(
    text,
    zoomEventAnchor,
    zoomEventPatched,
    'Reactree zoom state sync',
    file,
  );
  text = replaceAllRequired(
    text,
    zoomEventPatched,
    zoomEventStatePersistPatched,
    'Reactree transform persistence',
    file,
  );
  text = replaceAllRequired(
    text,
    zoomFilterPatched,
    zoomFilterRootFixed,
    'Reactree wheel filter override',
    file,
  );
  text = replaceAllRequired(
    text,
    zoomWheelAnchor,
    zoomWheelPatched,
    'Reactree trackpad pan handler',
    file,
  );
  text = replaceAllRequired(
    text,
    zoomScaleHandlerAnchor,
    zoomScaleHandlerPatched,
    'Reactree viewport slider handler',
    file,
  );
  text = replaceAllRequired(
    text,
    zoomPctAnchor,
    zoomPctPatched,
    'Reactree viewport zoom percent',
    file,
  );
  text = replaceAllRequired(
    text,
    zoomSliderAnchor,
    zoomSliderPatched,
    'Reactree viewport zoom slider',
    file,
  );
  text = replaceAllRequired(
    text,
    panHintOriginal,
    panHintPatched,
    'Reactree pan-zoom hint copy',
    file,
  );
  text = replaceAllRequired(
    text,
    snapshotStateEffectAnchor,
    snapshotStateEffectPatched,
    'Reactree transform state bridge',
    file,
  );
  for (const [original, replacement] of [
    ['    const scrollWrap = alnScrollWrapRef.current;\n    const visibleStartCol = scrollWrap ? Math.max(0, Math.floor(scrollWrap.scrollLeft / CELL_W) - 2) : 0;\n    const visibleEndCol = scrollWrap ? Math.min(maxLen, Math.ceil((scrollWrap.scrollLeft + scrollWrap.clientWidth) / CELL_W) + 2) : maxLen;\n    const paintX = visibleStartCol * CELL_W;\n    const paintW = Math.max(CELL_W, (visibleEndCol - visibleStartCol) * CELL_W);\n', ''],
    ['    sc.fillRect(paintX, 0, paintW, H);', '    sc.fillRect(0, 0, totalW, H);'],
    ['        sc.fillRect(paintX, top, paintW, cellH);', '        sc.fillRect(0, top, totalW, cellH);'],
    ['        sc.fillRect(paintX, top + cellH - 0.5, paintW, 0.5);', '        sc.fillRect(0, top + cellH - 0.5, totalW, 0.5);'],
    ['      for (let i = visibleStartCol; i < Math.min(seq.length, visibleEndCol); i++) {', '      for (let i = 0; i < seq.length; i++) {'],
    ['    sc.fillRect(paintX, 0, paintW, AXIS_H);', '    sc.fillRect(0, 0, totalW, AXIS_H);'],
    ['    sc.fillRect(paintX, AXIS_H - 1, paintW, 1);', '    sc.fillRect(0, AXIS_H - 1, totalW, 1);'],
    ['    for (let i = visibleStartCol; i < visibleEndCol; i++) {', '    for (let i = 0; i < maxLen; i++) {'],
    ['.transition().duration(130)', '.transition().duration(0)'],
    ['.transition().duration(120)', '.transition().duration(0)'],
    ['.transition().duration(280)', '.transition().duration(0)'],
    ['.transition().duration(380)', '.transition().duration(0)'],
    ['.transition().duration(350)', '.transition().duration(0)'],
    ['.transition().duration(isLayoutChange || isFirstRender ? 320 : 200)', '.transition().duration(0)'],
  ]) {
    text = replaceAllRequired(text, original, replacement, 'Reactree D3 animation removal', file);
  }
  writeFileSync(file, text);
}

const viewerStateHelpers = `function cloneTreeData(node) {
  if (!node || typeof node !== "object") return node;
  const out = { name: String(node.name ?? "") };
  if (node.length != null && !isNaN(node.length)) out.length = Number(node.length);
  if (Array.isArray(node.children)) out.children = node.children.map(cloneTreeData);
  return out;
}
function mapToEntries(map) {
  return Array.from(map.entries()).map(([key, value]) => [key, value]);
}
function entriesToMap(entries) {
  return new Map(Array.isArray(entries) ? entries : []);
}
function setToValues(set) {
  return Array.from(set.values());
}
function valuesToSet(values) {
  return new Set(Array.isArray(values) ? values : []);
}
function numberOr(value, fallback) {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}
function boolOr(value, fallback) {
  return typeof value === "boolean" ? value : fallback;
}
function stringOr(value, fallback) {
  return typeof value === "string" ? value : fallback;
}
function transformToSnapshot(transform) {
  if (!transform) return null;
  return { x: numberOr(transform.x, 0), y: numberOr(transform.y, 0), k: numberOr(transform.k, 1) };
}
function transformFromSnapshot(transform) {
  if (!transform || typeof transform !== "object") return null;
  return d32.zoomIdentity.translate(numberOr(transform.x, 0), numberOr(transform.y, 0)).scale(numberOr(transform.k, 1));
}
function normalizeReactreeStateSnapshot(state) {
  return state && typeof state === "object" ? state : {};
}
function sanitizeLabelMode(labelMode, hasBootstraps, hasBranchLengths) {
  if (labelMode === "bootstrap" && hasBootstraps) return "bootstrap";
  if (labelMode === "branchlength" && hasBranchLengths) return "branchlength";
  if (labelMode === "none") return "none";
  return "none";
}`;

const viewerStateMenu = `            /* @__PURE__ */ jsx2("div", { className: Reactree_default.downloadMenuDivider }),
            /* @__PURE__ */ jsxs2(
              "button",
              {
                className: Reactree_default.downloadMenuItem,
                onClick: () => {
                  onViewerSnapshot?.(snapshotState());
                  setDownloadOpen(false);
                },
                children: [
                  /* @__PURE__ */ jsx2("span", { className: Reactree_default.downloadMenuIcon, children: /* @__PURE__ */ jsxs2("svg", { width: "12", height: "12", viewBox: "0 0 12 12", fill: "none", children: [
                    /* @__PURE__ */ jsx2("rect", { x: "1.2", y: "1.2", width: "9.6", height: "9.6", rx: "1.8", stroke: "currentColor", strokeWidth: "1.3" }),
                    /* @__PURE__ */ jsx2("path", { d: "M3.4 4h5.2M3.4 6h5.2M3.4 8h3.4", stroke: "currentColor", strokeWidth: "1.1", strokeLinecap: "round" })
                  ] }) }),
                  /* @__PURE__ */ jsx2("span", { className: Reactree_default.downloadMenuLabel, children: "PHgo Viewer Snapshot" }),
                  /* @__PURE__ */ jsx2("span", { className: Reactree_default.downloadMenuDesc, children: "Full state" })
                ]
              }
            ),`;

function patchReactreeViewerStateMJS(file) {
  let text = readFileSync(file, 'utf8').replace(/\r\n/g, '\n');
  if (text.includes('onViewerSnapshot') && text.includes('snapshotState') && !text.includes('function sanitizeLabelMode(')) {
    text = replaceRequired(
      text,
      `function normalizeReactreeStateSnapshot(state) {
  return state && typeof state === "object" ? state : {};
}`,
      `function normalizeReactreeStateSnapshot(state) {
  return state && typeof state === "object" ? state : {};
}
function sanitizeLabelMode(labelMode, hasBootstraps, hasBranchLengths) {
  if (labelMode === "bootstrap" && hasBootstraps) return "bootstrap";
  if (labelMode === "branchlength" && hasBranchLengths) return "branchlength";
  if (labelMode === "none") return "none";
  return "none";
}`,
      'Reactree sanitize-label helper',
      file,
    );
    writeFileSync(file, text);
    console.log(`Patched Reactree sanitize-label helper: ${file}`);
  }
  if (text.includes('onViewerSnapshot') && text.includes('snapshotState')) {
    console.log(`Reactree viewer-state bridge already present: ${file}`);
    return;
  }
  text = replaceRequired(text, displayNamePatched, `${displayNamePatched}
${viewerStateHelpers}`, 'Reactree viewer-state helpers', file);
  text = replaceRequired(
    text,
    'function Reactree({ newick, defaultHeight = 520, fasta }) {',
    'function Reactree({ newick, defaultHeight = 520, fasta, initialState, onStateChange, onViewerSnapshot }) {',
    'Reactree viewer-state props',
    file,
  );
  text = replaceRequired(
    text,
    `  const [{ treeData, history }, dispatch] = useReducer(
    treeReducer,
    null,
    () => ({ treeData: parseNewick(newick), history: [] })
  );`,
    `  const initialSnapshot = normalizeReactreeStateSnapshot(initialState);
  const [{ treeData, history }, dispatch] = useReducer(
    treeReducer,
    null,
    () => ({ treeData: initialSnapshot.treeData ? cloneTreeData(initialSnapshot.treeData) : parseNewick(newick), history: Array.isArray(initialSnapshot.history) ? initialSnapshot.history.map(cloneTreeData) : [] })
  );`,
    'Reactree viewer-state reducer seed',
    file,
  );
  const replacements = [
    ['  const [rerootMode, setRerootMode] = useState(false);', '  const [rerootMode, setRerootMode] = useState(boolOr(initialSnapshot.rerootMode, false));'],
    ['  const [flipMode, setFlipMode] = useState(false);', '  const [flipMode, setFlipMode] = useState(boolOr(initialSnapshot.flipMode, false));'],
    ['  const [swapMode, setSwapMode] = useState(false);', '  const [swapMode, setSwapMode] = useState(boolOr(initialSnapshot.swapMode, false));'],
    ['  const [downloadOpen, setDownloadOpen] = useState(false);', '  const [downloadOpen, setDownloadOpen] = useState(boolOr(initialSnapshot.downloadOpen, false));'],
    ['  const [downloadMenuPos, setDownloadMenuPos] = useState(null);', '  const [downloadMenuPos, setDownloadMenuPos] = useState(initialSnapshot.downloadMenuPos ?? null);'],
    ['  const [colorMode, setColorMode] = useState(false);', '  const [colorMode, setColorMode] = useState(boolOr(initialSnapshot.colorMode, false));'],
    ['  const [activeColor, setActiveColor] = useState(PALETTE[0].hex);', '  const [activeColor, setActiveColor] = useState(stringOr(initialSnapshot.activeColor, PALETTE[0].hex));'],
    ['  const [colorOverrides, setColorOverrides] = useState(/* @__PURE__ */ new Map());', '  const [colorOverrides, setColorOverrides] = useState(entriesToMap(initialSnapshot.colorOverrides));'],
    ['  const [palettePos, setPalettePos] = useState(null);', '  const [palettePos, setPalettePos] = useState(initialSnapshot.palettePos ?? null);'],
    ['  const [layout, setLayout] = useState("rectangular");', '  const [layout, setLayout] = useState(stringOr(initialSnapshot.layout, "rectangular"));'],
    ['  const [treeType, setTreeType] = useState("phylogram");', '  const [treeType, setTreeType] = useState(stringOr(initialSnapshot.treeType, "phylogram"));'],
    ['  const [labelMode, setLabelMode] = useState("bootstrap");', '  const [labelMode, setLabelMode] = useState(stringOr(initialSnapshot.labelMode, "bootstrap"));'],
    ['  const [alignLabels, setAlignLabels] = useState(false);', '  const [alignLabels, setAlignLabels] = useState(boolOr(initialSnapshot.alignLabels, false));'],
    ['  const [truncateNames, setTruncateNames] = useState(false);', '  const [truncateNames, setTruncateNames] = useState(boolOr(initialSnapshot.truncateNames, false));'],
    ['  const [showAlignment, setShowAlignment] = useState(false);', '  const [showAlignment, setShowAlignment] = useState(boolOr(initialSnapshot.showAlignment, false));'],
    ['  const [collapsedNodes, setCollapsedNodes] = useState(/* @__PURE__ */ new Set());', '  const [collapsedNodes, setCollapsedNodes] = useState(valuesToSet(initialSnapshot.collapsedNodes));'],
    ['  const [collapseLabels, setCollapseLabels] = useState(/* @__PURE__ */ new Map());', '  const [collapseLabels, setCollapseLabels] = useState(entriesToMap(initialSnapshot.collapseLabels));'],
    ['  const [searchOpen, setSearchOpen] = useState(false);', '  const [searchOpen, setSearchOpen] = useState(boolOr(initialSnapshot.searchOpen, false));'],
    ['  const [searchQuery, setSearchQuery] = useState("");', '  const [searchQuery, setSearchQuery] = useState(stringOr(initialSnapshot.searchQuery, ""));'],
    ['  const [searchMatchIdx, setSearchMatchIdx] = useState(0);', '  const [searchMatchIdx, setSearchMatchIdx] = useState(numberOr(initialSnapshot.searchMatchIdx, 0));'],
    ['  const [containerH, setContainerH] = useState(defaultHeight);', '  const [containerH, setContainerH] = useState(numberOr(initialSnapshot.containerH, defaultHeight));'],
    ['  const [hScale, setHScale] = useState(1);', '  const [hScale, setHScale] = useState(numberOr(initialSnapshot.hScale, 1));'],
    ['  const [vScale, setVScale] = useState(1);', '  const [vScale, setVScale] = useState(numberOr(initialSnapshot.vScale, 1));'],
    ['  const [fontScale, setFontScale] = useState(1);', '  const [fontScale, setFontScale] = useState(numberOr(initialSnapshot.fontScale, 1));'],
    ['  const [strokeWidth, setStrokeWidth] = useState(1.5);', '  const [strokeWidth, setStrokeWidth] = useState(numberOr(initialSnapshot.strokeWidth, 1.5));'],
    ['      if (isLayoutChange || isFirstRender) {', '      if ((isLayoutChange || isFirstRender) && !transformRef.current) {'],
  ];
  for (const [original, replacement] of replacements) {
    text = replaceAllRequired(text, original, replacement, 'Reactree viewer-state field', file);
  }
  text = replaceRequired(
    text,
    `  const containerHRef = useRef(defaultHeight);
  containerHRef.current = containerH;
  useEffect(() => {
    dispatch({ type: "RESET", data: parseNewick(newick) });
    setRerootMode(false);
    setFlipMode(false);
    setSwapMode(false);
    setSwapFirst(null);
    setColorMode(false);
    setColorOverrides(/* @__PURE__ */ new Map());
    setCollapsedNodes(/* @__PURE__ */ new Set());
    setSearchQuery("");
    setSearchMatchIdx(0);
    setSearchOpen(false);
    setNodeInfo(null);
    transformRef.current = null;
  }, [newick]);`,
    `  const containerHRef = useRef(defaultHeight);
  containerHRef.current = containerH;
  const snapshotState = useCallback(() => {
    const cladeEditorDraft = cladeEditor?.sid ? document.getElementById(\`clade-input-\${cladeEditor.sid}\`)?.value ?? "" : "";
    return {
      schema_version: 1,
      treeData: cloneTreeData(treeData),
      history: history.map(cloneTreeData),
      currentNewick: toNewick(treeData) + ";",
      layout,
      treeType,
      labelMode,
      alignLabels,
      truncateNames,
      showAlignment,
      containerH,
      hScale,
      vScale,
      fontScale,
      strokeWidth,
      colorOverrides: mapToEntries(colorOverrides),
      collapsedNodes: setToValues(collapsedNodes),
      collapseLabels: mapToEntries(collapseLabels),
      activeColor,
      colorMode,
      rerootMode,
      flipMode,
      swapMode,
      searchOpen,
      searchQuery,
      searchMatchIdx,
      downloadOpen,
      downloadMenuPos,
      palettePos,
      cladeEditor: cladeEditor ? { ...cladeEditor, draft: cladeEditorDraft } : null,
      nodeInfo,
      transform: transformToSnapshot(transformRef.current),
      theme
    };
  }, [treeData, history, layout, treeType, labelMode, alignLabels, truncateNames, showAlignment, containerH, hScale, vScale, fontScale, strokeWidth, colorOverrides, collapsedNodes, collapseLabels, activeColor, colorMode, rerootMode, flipMode, swapMode, searchOpen, searchQuery, searchMatchIdx, downloadOpen, downloadMenuPos, palettePos, cladeEditor, nodeInfo, theme]);
  useEffect(() => {
    onStateChange?.(snapshotState());
  }, [onStateChange, snapshotState]);
  useEffect(() => {
    const snapshot = normalizeReactreeStateSnapshot(initialState);
    dispatch({ type: "RESET", data: snapshot.treeData ? cloneTreeData(snapshot.treeData) : parseNewick(newick) });
    setRerootMode(boolOr(snapshot.rerootMode, false));
    setFlipMode(boolOr(snapshot.flipMode, false));
    setSwapMode(boolOr(snapshot.swapMode, false));
    setSwapFirst(null);
    setColorMode(boolOr(snapshot.colorMode, false));
    setActiveColor(stringOr(snapshot.activeColor, PALETTE[0].hex));
    setColorOverrides(entriesToMap(snapshot.colorOverrides));
    setCollapsedNodes(valuesToSet(snapshot.collapsedNodes));
    setCollapseLabels(entriesToMap(snapshot.collapseLabels));
    setLayout(stringOr(snapshot.layout, "rectangular"));
    setTreeType(stringOr(snapshot.treeType, "phylogram"));
    setLabelMode(stringOr(snapshot.labelMode, "bootstrap"));
    setAlignLabels(boolOr(snapshot.alignLabels, false));
    setShowAlignment(boolOr(snapshot.showAlignment, false));
    setContainerH(numberOr(snapshot.containerH, defaultHeight));
    setHScale(numberOr(snapshot.hScale, 1));
    setVScale(numberOr(snapshot.vScale, 1));
    setFontScale(numberOr(snapshot.fontScale, 1));
    setStrokeWidth(numberOr(snapshot.strokeWidth, 1.5));
    setSearchQuery(stringOr(snapshot.searchQuery, ""));
    setSearchMatchIdx(numberOr(snapshot.searchMatchIdx, 0));
    setSearchOpen(boolOr(snapshot.searchOpen, false));
    setDownloadOpen(boolOr(snapshot.downloadOpen, false));
    setDownloadMenuPos(snapshot.downloadMenuPos ?? null);
    setPalettePos(snapshot.palettePos ?? null);
    setCladeEditor(snapshot.cladeEditor ?? null);
    setNodeInfo(snapshot.nodeInfo ?? null);
    transformRef.current = transformFromSnapshot(snapshot.transform);
  }, [newick, initialState]);`,
    'Reactree viewer-state snapshot effect',
    file,
  );
  text = replaceRequired(
    text,
    '                  defaultValue: existing?.label ?? "",',
    '                  defaultValue: cladeEditor.draft ?? existing?.label ?? "",',
    'Reactree clade editor draft restore',
    file,
  );
  text = replaceRequired(
    text,
    '            /* @__PURE__ */ jsx2("div", { className: Reactree_default.downloadMenuDivider }),',
    viewerStateMenu,
    'Reactree PGV export menu item',
    file,
  );
  writeFileSync(file, text);
  console.log(`Patched Reactree viewer-state bridge: ${file}`);
}

patchReactreeViewerStateMJS(join(packageRoot, 'dist', 'index.mjs'));

function patchReactreeViewerPolishMJS(file) {
  let text = readFileSync(file, 'utf8').replace(/\r\n/g, '\n');
  const originalText = text;
  const okabeItoPalette = `var PALETTE = [
  { hex: "#0072b2", name: "Okabe-Ito Blue" },
  { hex: "#e69f00", name: "Okabe-Ito Orange" },
  { hex: "#009e73", name: "Okabe-Ito Bluish Green" },
  { hex: "#d55e00", name: "Okabe-Ito Vermillion" },
  { hex: "#cc79a7", name: "Okabe-Ito Reddish Purple" },
  { hex: "#56b4e9", name: "Okabe-Ito Sky Blue" },
  { hex: "#f0e442", name: "Okabe-Ito Yellow" },
  { hex: "#000000", name: "Okabe-Ito Black" }
];`;
  const wcagAAAPalette = `var PALETTE = [
  { hex: "#000000", name: "Black" },
  { hex: "#2f4f4f", name: "DarkSlateGray" },
  { hex: "#000080", name: "Navy" },
  { hex: "#00008b", name: "DarkBlue" },
  { hex: "#191970", name: "MidnightBlue" },
  { hex: "#0000cd", name: "MediumBlue" },
  { hex: "#0000ff", name: "Blue" },
  { hex: "#4b0082", name: "Indigo" },
  { hex: "#800080", name: "Purple" },
  { hex: "#483d8b", name: "DarkSlateBlue" },
  { hex: "#8b008b", name: "DarkMagenta" },
  { hex: "#8b0000", name: "DarkRed" },
  { hex: "#800000", name: "Maroon" },
  { hex: "#8b4513", name: "SaddleBrown" },
  { hex: "#a52a2a", name: "Brown" },
  { hex: "#006400", name: "DarkGreen" }
];`;
  const legacyMegaTreeFontLine = '    const treeFontFamily = isMegaStyle ? "\\"Times New ' + 'Roman\\", Times, serif" : "system-ui, -apple-system, sans-serif";';

  for (const [original, replacement] of [
    ['    const scrollWrap = alnScrollWrapRef.current;\n    const visibleStartCol = scrollWrap ? Math.max(0, Math.floor(scrollWrap.scrollLeft / CELL_W) - 2) : 0;\n    const visibleEndCol = scrollWrap ? Math.min(maxLen, Math.ceil((scrollWrap.scrollLeft + scrollWrap.clientWidth) / CELL_W) + 2) : maxLen;\n    const paintX = visibleStartCol * CELL_W;\n    const paintW = Math.max(CELL_W, (visibleEndCol - visibleStartCol) * CELL_W);\n', ''],
    ['    sc.fillRect(paintX, 0, paintW, H);', '    sc.fillRect(0, 0, totalW, H);'],
    ['        sc.fillRect(paintX, top, paintW, cellH);', '        sc.fillRect(0, top, totalW, cellH);'],
    ['        sc.fillRect(paintX, top + cellH - 0.5, paintW, 0.5);', '        sc.fillRect(0, top + cellH - 0.5, totalW, 0.5);'],
    ['      for (let i = visibleStartCol; i < Math.min(seq.length, visibleEndCol); i++) {', '      for (let i = 0; i < seq.length; i++) {'],
    ['    sc.fillRect(paintX, 0, paintW, AXIS_H);', '    sc.fillRect(0, 0, totalW, AXIS_H);'],
    ['    sc.fillRect(paintX, AXIS_H - 1, paintW, 1);', '    sc.fillRect(0, AXIS_H - 1, totalW, 1);'],
    ['    for (let i = visibleStartCol; i < visibleEndCol; i++) {', '    for (let i = 0; i < maxLen; i++) {'],
  ]) {
    text = text.split(original).join(replacement);
  }

  function replaceOnce(original, replacement, description) {
    if (text.includes(replacement)) return;
    if (!text.includes(original)) {
      throw new Error(`${description} anchor not found in ${file}`);
    }
    text = text.replace(original, replacement);
    console.log(`Patched ${description}: ${file}`);
  }

  function replaceAll(original, replacement, description) {
    if (text.includes(replacement)) return;
    if (!text.includes(original)) {
      throw new Error(`${description} anchor not found in ${file}`);
    }
    text = text.split(original).join(replacement);
    console.log(`Patched ${description}: ${file}`);
  }

  text = text.split(wcagAAAPalette).join(okabeItoPalette);
  replaceOnce(
    'var PALETTE = [\n  { hex: "#ef4444", name: "Red" },\n  { hex: "#f97316", name: "Orange" },\n  { hex: "#f59e0b", name: "Amber" },\n  { hex: "#84cc16", name: "Lime" },\n  { hex: "#10b981", name: "Emerald" },\n  { hex: "#14b8a6", name: "Teal" },\n  { hex: "#0ea5e9", name: "Sky" },\n  { hex: "#6366f1", name: "Indigo" },\n  { hex: "#8b5cf6", name: "Violet" },\n  { hex: "#ec4899", name: "Pink" },\n  { hex: "#64748b", name: "Slate" },\n  { hex: "#92400e", name: "Brown" }\n];',
    okabeItoPalette,
    'Reactree Okabe-Ito clade palette',
  );
  replaceOnce(
    '  const [layout, setLayout] = useState(stringOr(initialSnapshot.layout, "rectangular"));',
    '  const [layout, setLayout] = useState(stringOr(initialSnapshot.layout, "rectangular"));\n  const [renderStyle, setRenderStyle] = useState(stringOr(initialSnapshot.renderStyle, "phgo"));',
    'Reactree PHgo/MEGA style state',
  );
  replaceOnce(
    'function stringOr(value, fallback) {\n  return typeof value === "string" ? value : fallback;\n}\nfunction transformToSnapshot(transform) {',
    'function stringOr(value, fallback) {\n  return typeof value === "string" ? value : fallback;\n}\nconst DEFAULT_TREE_FONT_FAMILY = "system-ui, -apple-system, sans-serif";\nconst FALLBACK_FONT_FAMILIES = [\n  "Arial",\n  "Helvetica",\n  "Verdana",\n  "Tahoma",\n  "Trebuchet MS",\n  "Georgia",\n  "Times New Roman",\n  "Courier New",\n  "Monaco",\n  "Arial Narrow"\n];\nfunction normalizeFontFamilyName(value) {\n  return typeof value === "string" ? value.trim() : "";\n}\nfunction buildFontOptions(values) {\n  const seen = /* @__PURE__ */ new Set();\n  const options = [];\n  const push = (family) => {\n    const trimmed = normalizeFontFamilyName(family);\n    if (!trimmed) return;\n    const key = trimmed.toLowerCase();\n    if (seen.has(key)) return;\n    seen.add(key);\n    options.push(trimmed);\n  };\n  push(DEFAULT_TREE_FONT_FAMILY);\n  Array.isArray(values) && values.forEach(push);\n  FALLBACK_FONT_FAMILIES.forEach(push);\n  if (options.length > 1) {\n    const [defaultFamily, ...rest] = options;\n    rest.sort((left, right) => left.localeCompare(right, void 0, { sensitivity: "base" }));\n    return [defaultFamily, ...rest];\n  }\n  return options;\n}\nasync function loadLocalFontFamilies(currentFontFamily) {\n  const discovered = [currentFontFamily];\n  if (typeof window !== "undefined" && typeof window.queryLocalFonts === "function") {\n    try {\n      const fonts = await window.queryLocalFonts();\n      fonts.forEach((font) => discovered.push(font?.family));\n    } catch {\n    }\n  }\n  if (typeof document !== "undefined" && document.fonts && typeof document.fonts.forEach === "function") {\n    try {\n      document.fonts.forEach((fontFace) => discovered.push(fontFace?.family));\n    } catch {\n    }\n  }\n  return buildFontOptions(discovered);\n}\nfunction transformToSnapshot(transform) {',
    'Reactree font helper utilities',
  );
  replaceOnce(
    '  const [fontScale, setFontScale] = useState(numberOr(initialSnapshot.fontScale, 1));',
    '  const [fontScale, setFontScale] = useState(numberOr(initialSnapshot.fontScale, 1));\n  const [fontFamily, setFontFamily] = useState(stringOr(initialSnapshot.fontFamily, DEFAULT_TREE_FONT_FAMILY));\n  const [fontOptions, setFontOptions] = useState(() => buildFontOptions([stringOr(initialSnapshot.fontFamily, DEFAULT_TREE_FONT_FAMILY)]));',
    'Reactree font picker state',
  );
  replaceOnce(
    '  const fontScaleRef = useRef(1);\n  fontScaleRef.current = fontScale;\n  const strokeWidthRef = useRef(1.5);',
    '  const fontScaleRef = useRef(1);\n  fontScaleRef.current = fontScale;\n  const fontFamilyRef = useRef(DEFAULT_TREE_FONT_FAMILY);\n  fontFamilyRef.current = fontFamily;\n  const strokeWidthRef = useRef(1.5);',
    'Reactree font family ref',
  );
  replaceOnce(
    '  const containerHRef = useRef(defaultHeight);\n  containerHRef.current = containerH;\n  const snapshotState = useCallback(() => {',
    '  const containerHRef = useRef(defaultHeight);\n  containerHRef.current = containerH;\n  useEffect(() => {\n    let cancelled = false;\n    loadLocalFontFamilies(fontFamily).then((families) => {\n      if (cancelled) return;\n      setFontOptions((current) => {\n        const next = buildFontOptions([...current, ...families, fontFamily]);\n        if (next.length === current.length && next.every((family, idx) => family === current[idx])) {\n          return current;\n        }\n        return next;\n      });\n    }).catch(() => {\n    });\n    return () => {\n      cancelled = true;\n    };\n  }, [fontFamily]);\n  const snapshotState = useCallback(() => {',
    'Reactree font family loader',
  );
  replaceOnce(
    '      layout,\n      treeType,',
    '      layout,\n      renderStyle,\n      treeType,',
    'Reactree style snapshot field',
  );
  replaceOnce(
    '      fontScale,\n      strokeWidth,',
    '      fontScale,\n      fontFamily,\n      strokeWidth,',
    'Reactree font snapshot field',
  );
  const snapshotDepsOriginal = '  }, [treeData, history, layout, treeType, labelMode, alignLabels, truncateNames, showAlignment, containerH, hScale, vScale, fontScale, strokeWidth, colorOverrides, collapsedNodes, collapseLabels, activeColor, colorMode, rerootMode, flipMode, swapMode, searchOpen, searchQuery, searchMatchIdx, downloadOpen, downloadMenuPos, palettePos, cladeEditor, nodeInfo, theme]);';
  const snapshotDepsRenderStyle = '  }, [treeData, history, layout, renderStyle, treeType, labelMode, alignLabels, truncateNames, showAlignment, containerH, hScale, vScale, fontScale, strokeWidth, colorOverrides, collapsedNodes, collapseLabels, activeColor, colorMode, rerootMode, flipMode, swapMode, searchOpen, searchQuery, searchMatchIdx, downloadOpen, downloadMenuPos, palettePos, cladeEditor, nodeInfo, theme]);';
  const snapshotDepsRuntimeFont = '  }, [treeData, history, layout, renderStyle, treeType, labelMode, alignLabels, truncateNames, showAlignment, containerH, hScale, vScale, fontScale, fontFamily, strokeWidth, colorOverrides, collapsedNodes, collapseLabels, activeColor, colorMode, rerootMode, flipMode, swapMode, searchOpen, searchQuery, searchMatchIdx, downloadOpen, downloadMenuPos, palettePos, cladeEditor, nodeInfo, theme]);';
  if (!text.includes(snapshotDepsRuntimeFont)) {
    if (text.includes(snapshotDepsOriginal)) {
      replaceOnce(snapshotDepsOriginal, snapshotDepsRenderStyle, 'Reactree style snapshot dependencies');
    }
    if (text.includes(snapshotDepsRenderStyle)) {
      replaceOnce(snapshotDepsRenderStyle, snapshotDepsRuntimeFont, 'Reactree font snapshot dependencies');
    }
    if (!text.includes(snapshotDepsRuntimeFont)) {
      throw new Error(`Reactree snapshot dependencies anchor not found in ${file}`);
    }
  }
  const snapshotRestoreOriginal = '    setLayout(stringOr(snapshot.layout, "rectangular"));\n    setTreeType(stringOr(snapshot.treeType, "phylogram"));';
  const snapshotRestoreRenderStyle = '    setLayout(stringOr(snapshot.layout, "rectangular"));\n    setRenderStyle(stringOr(snapshot.renderStyle, "phgo"));\n    setTreeType(stringOr(snapshot.treeType, "phylogram"));';
  const snapshotRestoreRuntimeFont = '    setLayout(stringOr(snapshot.layout, "rectangular"));\n    setRenderStyle(stringOr(snapshot.renderStyle, "phgo"));\n    setTreeType(stringOr(snapshot.treeType, "phylogram"));\n    setFontFamily(stringOr(snapshot.fontFamily, DEFAULT_TREE_FONT_FAMILY));';
  if (!text.includes(snapshotRestoreRuntimeFont)) {
    if (text.includes(snapshotRestoreOriginal)) {
      replaceOnce(snapshotRestoreOriginal, snapshotRestoreRenderStyle, 'Reactree style restore');
    }
    if (text.includes(snapshotRestoreRenderStyle)) {
      replaceOnce(snapshotRestoreRenderStyle, snapshotRestoreRuntimeFont, 'Reactree font restore');
    }
    if (!text.includes(snapshotRestoreRuntimeFont)) {
      throw new Error(`Reactree snapshot restore anchor not found in ${file}`);
    }
  }
  replaceOnce(
    '    const isCircular = layout === "circular";',
    '    const isCircular = layout === "circular";\n    const isMegaStyle = renderStyle === "mega";',
    'Reactree style render flag',
  );
  text = text.split(
    '    const baseColor = isMegaStyle ? "#000000" : isDark ? "#cbd5e1" : "#475569";\n    const scaleColor = isMegaStyle ? "#000000" : isDark ? "#94a3b8" : "#64748b";\n    const branchLenColor = isMegaStyle ? "#000000" : isDark ? "#22d3ee" : "#0891b2";\n' + legacyMegaTreeFontLine,
  ).join(
    '    const baseColor = isMegaStyle ? "#000000" : isDark ? "#cbd5e1" : "#475569";\n    const scaleColor = isMegaStyle ? "#000000" : isDark ? "#94a3b8" : "#64748b";\n    const branchLenColor = isMegaStyle ? "#000000" : isDark ? "#22d3ee" : "#0891b2";\n    const treeFontFamily = fontFamily;',
  );
  const megaPaletteOriginal = '    const baseColor = isDark ? "#cbd5e1" : "#475569";\n    const scaleColor = isDark ? "#94a3b8" : "#64748b";\n    const branchLenColor = isDark ? "#22d3ee" : "#0891b2";';
  const megaPaletteWithHardcodedFont = '    const baseColor = isMegaStyle ? "#000000" : isDark ? "#cbd5e1" : "#475569";\n    const scaleColor = isMegaStyle ? "#000000" : isDark ? "#94a3b8" : "#64748b";\n    const branchLenColor = isMegaStyle ? "#000000" : isDark ? "#22d3ee" : "#0891b2";\n    const treeFontFamily = "system-ui, -apple-system, sans-serif";';
  const megaPaletteWithRuntimeFont = '    const baseColor = isMegaStyle ? "#000000" : isDark ? "#cbd5e1" : "#475569";\n    const scaleColor = isMegaStyle ? "#000000" : isDark ? "#94a3b8" : "#64748b";\n    const branchLenColor = isMegaStyle ? "#000000" : isDark ? "#22d3ee" : "#0891b2";\n    const treeFontFamily = fontFamily;';
  if (!text.includes(megaPaletteWithRuntimeFont)) {
    if (text.includes(megaPaletteOriginal)) {
      replaceOnce(
        megaPaletteOriginal,
        megaPaletteWithRuntimeFont,
        'Reactree MEGA palette and font',
      );
    } else if (text.includes(megaPaletteWithHardcodedFont)) {
      replaceOnce(
        megaPaletteWithHardcodedFont,
        megaPaletteWithRuntimeFont,
        'Reactree MEGA palette runtime font upgrade',
      );
    } else {
      throw new Error(`Reactree MEGA palette and font anchor not found in ${file}`);
    }
  }
  if (!text.includes('    const treeFontFamily = fontFamily;') && text.includes(legacyMegaTreeFontLine)) {
    replaceOnce(
      legacyMegaTreeFontLine,
      '    const treeFontFamily = fontFamily;',
      'Reactree MEGA font reset',
    );
  }
  replaceOnce(
    '    const { seqMap, maxLen } = pfasta;\n    const t = transformRef.current ?? d32.zoomIdentity;\n',
    '    const { seqMap, maxLen } = pfasta;\n    const canvasFontFamily = fontFamilyRef.current;\n    const t = transformRef.current ?? d32.zoomIdentity;\n',
    'Reactree alignment canvas font family state',
  );
  replaceAll(
    '`400 ${numFs}px system-ui,-apple-system,sans-serif`',
    '`400 ${numFs}px ${canvasFontFamily}`',
    'Reactree alignment row number font',
  );
  replaceAll(
    '`italic 500 ${fs}px system-ui,-apple-system,sans-serif`',
    '`italic 500 ${fs}px ${canvasFontFamily}`',
    'Reactree alignment label font',
  );
  replaceAll(
    '`600 8.5px system-ui,-apple-system,sans-serif`',
    '`600 8.5px ${canvasFontFamily}`',
    'Reactree alignment header font',
  );
  replaceAll(
    '`400 7.5px system-ui,-apple-system,sans-serif`',
    '`400 7.5px ${canvasFontFamily}`',
    'Reactree alignment index font',
  );
  replaceAll(
    '"8.5px system-ui,-apple-system,sans-serif"',
    '`8.5px ${canvasFontFamily}`',
    'Reactree alignment axis font',
  );
  replaceOnce(
    '    const treeFontFamily = "system-ui, -apple-system, sans-serif";',
    '    const treeFontFamily = fontFamily;',
    'Reactree tree font family state',
  );
  replaceAll(
    '.attr("stroke-opacity", 0.8)',
    '.attr("stroke-opacity", isMegaStyle ? 1 : 0.8)',
    'Reactree MEGA line opacity',
  );
  replaceAll(
    'linkPaths.filter((p) => p === d).attr("stroke-opacity", 0.8)',
    'linkPaths.filter((p) => p === d).attr("stroke-opacity", isMegaStyle ? 1 : 0.8)',
    'Reactree MEGA hover line opacity',
  );
  replaceAll(
    '      _addNodeVisuals(node);',
    '      if (!isMegaStyle) _addNodeVisuals(node);',
    'Reactree MEGA node circle removal',
  );
  replaceAll(
    '.attr("font-family", "system-ui, -apple-system, sans-serif").style("fill", "#ef4444")',
    '.attr("font-family", treeFontFamily).style("fill", isMegaStyle ? "#000000" : "#ef4444")',
    'Reactree MEGA bootstrap label style',
  );
  replaceAll(
    '.attr("font-family", "system-ui, -apple-system, sans-serif").style("fill", branchLenColor)',
    '.attr("font-family", treeFontFamily).style("fill", branchLenColor)',
    'Reactree MEGA branch label font',
  );
  replaceOnce(
    '        }).attr("text-anchor", "middle");\n      } else {\n        labels.attr("x", (d) => (d.parent.y - d.y) / 2).attr("y", -4).attr("text-anchor", "middle");\n      }',
    '        }).attr("text-anchor", isMegaStyle ? "end" : "middle");\n      } else {\n        labels.attr("x", isMegaStyle ? -6 : (d) => (d.parent.y - d.y) / 2).attr("y", -4).attr("text-anchor", isMegaStyle ? "end" : "middle");\n      }',
    'Reactree MEGA branch label alignment',
  );
  replaceAll(
    '.attr("font-family", "system-ui, -apple-system, sans-serif").attr("font-style", "italic")',
    '.attr("font-family", treeFontFamily).attr("font-style", isMegaStyle ? "normal" : "italic")',
    'Reactree MEGA leaf label font',
  );
  replaceOnce(
    '        (sel) => sel.attr("x", alignLabels ? (d) => innerWidth - d.y + 10 : 12).attr("dy", "0.35em").attr("font-size", fontSize).attr("font-family", treeFontFamily).attr("font-style", isMegaStyle ? "normal" : "italic")',
    '        (sel) => sel.attr("x", alignLabels ? (d) => innerWidth - d.y + (isMegaStyle ? 4 : 10) : isMegaStyle ? 4 : 12).attr("dy", "0.35em").attr("font-size", fontSize).attr("font-family", treeFontFamily).attr("font-style", isMegaStyle ? "normal" : "italic")',
    'Reactree MEGA rectangular label gap',
  );
  replaceOnce(
    '      if (alignLabels) {\n        const leaderColor = isMegaStyle ? "#000000" : isDark ? "rgba(148,163,184,0.35)" : "rgba(100,116,139,0.3)";\n        node.filter((d) => !d.children).append("line").attr("class", "leader-line").attr("x1", 8).attr("x2", (d) => innerWidth - d.y).attr("y1", 0).attr("y2", 0).attr("stroke", leaderColor).attr("stroke-width", 1).attr("stroke-dasharray", "2 3").style("pointer-events", "none");\n      }\n      addLeafLabels(',
    '      if (alignLabels) {\n        const leaderColor = isMegaStyle ? "#000000" : isDark ? "rgba(148,163,184,0.35)" : "rgba(100,116,139,0.3)";\n        node.filter((d) => !d.children).append("line").attr("class", "leader-line").attr("x1", isMegaStyle ? 0 : 8).attr("x2", (d) => innerWidth - d.y + (isMegaStyle ? 4 : 0)).attr("y1", 0).attr("y2", 0).attr("stroke", leaderColor).attr("stroke-width", 1).attr("stroke-dasharray", isMegaStyle ? null : "2 3").style("pointer-events", "none");\n      }\n      if (isMegaStyle && !alignLabels) {\n        node.filter((d) => !d.children).append("line").attr("class", "terminal-label-line").attr("x1", 0).attr("x2", 4).attr("y1", 0).attr("y2", 0).attr("stroke", baseColor).attr("stroke-width", strokeWidth).attr("stroke-linecap", "square").style("pointer-events", "none");\n      }\n      addLeafLabels(',
    'Reactree MEGA rectangular terminal label connector',
  );
  replaceOnce(
    '          if (!alignLabels) return d.x < Math.PI ? 12 : -12;\n          return d.x < Math.PI ? radius - d.y + 12 : -(radius - d.y) - 12;',
    '          const labelPad = isMegaStyle ? 4 : 12;\n          if (!alignLabels) return d.x < Math.PI ? labelPad : -labelPad;\n          return d.x < Math.PI ? radius - d.y + labelPad : -(radius - d.y) - labelPad;',
    'Reactree MEGA circular label gap',
  );
  replaceAll(
    '.style("user-select", "none").text((d) => {',
    '.style("user-select", "none").style("-webkit-user-select", "none").attr("unselectable", "on").text((d) => {',
    'Reactree SVG text native selection guard',
  );
  replaceAll(
    '.attr("font-family", "system-ui, -apple-system, sans-serif").attr("font-style", cladeInfo?.label ? "normal" : "italic")',
    '.attr("font-family", treeFontFamily).attr("font-style", isMegaStyle ? "normal" : cladeInfo?.label ? "normal" : "italic")',
    'Reactree MEGA clade label font',
  );
  replaceAll(
    '        g.append("circle").attr("cx", 0).attr("cy", 0).attr("r", 3).attr("fill", triColor).attr("stroke", isDark ? "#1e293b" : "#fff").attr("stroke-width", 1.5).style("pointer-events", "none");',
    '        if (!isMegaStyle) g.append("circle").attr("cx", 0).attr("cy", 0).attr("r", 3).attr("fill", triColor).attr("stroke", isDark ? "#1e293b" : "#fff").attr("stroke-width", 1.5).style("pointer-events", "none");',
    'Reactree MEGA clade circle removal',
  );
  replaceAll(
    '        const leaderColor = isDark ? "rgba(148,163,184,0.35)" : "rgba(100,116,139,0.3)";',
    '        const leaderColor = isMegaStyle ? "#000000" : isDark ? "rgba(148,163,184,0.35)" : "rgba(100,116,139,0.3)";',
    'Reactree MEGA leader line color',
  );
  replaceOnce(
    '        d32.select(el).insert("circle", ":first-child").attr("class", "search-glow").attr("r", r).attr("fill", isCurrent ? "rgba(245,158,11,0.14)" : isDark ? "rgba(129,140,248,0.12)" : "rgba(99,102,241,0.09)").attr("stroke", color).attr("stroke-width", isCurrent ? 2.2 : 1.6).style("pointer-events", "none");\n        positions.push(getPos(d));',
    '        if (!isMegaStyle) d32.select(el).insert("circle", ":first-child").attr("class", "search-glow").attr("r", r).attr("fill", isCurrent ? "rgba(245,158,11,0.14)" : isDark ? "rgba(129,140,248,0.12)" : "rgba(99,102,241,0.09)").attr("stroke", color).attr("stroke-width", isCurrent ? 2.2 : 1.6).style("pointer-events", "none");\n        positions.push(getPos(d));',
    'Reactree MEGA search circle removal',
  );
  replaceOnce(
    '      if (isPhylogram && maxCumLen > 0) {\n        const scaleX = d32.scaleLinear().domain([0, maxCumLen]).range([0, innerWidth]);\n        const ticks = scaleX.ticks(8);\n        const sb = content.append("g").attr("transform", `translate(0,${innerHeight + 28})`);\n        sb.append("line").attr("x1", 0).attr("x2", innerWidth).attr("stroke", scaleColor).attr("stroke-width", 1).attr("stroke-linecap", "round");\n        ticks.forEach((t, i) => {\n          const x = scaleX(t), e = i === 0 || i === ticks.length - 1;\n          sb.append("line").attr("x1", x).attr("x2", x).attr("y2", e ? 8 : 5).attr("stroke", scaleColor).attr("stroke-width", e ? 1.4 : 1);\n          sb.append("text").attr("x", x).attr("y", 20).attr("text-anchor", i === 0 ? "start" : i === ticks.length - 1 ? "end" : "middle").attr("font-size", "9.5px").attr("font-family", "system-ui").style("fill", "var(--clr-text-muted,#64748b)").text(formatTick(t));\n        });\n        sb.append("text").attr("x", innerWidth / 2).attr("y", 36).attr("text-anchor", "middle").attr("font-size", "9px").attr("font-family", "system-ui").attr("letter-spacing", "0.04em").style("fill", "var(--clr-text-muted,#64748b)").text("substitutions / site");\n      }',
    '      if (isPhylogram && maxCumLen > 0) {\n        if (isMegaStyle) {\n          const barPx = Math.max(52, Math.min(120, innerWidth * 0.16));\n          const barLen = maxCumLen * barPx / innerWidth;\n          const sb = content.append("g").attr("transform", `translate(0,${innerHeight + 30})`);\n          sb.append("line").attr("x1", 0).attr("x2", barPx).attr("stroke", scaleColor).attr("stroke-width", 1.5).attr("stroke-linecap", "square");\n          [0, barPx].forEach((x) => sb.append("line").attr("x1", x).attr("x2", x).attr("y1", -5).attr("y2", 5).attr("stroke", scaleColor).attr("stroke-width", 1.5));\n          sb.append("text").attr("x", barPx / 2).attr("y", -8).attr("text-anchor", "middle").attr("font-size", "10px").attr("font-family", treeFontFamily).style("fill", scaleColor).text(formatTick(barLen));\n        } else {\n          const scaleX = d32.scaleLinear().domain([0, maxCumLen]).range([0, innerWidth]);\n          const ticks = scaleX.ticks(8);\n          const sb = content.append("g").attr("transform", `translate(0,${innerHeight + 28})`);\n          sb.append("line").attr("x1", 0).attr("x2", innerWidth).attr("stroke", scaleColor).attr("stroke-width", 1).attr("stroke-linecap", "round");\n          ticks.forEach((t, i) => {\n            const x = scaleX(t), e = i === 0 || i === ticks.length - 1;\n            sb.append("line").attr("x1", x).attr("x2", x).attr("y2", e ? 8 : 5).attr("stroke", scaleColor).attr("stroke-width", e ? 1.4 : 1);\n            sb.append("text").attr("x", x).attr("y", 20).attr("text-anchor", i === 0 ? "start" : i === ticks.length - 1 ? "end" : "middle").attr("font-size", "9.5px").attr("font-family", "system-ui").style("fill", "var(--clr-text-muted,#64748b)").text(formatTick(t));\n          });\n          sb.append("text").attr("x", innerWidth / 2).attr("y", 36).attr("text-anchor", "middle").attr("font-size", "9px").attr("font-family", "system-ui").attr("letter-spacing", "0.04em").style("fill", "var(--clr-text-muted,#64748b)").text("substitutions / site");\n        }\n      }',
    'Reactree MEGA rectangular scale bar',
  );
  replaceAll(
    '.attr("font-family", "system-ui").style("fill", "var(--clr-text-muted,#64748b)")',
    '.attr("font-family", treeFontFamily).style("fill", scaleColor)',
    'Reactree MEGA scale font',
  );
  text = text.split(
    '        sb.append("text").attr("x", barPx / 2).attr("y", 16).attr("text-anchor", "middle").attr("font-size", "9.5px").attr("font-family", "system-ui").style("fill", "var(--clr-text-muted,#64748b)").text(formatTick(barLen));',
  ).join(
    '        sb.append("text").attr("x", barPx / 2).attr("y", 16).attr("text-anchor", "middle").attr("font-size", "9.5px").attr("font-family", treeFontFamily).style("fill", scaleColor).text(formatTick(barLen));',
  );
  replaceOnce(
    '  }, [showAlignment, parsedFasta, containerH, vScale, fontScale, layout, drawAlnCanvas]);',
    '  }, [showAlignment, parsedFasta, containerH, vScale, fontScale, fontFamily, layout, drawAlnCanvas]);',
    'Reactree alignment redraw font dependency',
  );
  const renderDepsOriginal = '  }, [treeData, containerH, hScale, vScale, fontScale, strokeWidth, layout, treeType, colorOverrides, theme, labelMode, alignLabels, truncateNames, collapsedNodes, collapseLabels, searchQuery, searchMatchIdx, showAlignment]);';
  const renderDepsRenderStyle = '  }, [treeData, containerH, hScale, vScale, fontScale, strokeWidth, layout, renderStyle, treeType, colorOverrides, theme, labelMode, alignLabels, truncateNames, collapsedNodes, collapseLabels, searchQuery, searchMatchIdx, showAlignment]);';
  const renderDepsRuntimeFont = '  }, [treeData, containerH, hScale, vScale, fontScale, fontFamily, strokeWidth, layout, renderStyle, treeType, colorOverrides, theme, labelMode, alignLabels, truncateNames, collapsedNodes, collapseLabels, searchQuery, searchMatchIdx, showAlignment]);';
  if (!text.includes(renderDepsRuntimeFont)) {
    if (text.includes(renderDepsOriginal)) {
      replaceOnce(renderDepsOriginal, renderDepsRenderStyle, 'Reactree style render dependencies');
    }
    if (text.includes(renderDepsRenderStyle)) {
      replaceOnce(renderDepsRenderStyle, renderDepsRuntimeFont, 'Reactree tree font render dependencies');
    }
    if (!text.includes(renderDepsRuntimeFont)) {
      throw new Error(`Reactree render dependencies anchor not found in ${file}`);
    }
  }
  replaceOnce(
    '        }, title: "Circular", children: /* @__PURE__ */ jsx2(IconCirc, {}) })\n      ] }),\n      /* @__PURE__ */ jsx2("div", { className: Reactree_default.divider }),',
    '        }, title: "Circular", children: /* @__PURE__ */ jsx2(IconCirc, {}) })\n      ] }),\n      /* @__PURE__ */ jsx2("div", { className: Reactree_default.divider }),\n      /* @__PURE__ */ jsxs2("div", { className: Reactree_default.btnGroup, children: [\n        /* @__PURE__ */ jsx2("button", { className: `${Reactree_default.btnGroupItem} ${renderStyle === "phgo" ? Reactree_default.btnGroupActive : ""}`, onClick: () => setRenderStyle("phgo"), title: "PHgo style", children: "P" }),\n        /* @__PURE__ */ jsx2("button", { className: `${Reactree_default.btnGroupItem} ${renderStyle === "mega" ? Reactree_default.btnGroupActive : ""}`, onClick: () => setRenderStyle("mega"), title: "MEGA style", children: "M" })\n      ] }),\n      /* @__PURE__ */ jsx2("div", { className: Reactree_default.divider }),',
    'Reactree PHgo/MEGA toolbar toggle',
  );

  if (text.includes('visibleStartCol') || text.includes('paintX') || text.includes('paintW')) {
    throw new Error(`Reactree alignment visible-column optimization still present in ${file}`);
  }
  if (text !== originalText) {
    writeFileSync(file, text);
    console.log(`Patched Reactree viewer polish: ${file}`);
  } else {
    console.log(`Reactree viewer polish already present: ${file}`);
  }
}

patchReactreeViewerPolishMJS(join(packageRoot, 'dist', 'index.mjs'));

for (const file of ['index.d.ts', 'index.d.mts'].map((name) => join(packageRoot, 'dist', name))) {
  let text = readFileSync(file, 'utf8');
  if (!text.includes('onViewerSnapshot')) {
    text = text
      .replace('    fasta?: string;\n', '    fasta?: string;\n    initialState?: Record<string, unknown>;\n    onStateChange?: (state: Record<string, unknown>) => void;\n    onViewerSnapshot?: (state: Record<string, unknown>) => void;\n')
      .replace('declare function Reactree({ newick, defaultHeight, fasta }: ReactreeProps)', 'declare function Reactree({ newick, defaultHeight, fasta, initialState, onStateChange, onViewerSnapshot }: ReactreeProps)');
    writeFileSync(file, text);
    console.log(`Patched Reactree viewer-state types: ${file}`);
  } else {
    console.log(`Reactree viewer-state types already present: ${file}`);
  }
}
