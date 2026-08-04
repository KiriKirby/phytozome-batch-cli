// The contents of this file are subject to the Common Public Attribution License Version 1.0 (CPAL-1.0);
// you may not use this file except in compliance with the License. You may obtain a copy of the License at
// https://opensource.org/license.html. Software distributed under the License is distributed on an "AS IS"
// basis, WITHOUT WARRANTY OF ANY KIND, either express or implied. The Original Code is phytozome GO. The
// Initial Developer is wangsychn. All portions of the code written by wangsychn are Copyright (c) 2026
// wangsychn. All Rights Reserved. Contributor(s): .

package workflow

import (
	"slices"
	"testing"

	"github.com/KiriKirby/phytozome-go/internal/model"
)

func TestKeywordReportMixedPLAZAAndNCBIRetainsSourceDatabase(t *testing.T) {
	rows := []model.KeywordResultRow{
		{SourceDatabase: "ncbi", SearchType: "protein"},
		{SourceDatabase: "plaza", SearchType: "PLAZA Gene locus priority", TranscriptID: "AT1G51680.1"},
	}
	headers := keywordReportHeaders(rows)
	if !slices.Contains(headers, "source_database") || !slices.Contains(headers, "transcript") {
		t.Fatalf("mixed PLAZA/NCBI report headers missing shared fields: %#v", headers)
	}
	if got := keywordReportCellValue("source_database", rows[1], 1); got != "plaza" {
		t.Fatalf("source_database = %q, want plaza", got)
	}
}
