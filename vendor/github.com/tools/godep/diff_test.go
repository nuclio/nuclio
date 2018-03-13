package main

import (
	"fmt"
	"strings"
	"testing"
)

var (
	d1 = fmt.Sprintf(`--- Godeps/Godeps.json
+++ $GOPATH
@@ -1,13 +1,13 @@
 {
        "ImportPath": "C",
        "GoVersion": "go1.2",
        "GodepVersion": "v%d",
        "Deps": [
                {
                        "ImportPath": "D101",
-                       "Comment": "D202",
+                       "Comment": "D303",
                        "Rev": ""
                }
        ]
 }
`, version)

	d2 = fmt.Sprintf(`--- Godeps/Godeps.json
+++ $GOPATH
@@ -1,13 +1,18 @@
 {
        "ImportPath": "C",
        "GoVersion": "go1.2",
        "GodepVersion": "v%d",
        "Deps": [
                {
                        "ImportPath": "D101",
                        "Comment": "D202",
                        "Rev": ""
+               },
+               {
+                       "ImportPath": "D102",
+                       "Comment": "D203",
+                       "Rev": ""
                }
        ]
 }
`, version)
)

var (
	dep1 = Godeps{
		ImportPath: "C",
		GoVersion:  "go1.2",
		Deps: []Dependency{
			{ImportPath: "D101", Comment: "D202"},
		},
	}

	dep2 = Godeps{
		ImportPath: "C",
		GoVersion:  "go1.2",
		Deps: []Dependency{
			{ImportPath: "D101", Comment: "D202"},
		},
	}
)

func TestDiff(t *testing.T) {
	// Equiv Godeps, should yield an empty diff.
	diff, _ := diffStr(&dep1, &dep2)
	if diff != "" {
		t.Errorf("Diff is %v want ''", diff)
	}

	// Test modifications in packages make it to the diff.
	dep2.Deps[0].Comment = "D303"
	diff, _ = diffStr(&dep1, &dep2)
	if !diffsEqual(strings.Fields(diff), strings.Fields(d1)) {
		t.Errorf("Expecting diffs to be equal. Obs <%s>. Exp <%s>", diff, d1)
	}

	// Test additional packages in new Godeps
	dep2.Deps[0].Comment = "D202"
	dep2.Deps = append(dep2.Deps, Dependency{ImportPath: "D102", Comment: "D203"})
	diff, _ = diffStr(&dep1, &dep2)

	if !diffsEqual(strings.Fields(diff), strings.Fields(d2)) {
		t.Errorf("Expecting diffs to be equal. Obs <%v>. Exp <%v>", diff, d2)
	}
}

// diffsEqual asserts that two slices are equivalent.
func diffsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
