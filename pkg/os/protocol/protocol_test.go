package protocol

import (
	"testing"

	"github.com/spf13/afero"
)

func TestURL(t *testing.T) {
	examples := []string{
		"example.txt",            // 本地文件
		"/path/to/some/file",     // 本地文件（Unix风格路径）
		"./path/to/some/file",    // 本地文件（Unix风格路径）
		"http://example.com",     // HTTP URL
		"https://example.com",    // HTTPS URL
		"file:///path/to/file",   // 明确的文件路径
		"C:\\path\\to\\file.txt", // Windows风格本地文件路径
	}

	for _, example := range examples {
		ptl := New(example)

		scheme, value := ptl.Value()

		if scheme != SchemeUnknown {
			t.Logf("Source: %s, Scheme: %s, Value: %s\n", example, scheme, value)
		}
	}
}

func TestMemMap(t *testing.T) {
	fs := afero.NewMemMapFs()
	afs := &afero.Afero{Fs: fs}

	t.Logf("%s", afs.GetTempDir("ss"))

	f, err := afs.TempFile("", "zzzzzz-test")
	if err != nil {
		t.Fatal(err)
		return
	}
	defer func() { _ = f.Close() }()

	_, err = f.WriteString("ffff")
	if err != nil {
		t.Fatal(err)
		return
	}

	t.Logf("%s", f.Name())
}
