//////////////////////////////////////////
// archivex_test.go
// Jhonathan Paulo Banczek - 2014
// jpbanczek@gmail.com - jhoonb.com
//////////////////////////////////////////

package archivex

import (
	"fmt"
	"os"
	"path"
	"reflect"
	"testing"
)

type archTest struct {
	addPath     string
	include     bool
	name        string
	filePath    string
	addString   string
	addFileName string
}

type archTypeTest struct {
	tests []archTest
	arch  Archivex
}

func Test_archivex(t *testing.T) {
	dir, _ := os.Getwd()

	// let's clean up the previous results, to be sure that we're not reading from an old result.
	if err := os.RemoveAll(path.Join(dir, "/testresults")); err != nil && !os.IsNotExist(err) {
		t.Fatalf("cannot clean up test results directory: %v", err)
	}

	if err := os.Mkdir(path.Join(dir, "/testresults"), 0777); err != nil && !os.IsExist(err) {
		t.Fatalf("cannot make test results directory: %v", err)
	}

	// All the different tests we want to run with different combinations of input paths and the includeCurrentFolder flag
	tests := []archTest{
		// absolute path
		archTest{dir + "/testfolder/", true, "absTrailInclude", dir + "/LICENSE", "string", "filename"},
		archTest{dir + "/testfolder/", false, "absTrailExclude", dir + "/LICENSE", "string", "filename"},

		// relative path
		archTest{"testfolder/", true, "relTrailInclude", "LICENSE", "string", "filename"},
		archTest{"testfolder/", false, "relTrailExclude", "LICENSE", "string", "filename"},

		// without trailing slashes
		archTest{dir + "/testfolder", true, "absInclude", dir + "/LICENSE", "string", "filename"},
		archTest{dir + "/testfolder", false, "absExclude", dir + "/LICENSE", "string", "filename"},
		archTest{"testfolder", true, "relInclude", "LICENSE", "string", "filename"},
		archTest{"testfolder", false, "relExclude", "LICENSE", "string", "filename"},
	}

	// We want to execute the batch of tests on both Zip and Tar
	typeTests := []archTypeTest{
		archTypeTest{tests, &ZipFile{}},
		archTypeTest{tests, &TarFile{}},
	}

	// Run all tests
	for _, typeTest := range typeTests {
		currentType := reflect.TypeOf(typeTest.arch)
		t.Logf("Running tests for archive type: %s", currentType.Elem())

		for i, test := range typeTest.tests {
			t.Logf("Running %s...", test.name)

			// Create the archive
			filename := fmt.Sprintf("%d_%s_test", i+1, test.name)
			arch := reflect.ValueOf(typeTest.arch).Interface().(Archivex)
			if err := arch.Create(path.Join("testresults", filename)); err != nil {
				t.Fatalf("Error creating '%s': %v", filename, err)
			}

			// Add the files to the archive
			if err := arch.AddAll(test.addPath, test.include); err != nil {
				t.Fatalf("Error doing AddAll with '%s' and includeCurrentFolder = %v: %v", test.addPath, test.include, err)
			}

			// Add a file to the archive
			if err := arch.AddFile(test.filePath); err != nil {
				t.Fatalf("Error doing AddFile with '%s': %v", test.filePath, err)
			}
			//}

			// Add a file to the archive
			if err := arch.Add(test.addFileName, []byte(test.addString)); err != nil {
				t.Fatalf("Error doing Add with '%s', '%s': %v", test.addString, test.addFileName, err)
			}

			// Close the archive
			arch.Close()
		}
	}
}
