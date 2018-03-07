package main

import (
	"testing"
)

// most of these were found via google searches
//  site:github.com FILENAME
//
func TestLicenseFiles(t *testing.T) {
	var testcases = []struct {
		filename string
		license  bool
		legal    bool
	}{
		{"license", true, true},
		{"License", true, true},
		{"LICENSE.md", true, true},
		{"LICENSE.rst", true, true},
		{"LICENSE.txt", true, true},
		{"licence", true, true},
		{"LICENCE.broadcom", true, true},
		{"LICENCE.md", true, true},
		{"copying", true, true},
		{"COPYING.txt", true, true},
		{"unlicense", true, true},
		{"copyright", true, true},
		{"COPYRIGHT.txt", true, true},
		{"copyleft", true, true},
		{"COPYLEFT.txt", true, true},
		{"copyleft.txt", true, true},
		{"Copyleft.txt", true, true},
		{"copyleft-next-0.2.1.txt", true, true},
		{"legal", false, true},
		{"notice", false, true},
		{"NOTICE", false, true},
		{"disclaimer", false, true},
		{"patent", false, true},
		{"patents", false, true},
		{"third-party", false, true},
		{"thirdparty", false, true},
		{"thirdparty.txt", false, true},
		{"license-ThirdParty.txt", true, true},
		{"LICENSE-ThirdParty.txt", true, true},
		{"THIRDPARTY.md", false, true},
		{"third-party.md", false, true},
		{"THIRD-PARTY.txt", false, true},
		{"extensions-third-party.md", false, true},
		{"ThirdParty.md", false, true},
		{"third-party-licenses.md", false, true},
		{"0070-01-01-third-party.md", false, true},
		{"LEGAL.txt", false, true},
		{"legal.txt", false, true},
		{"Legal.md", false, true},
		{"LEGAL.md", false, true},
		{"legal.rst", false, true},
		{"Legal.rtf", false, true},
		{"legal.rtf", false, true},
		{"PATENTS.TXT", false, true},
		{"ttf-PATENTS.txt", false, true},
		{"patents.txt", false, true},
		{"INRIA-DISCLAIMER.txt", false, true},

		{"MPL-2.0-no-copyleft-exception.txt", false, false},
	}

	for pos, tt := range testcases {
		license := IsLicenseFile(tt.filename)
		if tt.license != license {
			if license {
				t.Errorf("%d/file %q is not marked as license", pos, tt.filename)
			} else {
				t.Errorf("%d/file %q was marked incorrectly as a license", pos, tt.filename)
			}
		}

		legal := IsLegalFile(tt.filename)
		if tt.legal != legal {
			if legal {
				t.Errorf("%d/File %q is not marked as legal file", pos, tt.filename)
			} else {
				t.Errorf("%d/File %q was marked incorrectly as a legal file", pos, tt.filename)
			}
		}
	}
}
