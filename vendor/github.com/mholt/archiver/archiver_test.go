package archiver

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestArchiver(t *testing.T) {
	for name, ar := range SupportedFormats {
		name, ar := name, ar
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// skip RAR for now
			if _, ok := ar.(rarFormat); ok {
				t.Skip("not supported")
			}
			testWriteRead(t, name, ar)
			testMakeOpen(t, name, ar)
		})
	}
}

// testWriteRead performs a symmetric test by using ar.Write to generate an archive
// from the test corpus, then using ar.Read to extract the archive and comparing
// the contents to ensure they are equal.
func testWriteRead(t *testing.T, name string, ar Archiver) {
	buf := new(bytes.Buffer)
	tmp, err := ioutil.TempDir("", "archiver")
	if err != nil {
		t.Fatalf("[%s] %v", name, err)
	}
	defer os.RemoveAll(tmp)

	// Test creating archive
	err = ar.Write(buf, []string{"testdata"})
	if err != nil {
		t.Fatalf("[%s] writing archive: didn't expect an error, but got: %v", name, err)
	}

	// Test extracting archive
	err = ar.Read(buf, tmp)
	if err != nil {
		t.Fatalf("[%s] reading archive: didn't expect an error, but got: %v", name, err)
	}

	// Check that what was extracted is what was compressed
	symmetricTest(t, name, tmp)
}

// testMakeOpen performs a symmetric test by using ar.Make to make an archive
// from the test corpus, then using ar.Open to open the archive and comparing
// the contents to ensure they are equal.
func testMakeOpen(t *testing.T, name string, ar Archiver) {
	tmp, err := ioutil.TempDir("", "archiver")
	if err != nil {
		t.Fatalf("[%s] %v", name, err)
	}
	defer os.RemoveAll(tmp)

	// Test creating archive
	outfile := filepath.Join(tmp, "test-"+name)
	err = ar.Make(outfile, []string{"testdata"})
	if err != nil {
		t.Fatalf("[%s] making archive: didn't expect an error, but got: %v", name, err)
	}

	if !ar.Match(outfile) {
		t.Fatalf("[%s] identifying format should be 'true', but got 'false'", name)
	}

	// Test extracting archive
	dest := filepath.Join(tmp, "extraction_test")
	os.Mkdir(dest, 0755)
	err = ar.Open(outfile, dest)
	if err != nil {
		t.Fatalf("[%s] extracting archive [%s -> %s]: didn't expect an error, but got: %v", name, outfile, dest, err)
	}

	// Check that what was extracted is what was compressed
	symmetricTest(t, name, dest)
}

// symmetricTest compares the contents of a destination directory to the contents
// of the test corpus and tests that they are equal.
func symmetricTest(t *testing.T, name, dest string) {
	var expectedFileCount int
	filepath.Walk("testdata", func(fpath string, info os.FileInfo, err error) error {
		expectedFileCount++
		return nil
	})

	// If outputs equals inputs, we're good; traverse output files
	// and compare file names, file contents, and file count.
	var actualFileCount int
	filepath.Walk(dest, func(fpath string, info os.FileInfo, err error) error {
		if fpath == dest {
			return nil
		}
		actualFileCount++

		origPath, err := filepath.Rel(dest, fpath)
		if err != nil {
			t.Fatalf("[%s] %s: Error inducing original file path: %v", name, fpath, err)
		}

		if info.IsDir() {
			// stat dir instead of read file
			_, err = os.Stat(origPath)
			if err != nil {
				t.Fatalf("[%s] %s: Couldn't stat original directory (%s): %v", name,
					fpath, origPath, err)
			}
			return nil
		}

		expectedFileInfo, err := os.Stat(origPath)
		if err != nil {
			t.Fatalf("[%s] %s: Error obtaining original file info: %v", name, fpath, err)
		}
		expected, err := ioutil.ReadFile(origPath)
		if err != nil {
			t.Fatalf("[%s] %s: Couldn't open original file (%s) from disk: %v", name,
				fpath, origPath, err)
		}

		actualFileInfo, err := os.Stat(fpath)
		if err != nil {
			t.Fatalf("[%s] %s: Error obtaining actual file info: %v", name, fpath, err)
		}
		actual, err := ioutil.ReadFile(fpath)
		if err != nil {
			t.Fatalf("[%s] %s: Couldn't open new file from disk: %v", name, fpath, err)
		}

		if actualFileInfo.Mode() != expectedFileInfo.Mode() {
			t.Fatalf("[%s] %s: File mode differed between on disk and compressed", name,
				expectedFileInfo.Mode().String()+" : "+actualFileInfo.Mode().String())
		}
		if !bytes.Equal(expected, actual) {
			t.Fatalf("[%s] %s: File contents differed between on disk and compressed", name, origPath)
		}

		return nil
	})

	if got, want := actualFileCount, expectedFileCount; got != want {
		t.Fatalf("[%s] Expected %d resulting files, got %d", name, want, got)
	}
}

func BenchmarkMake(b *testing.B) {
	tmp, err := ioutil.TempDir("", "archiver")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	for name, ar := range SupportedFormats {
		name, ar := name, ar
		b.Run(name, func(b *testing.B) {
			// skip RAR for now
			if _, ok := ar.(rarFormat); ok {
				b.Skip("not supported")
			}
			outfile := filepath.Join(tmp, "benchMake-"+name)
			for i := 0; i < b.N; i++ {
				err = ar.Make(outfile, []string{"testdata"})
				if err != nil {
					b.Fatalf("making archive: didn't expect an error, but got: %v", err)
				}
			}
		})
	}
}

func BenchmarkOpen(b *testing.B) {
	tmp, err := ioutil.TempDir("", "archiver")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	for name, ar := range SupportedFormats {
		name, ar := name, ar
		b.Run(name, func(b *testing.B) {
			// skip RAR for now
			if _, ok := ar.(rarFormat); ok {
				b.Skip("not supported")
			}
			// prepare a archive
			outfile := filepath.Join(tmp, "benchMake-"+name)
			err = ar.Make(outfile, []string{"testdata"})
			if err != nil {
				b.Fatalf("open archive: didn't expect an error, but got: %v", err)
			}
			// prepare extraction destination
			dest := filepath.Join(tmp, "extraction_test")
			os.Mkdir(dest, 0755)

			// let's go
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				err = ar.Open(outfile, dest)
				if err != nil {
					b.Fatalf("open archive: didn't expect an error, but got: %v", err)
				}
			}
		})
	}
}
