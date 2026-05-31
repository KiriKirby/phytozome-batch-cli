// The contents of this file are subject to the Common Public Attribution License Version 1.0 (CPAL-1.0);
// you may not use this file except in compliance with the License. You may obtain a copy of the License at
// https://opensource.org/license/CPAL-1.0. Software distributed under the License is distributed on an "AS IS"
// basis, WITHOUT WARRANTY OF ANY KIND, either express or implied. The Original Code is phytozome GO. The
// Initial Developer is wangsychn. All portions of the code written by wangsychn are Copyright (c) 2026
// wangsychn. All Rights Reserved. Contributor(s): .

package prompt

import (
	"fmt"
	"strings"

	"github.com/KiriKirby/phytozome-go/internal/model"
	"github.com/KiriKirby/phytozome-go/internal/tui"
)

func (p *Prompter) SnapshotKeywordReviewState(groups []model.KeywordSearchGroup) tui.RowSelectionState {
	return p.rowStates[keywordSelectionCacheKey(groups)]
}

func (p *Prompter) RestoreKeywordReviewState(groups []model.KeywordSearchGroup, selected []bool, state tui.RowSelectionState) {
	stateKey := keywordSelectionCacheKey(groups)
	if len(selected) == countKeywordResultRows(groups) {
		p.keywordSelections[stateKey] = append([]bool(nil), selected...)
	} else {
		delete(p.keywordSelections, stateKey)
	}
	if state.Valid {
		p.rowStates[stateKey] = state
	} else {
		delete(p.rowStates, stateKey)
	}
}

func (p *Prompter) SnapshotBlastRowReviewState(rows []model.BlastResultRow) tui.RowSelectionState {
	return p.rowStates[blastSelectionCacheKey(rows)]
}

func (p *Prompter) RestoreBlastRowReviewState(rows []model.BlastResultRow, selected []bool, filterFlags []bool, filterSettings model.BlastFilterSettings, state tui.RowSelectionState) {
	stateKey := blastSelectionCacheKey(rows)
	if len(selected) == len(rows) {
		p.blastSelections[stateKey] = append([]bool(nil), selected...)
	} else {
		delete(p.blastSelections, stateKey)
	}
	if len(filterFlags) == len(rows) {
		p.blastFilterFlags[stateKey] = append([]bool(nil), filterFlags...)
	} else {
		delete(p.blastFilterFlags, stateKey)
	}
	if state.Valid {
		p.rowStates[stateKey] = state
	} else {
		delete(p.rowStates, stateKey)
	}
	p.blastFilterSettings = filterSettings
}

func (p *Prompter) SnapshotBlastRunsReviewState(runs []BlastRunView) tui.BlastRunSelectionState {
	return p.blastRunStates[blastRunsSelectionCacheKey(runs)]
}

func (p *Prompter) RestoreBlastRunsReviewState(runs []BlastRunView, selectedByRun [][]bool, filterFlagsByRun [][]bool, filterSettings model.BlastFilterSettings, state tui.BlastRunSelectionState) {
	stateKey := blastRunsSelectionCacheKey(runs)
	if boolMatrixMatchesRunRows(runs, selectedByRun) {
		p.blastRunSelected[stateKey] = cloneBoolMatrixPrompt(selectedByRun)
	} else {
		delete(p.blastRunSelected, stateKey)
	}
	if boolMatrixMatchesRunRows(runs, filterFlagsByRun) {
		p.blastRunFilterFlags[stateKey] = cloneBoolMatrixPrompt(filterFlagsByRun)
	} else {
		delete(p.blastRunFilterFlags, stateKey)
	}
	if state.Valid {
		p.blastRunStates[stateKey] = cloneBlastRunSelectionState(state)
	} else {
		delete(p.blastRunStates, stateKey)
	}
	p.blastFilterSettings = filterSettings
}

func keywordSelectionCacheKey(groups []model.KeywordSearchGroup) string {
	rows := flattenPromptKeywordSearchGroups(groups)
	columns, tableRows := buildKeywordSelectionTable(rows)
	return tableStateKey("keyword", columns, tableRows)
}

func blastSelectionCacheKey(rows []model.BlastResultRow) string {
	columns, tableRows := buildBlastSelectionTable(rows)
	return tableStateKey("blast", columns, tableRows)
}

func blastRunsSelectionCacheKey(runs []BlastRunView) string {
	tableKeyParts := make([]string, 0, len(runs))
	for i, run := range runs {
		columns, tableRows := buildBlastSelectionTable(run.Rows)
		tableKeyParts = append(tableKeyParts, tableStateKey(fmt.Sprintf("blast-run-%d", i), columns, tableRows))
	}
	return "blast-runs:" + digestStrings(tableKeyParts)
}

func boolMatrixMatchesRunRows(runs []BlastRunView, values [][]bool) bool {
	if len(values) != len(runs) {
		return false
	}
	for i := range runs {
		if len(values[i]) != len(runs[i].Rows) {
			return false
		}
	}
	return true
}

func cloneBlastRunSelectionState(state tui.BlastRunSelectionState) tui.BlastRunSelectionState {
	state.Tables = append([]tui.BlastRunTableState(nil), state.Tables...)
	return state
}

func flattenPromptKeywordSearchGroups(groups []model.KeywordSearchGroup) []model.KeywordResultRow {
	rows := make([]model.KeywordResultRow, 0)
	for _, group := range groups {
		rows = append(rows, group.Rows...)
	}
	return rows
}

func (p *Prompter) SnapshotCanvasReviewState(key string) tui.BlastRunSelectionState {
	return p.blastRunStates[strings.TrimSpace(key)]
}

func (p *Prompter) SnapshotCanvasTreePanelState(key string) tui.CanvasTreePanelState {
	return p.canvasTreeStates[strings.TrimSpace(key)]
}

func (p *Prompter) RestoreCanvasReviewState(key string, state tui.BlastRunSelectionState) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	if state.Valid {
		p.blastRunStates[key] = cloneBlastRunSelectionState(state)
		return
	}
	delete(p.blastRunStates, key)
}

func (p *Prompter) RestoreCanvasTreePanelState(key string, state tui.CanvasTreePanelState) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	if state.AlignmentParams == nil {
		state.AlignmentParams = map[string]string{}
	}
	if state.TreeParams == nil {
		state.TreeParams = map[string]string{}
	}
	if state.EnabledEver || state.Expanded || state.Focused || state.CurrentControl != 0 || strings.TrimSpace(state.DisplayNameSource) != "" || strings.TrimSpace(state.ConversionTarget) != "" || strings.TrimSpace(state.ConversionAction) != "" || state.ConversionSkipUnselect || strings.TrimSpace(state.AlignmentMethod) != "" || strings.TrimSpace(state.TreeMethod) != "" || len(state.AlignmentParams) > 0 || len(state.TreeParams) > 0 {
		p.canvasTreeStates[key] = state
		return
	}
	delete(p.canvasTreeStates, key)
}
