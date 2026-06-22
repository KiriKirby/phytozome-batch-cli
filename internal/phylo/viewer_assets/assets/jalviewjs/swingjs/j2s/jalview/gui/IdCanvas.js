(function(){var P$=Clazz.newPackage("jalview.gui"),p$1={},I$=[[0,'java.awt.BorderLayout','jalview.gui.PaintRefresher','java.awt.Color','java.awt.image.BufferedImage','java.awt.Font','java.awt.RenderingHints','jalview.gui.AnnotationPanel','jalview.gui.AnnotationLabels']],$I$=function(i){return I$[i]||(I$[i]=Clazz.load(I$[0][i]))};
var C$=Clazz.newClass(P$, "IdCanvas", null, 'javax.swing.JPanel', 'jalview.viewmodel.ViewportListenerI');

C$.$clinit$ = function() {Clazz.load(C$, 1);
}

Clazz.newMeth(C$, '$init0$', function () {
var c;if((c = C$.superclazz) && (c = c.$init0$))c.apply(this);
this.av=null;
this.showScores=false;
this.maxIdLength=0;
this.maxIdStr=null;
this.image=null;
this.imgHeight=0;
this.fastPaint=false;
this.searchResults=null;
this.ap=null;
this.idfont=null;
}, 1);

Clazz.newMeth(C$, '$init$', function () {
this.showScores=true;
this.maxIdLength=-1;
this.maxIdStr=null;
this.imgHeight=0;
this.fastPaint=false;
}, 1);

Clazz.newMeth(C$, 'c$$jalview_gui_AlignViewport', function (av) {
Clazz.super_(C$, this,1);
this.setLayout$java_awt_LayoutManager(Clazz.new_($I$(1)));
this.av=av;
$I$(2).Register$java_awt_Component$S(this, av.getSequenceSetId$());
av.getRanges$().addPropertyChangeListener$jalview_viewmodel_ViewportListenerI(this);
}, 1);

Clazz.newMeth(C$, 'phgoCheckboxColumnWidth$', function () {
{
var bridge=window.__PHGOJalviewBridgeAPI;
return bridge && bridge.phgoState && bridge.phgoState.session ? bridge.checkboxColumnWidth() : 0;
}
});

Clazz.newMeth(C$, 'phgoSelectionState$jalview_datamodel_SequenceI$I', function (seq, index) {
{
var bridge=window.__PHGOJalviewBridgeAPI;
if (!bridge || !seq) return "";
try {
return bridge.selectionStateForSequence(seq.phgoTaxonID || "", seq.getName$(), index);
} catch (e) {
return "";
}
}
});

Clazz.newMeth(C$, 'phgoDisplayPrefix$jalview_datamodel_SequenceI$I', function (seq, index) {
{
var bridge=window.__PHGOJalviewBridgeAPI;
if (!bridge || !seq || !bridge.displayPrefixForSequence) return "";
try {
var name="";
try {
name=seq.getName$();
} catch (nameError) {
name="";
}
return bridge.displayPrefixForSequence(seq.phgoTaxonID || "", name, index) || "";
} catch (e) {
return "";
}
}
});

Clazz.newMeth(C$, 'phgoDisplayName$jalview_datamodel_SequenceI', function (seq) {
if (seq == null ) {
return "";
}
try {
var name=seq.getName$();
if (name != null  && String(name).length > 0) {
return name;
}
} catch (e) {
}
try {
var id=seq.getDisplayId$Z(false);
return id == null  ? "" : id;
} catch (e2) {
return "";
}
});

Clazz.newMeth(C$, 'phgoResidueRatio$jalview_datamodel_SequenceI', function (seq) {
if (seq == null ) {
return "";
}
try {
var text=seq.getSequenceAsString$();
if (text == null ) {
return "";
}
var total=String(text).length;
if (total < 1) {
return "";
}
var residues=0;
for (var i=0; i < total; i++) {
var ch=String(text).charAt(i);
if (ch !== "-" && ch !== "." && ch !== " " && ch !== "\t" && ch !== "\r" && ch !== "\n" && ch !== "*") {
residues++;
}
}
var pct=Math.round((residues * 1000) / total) / 10;
return " " + residues + "/" + total + " " + pct + "%";
} catch (e) {
return "";
}
});

Clazz.newMeth(C$, 'phgoDisplayLabel$jalview_datamodel_SequenceI$I', function (seq, index) {
var prefix=this.phgoDisplayPrefix$jalview_datamodel_SequenceI$I(seq, index);
var string=this.phgoDisplayName$jalview_datamodel_SequenceI(seq);
if (String(prefix).length > 0) {
string=prefix + " " + string;
}
return string + this.phgoResidueRatio$jalview_datamodel_SequenceI(seq);
});

Clazz.newMeth(C$, 'drawPHgoCheckbox$java_awt_Graphics2D$jalview_datamodel_SequenceI$I$I$I$I$java_awt_Color', function (g, s, index, y, charHeight, width, textColor) {
{
if (!width) return;
var state=this.phgoSelectionState$jalview_datamodel_SequenceI$I(s, index);
if (!state) return;
var size=Math.max(8, Math.min(11, charHeight - 5));
var x=Math.max(2, ((width - size)/2|0));
var yy=y + Math.max(1, ((charHeight - size)/2|0));
var border=$I$(3).gray;
var fill=$I$(3).white;
if (state === "green") {
border=Clazz.new_($I$(3).c$$I$I$I,[22, 128, 72]);
}
g.setColor$java_awt_Color(fill);
g.fillRect$I$I$I$I(x, yy, size, size);
g.setColor$java_awt_Color(border);
g.drawRect$I$I$I$I(x, yy, size, size);
if (state === "green") {
g.setColor$java_awt_Color(border);
g.drawLine$I$I$I$I(x + 2, yy + (size/2|0), x + (size/2|0), yy + size - 2);
g.drawLine$I$I$I$I(x + (size/2|0), yy + size - 2, x + size - 2, yy + 2);
}
g.setColor$java_awt_Color($I$(3).lightGray);
g.drawLine$I$I$I$I(width - 1, y, width - 1, y + charHeight);
g.setColor$java_awt_Color(textColor);
}
});

Clazz.newMeth(C$, 'drawIdString$java_awt_Graphics2D$Z$jalview_datamodel_SequenceI$I$I$I', function (g, hiddenRows, s, i, starty, ypos) {
var checkboxWidth=this.phgoCheckboxColumnWidth$();
var xPos=checkboxWidth;
var panelWidth=this.getWidth$();
var charHeight=this.av.getCharHeight$();
var y=((i - starty) * charHeight) + ypos;
var string=this.phgoDisplayLabel$jalview_datamodel_SequenceI$I(s, i);
if ((this.searchResults != null ) && this.searchResults.contains$O(s) ) {
g.setColor$java_awt_Color($I$(3).black);
g.fillRect$I$I$I$I(0, y, this.getWidth$(), charHeight);
g.setColor$java_awt_Color($I$(3).white);
} else if ((this.av.getSelectionGroup$() != null ) && this.av.getSelectionGroup$().getSequences$java_util_Map(null).contains$O(s) ) {
g.setColor$java_awt_Color($I$(3).lightGray);
g.fillRect$I$I$I$I(0, y, this.getWidth$(), charHeight);
g.setColor$java_awt_Color($I$(3).white);
} else {
g.setColor$java_awt_Color(this.av.getSequenceColour$jalview_datamodel_SequenceI(s));
g.fillRect$I$I$I$I(0, y, this.getWidth$(), charHeight);
g.setColor$java_awt_Color($I$(3).black);
}g.drawString$S$I$I(string, xPos, (((i - starty + 1) * charHeight) + ypos) - ((charHeight/5|0)));
this.drawPHgoCheckbox$java_awt_Graphics2D$jalview_datamodel_SequenceI$I$I$I$I$java_awt_Color(g, s, i, y, charHeight, checkboxWidth, g.getColor$());
if (hiddenRows) {
this.drawMarker$java_awt_Graphics2D$jalview_gui_AlignViewport$I$I$I(g, this.av, i, starty, ypos);
}});

Clazz.newMeth(C$, 'fastPaint$I', function (vertical) {
if (this.image == null  || this.av.getWrapAlignment$() ) {
this.repaint$();
return;
}var ranges=this.av.getRanges$();
var gg=this.image.createGraphics$();
gg.copyArea$I$I$I$I$I$I(0, 0, this.getWidth$(), this.imgHeight, 0, -vertical * this.av.getCharHeight$());
var ss=ranges.getStartSeq$();
var es=ranges.getEndSeq$();
var transY=0;
if (vertical > 0) {
ss=es - vertical;
if (ss < ranges.getStartSeq$()) {
ss=ranges.getStartSeq$();
} else {
transY=this.imgHeight - ((vertical + 1) * this.av.getCharHeight$());
}} else if (vertical < 0) {
es=ss - vertical;
if (es > ranges.getEndSeq$()) {
es=ranges.getEndSeq$();
}}gg.translate$I$I(0, transY);
this.drawIds$java_awt_Graphics2D$jalview_gui_AlignViewport$I$I$java_util_List(gg, this.av, ss, es, this.searchResults);
gg.translate$I$I(0, -transY);
gg.dispose$();
this.fastPaint=true;
this.av.getAlignPanel$().repaint$();
});

Clazz.newMeth(C$, 'paintComponent$java_awt_Graphics', function (g) {
g.setColor$java_awt_Color($I$(3).white);
g.fillRect$I$I$I$I(0, 0, this.getWidth$(), this.getHeight$());
if (this.fastPaint && this.image != null ) {
this.fastPaint=false;
g.drawImage$java_awt_Image$I$I$java_awt_image_ImageObserver(this.image, 0, 0, this);
return;
}this.fastPaint=false;
var oldHeight=this.imgHeight;
this.imgHeight=this.getHeight$();
this.imgHeight-=(this.imgHeight % this.av.getCharHeight$());
if (this.imgHeight < 1) {
return;
}if (this.image == null  || oldHeight != this.imgHeight || this.image.getWidth$java_awt_image_ImageObserver(this) != this.getWidth$() ) {
this.image=Clazz.new_($I$(4).c$$I$I$I,[this.getWidth$(), this.imgHeight, 1]);
}var gg=this.image.createGraphics$();
gg.setColor$java_awt_Color($I$(3).white);
gg.fillRect$I$I$I$I(0, 0, this.getWidth$(), this.imgHeight);
this.drawIds$java_awt_Graphics2D$jalview_gui_AlignViewport$I$I$java_util_List(gg, this.av, this.av.getRanges$().getStartSeq$(), this.av.getRanges$().getEndSeq$(), this.searchResults);
gg.dispose$();
g.drawImage$java_awt_Image$I$I$java_awt_image_ImageObserver(this.image, 0, 0, this);
});

Clazz.newMeth(C$, 'drawIds$java_awt_Graphics2D$jalview_gui_AlignViewport$I$I$java_util_List', function (g, alignViewport, startSeq, endSeq, selection) {
var font=alignViewport.getFont$();
if (alignViewport.isSeqNameItalics$()) {
this.setIdfont$java_awt_Font(Clazz.new_($I$(5).c$$S$I$I,[font.getName$(), 2, font.getSize$()]));
} else {
this.setIdfont$java_awt_Font(font);
}g.setFont$java_awt_Font(this.getIdfont$());
var fm=g.getFontMetrics$();
if (alignViewport.antiAlias) {
g.setRenderingHint$java_awt_RenderingHints_Key$O($I$(6).KEY_ANTIALIASING, $I$(6).VALUE_ANTIALIAS_ON);
}var currentColor=$I$(3).white;
var currentTextColor=$I$(3).black;
var hasHiddenRows=alignViewport.hasHiddenRows$();
if (alignViewport.getWrapAlignment$()) {
this.drawIdsWrapped$java_awt_Graphics2D$jalview_gui_AlignViewport$I$I(g, alignViewport, startSeq, this.getHeight$());
return;
}var panelWidth=this.getWidth$();
var checkboxWidth=this.phgoCheckboxColumnWidth$();
var xPos=checkboxWidth;
for (var i=startSeq; i <= endSeq; i++) {
var sequence=alignViewport.getAlignment$().getSequenceAt$I(i);
if (sequence == null ) {
continue;
}if (hasHiddenRows || alignViewport.isDisplayReferenceSeq$() ) {
g.setFont$java_awt_Font(p$1.getHiddenFont$jalview_datamodel_SequenceI$jalview_gui_AlignViewport.apply(this, [sequence, alignViewport]));
}if (selection != null  && selection.contains$O(sequence) ) {
currentColor=$I$(3).black;
currentTextColor=$I$(3).white;
} else if ((alignViewport.getSelectionGroup$() != null ) && alignViewport.getSelectionGroup$().getSequences$java_util_Map(null).contains$O(sequence) ) {
currentColor=$I$(3).lightGray;
currentTextColor=$I$(3).black;
} else {
currentColor=alignViewport.getSequenceColour$jalview_datamodel_SequenceI(sequence);
currentTextColor=$I$(3).black;
}g.setColor$java_awt_Color(currentColor);
var charHeight=alignViewport.getCharHeight$();
var y=(i - startSeq) * charHeight;
g.fillRect$I$I$I$I(0, y, this.getWidth$(), charHeight);
g.setColor$java_awt_Color(currentTextColor);
var string=this.phgoDisplayLabel$jalview_datamodel_SequenceI$I(sequence, i);
g.drawString$S$I$I(string, xPos, (((i - startSeq) * charHeight) + charHeight) - ((charHeight/5|0)));
this.drawPHgoCheckbox$java_awt_Graphics2D$jalview_datamodel_SequenceI$I$I$I$I$java_awt_Color(g, sequence, i, y, charHeight, checkboxWidth, g.getColor$());
if (hasHiddenRows) {
this.drawMarker$java_awt_Graphics2D$jalview_gui_AlignViewport$I$I$I(g, alignViewport, i, startSeq, 0);
}}
});

Clazz.newMeth(C$, 'drawIdsWrapped$java_awt_Graphics2D$jalview_gui_AlignViewport$I$I', function (g, alignViewport, startSeq, pageHeight) {
var alignmentWidth=alignViewport.getAlignment$().getWidth$();
var alheight=alignViewport.getAlignment$().getHeight$();
var annotationHeight=0;
var labels=null;
if (alignViewport.isShowAnnotation$()) {
if (this.ap == null ) {
this.ap=Clazz.new_($I$(7).c$$jalview_gui_AlignViewport,[alignViewport]);
}annotationHeight=this.ap.adjustPanelHeight$();
labels=Clazz.new_($I$(8).c$$jalview_gui_AlignViewport,[alignViewport]);
}var charHeight=alignViewport.getCharHeight$();
var hgap=charHeight;
if (alignViewport.getScaleAboveWrapped$()) {
hgap+=charHeight;
}var cHeight=alheight * charHeight + hgap + annotationHeight;
var ranges=alignViewport.getRanges$();
var rowSize=ranges.getViewportWidth$();
var hasHiddenRows=alignViewport.hasHiddenRows$();
var ypos=hgap;
var rowStartRes=ranges.getStartRes$();
while ((ypos <= pageHeight) && (rowStartRes < alignmentWidth) ){
for (var i=startSeq; i < alheight; i++) {
var s=alignViewport.getAlignment$().getSequenceAt$I(i);
if (hasHiddenRows || alignViewport.isDisplayReferenceSeq$() ) {
g.setFont$java_awt_Font(p$1.getHiddenFont$jalview_datamodel_SequenceI$jalview_gui_AlignViewport.apply(this, [s, alignViewport]));
} else {
g.setFont$java_awt_Font(this.getIdfont$());
}this.drawIdString$java_awt_Graphics2D$Z$jalview_datamodel_SequenceI$I$I$I(g, hasHiddenRows, s, i, 0, ypos);
}
if (labels != null  && alignViewport.isShowAnnotation$() ) {
g.translate$I$I(0, ypos + (alheight * charHeight));
labels.drawComponent$java_awt_Graphics$I(g, this.getWidth$());
g.translate$I$I(0, -ypos - (alheight * charHeight));
}ypos+=cHeight;
rowStartRes+=rowSize;
}
});

Clazz.newMeth(C$, 'drawMarker$java_awt_Graphics2D$jalview_gui_AlignViewport$I$I$I', function (g, alignViewport, seqIndex, starty, yoffset) {
var hseqs=alignViewport.getAlignment$().getHiddenSequences$().hiddenSequences;
var hSize=hseqs.length;
var hiddenIndex=seqIndex;
var lastIndex=seqIndex - 1;
var nextIndex=seqIndex + 1;
for (var j=0; j < hSize; j++) {
if (hseqs[j] != null ) {
if (j - 1 < hiddenIndex) {
hiddenIndex++;
}if (j - 1 < lastIndex) {
lastIndex++;
}if (j - 1 < nextIndex) {
nextIndex++;
}}}
var below=(hiddenIndex > lastIndex + 1);
var above=(nextIndex > hiddenIndex + 1);
g.setColor$java_awt_Color($I$(3).blue);
var charHeight=this.av.getCharHeight$();
var xPoints=Clazz.array(Integer.TYPE, -1, [this.getWidth$() - charHeight, this.getWidth$() - charHeight, this.getWidth$()]);
var yShift=seqIndex - starty;
if (below) {
var yPoints=Clazz.array(Integer.TYPE, -1, [yShift * charHeight + yoffset, yShift * charHeight + yoffset + (charHeight/4|0), yShift * charHeight + yoffset]);
g.fillPolygon$IA$IA$I(xPoints, yPoints, 3);
}if (above) {
yShift++;
var yPoints=Clazz.array(Integer.TYPE, -1, [yShift * charHeight + yoffset, yShift * charHeight + yoffset - (charHeight/4|0), yShift * charHeight + yoffset]);
g.fillPolygon$IA$IA$I(xPoints, yPoints, 3);
}});

Clazz.newMeth(C$, 'getHiddenFont$jalview_datamodel_SequenceI$jalview_gui_AlignViewport', function (seq, alignViewport) {
if (this.av.isReferenceSeq$jalview_datamodel_SequenceI(seq) || this.av.isHiddenRepSequence$jalview_datamodel_SequenceI(seq) ) {
return Clazz.new_($I$(5).c$$S$I$I,[this.av.getFont$().getName$(), 1, this.av.getFont$().getSize$()]);
}return this.getIdfont$();
}, p$1);

Clazz.newMeth(C$, 'setHighlighted$java_util_List', function (list) {
this.searchResults=list;
this.repaint$();
});

Clazz.newMeth(C$, 'getIdfont$', function () {
return this.idfont;
});

Clazz.newMeth(C$, 'setIdfont$java_awt_Font', function (idfont) {
this.idfont=idfont;
});

Clazz.newMeth(C$, ['propertyChange$java_beans_PropertyChangeEvent','propertyChange$'], function (evt) {
var propertyName=evt.getPropertyName$();
if (propertyName.equals$O("startseq") || (this.av.getWrapAlignment$() && propertyName.equals$O("startres") ) ) {
this.fastPaint$I(((evt.getNewValue$()).objectValue$()|0) - ((evt.getOldValue$()).objectValue$()|0));
} else if (propertyName.equals$O("startresandseq")) {
this.fastPaint$I((evt.getNewValue$())[1] - (evt.getOldValue$())[1]);
} else if (propertyName.equals$O("move_viewport")) {
this.repaint$();
}});

Clazz.newMeth(C$);
})();
;Clazz.setTVer('3.2.4.07');//Created 2019-05-24 12:54:12 Java2ScriptVisitor version 3.2.4.07 net.sf.j2s.core.jar version 3.2.4.07
