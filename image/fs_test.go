package image

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/distribution/digest"
)

func TestFSGetSet(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "images-fs-store")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	fs, err := NewFSStoreBackend(tmpdir)
	if err != nil {
		t.Fatal(err)
	}

	testGetSet(t, fs)
}

func TestFSGetInvalidData(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "images-fs-store")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	fs, err := NewFSStoreBackend(tmpdir)
	if err != nil {
		t.Fatal(err)
	}

	id, err := fs.Set([]byte("foobar"))
	if err != nil {
		t.Fatal(err)
	}

	dgst := digest.Digest(id)

	if err := ioutil.WriteFile(filepath.Join(tmpdir, contentDirName, string(dgst.Algorithm()), dgst.Hex()), []byte("foobar2"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err = fs.Get(id)
	if err == nil {
		t.Fatal("Expected get to fail after data modification.")
	}
}

func TestFSInvalidSet(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "images-fs-store")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	fs, err := NewFSStoreBackend(tmpdir)
	if err != nil {
		t.Fatal(err)
	}

	id, err := digest.FromBytes([]byte("foobar"))
	if err != nil {
		t.Fatal(err)
	}
	err = os.Mkdir(filepath.Join(tmpdir, contentDirName, string(id.Algorithm()), id.Hex()), 0700)
	if err != nil {
		t.Fatal(err)
	}

	_, err = fs.Set([]byte("foobar"))
	if err == nil {
		t.Fatal("Expecting error from invalid filesystem data.")
	}
}

func TestFSInvalidRoot(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "images-fs-store")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	tcases := []struct {
		root, invalidFile string
	}{
		{"root", "root"},
		{"root", "root/content"},
		{"root", "root/metadata"},
	}

	for _, tc := range tcases {
		root := filepath.Join(tmpdir, tc.root)
		filePath := filepath.Join(tmpdir, tc.invalidFile)
		err := os.MkdirAll(filepath.Dir(filePath), 0700)
		if err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(filePath)
		if err != nil {
			t.Fatal(err)
		}
		f.Close()

		_, err = NewFSStoreBackend(root)
		if err == nil {
			t.Fatalf("Expected error from root %q and invlid file %q", tc.root, tc.invalidFile)
		}

		os.RemoveAll(root)
	}

}

func testMetadataGetSet(t *testing.T, store StoreBackend) {
	id, err := store.Set([]byte("foo"))
	if err != nil {
		t.Fatal(err)
	}
	id2, err := store.Set([]byte("bar"))
	if err != nil {
		t.Fatal(err)
	}

	tcases := []struct {
		id    ID
		key   string
		value []byte
	}{
		{id, "tkey", []byte("tval1")},
		{id, "tkey2", []byte("tval2")},
		{id2, "tkey", []byte("tval3")},
	}

	for _, tc := range tcases {
		err = store.SetMetadata(tc.id, tc.key, tc.value)
		if err != nil {
			t.Fatal(err)
		}

		actual, err := store.GetMetadata(tc.id, tc.key)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Compare(actual, tc.value) != 0 {
			t.Fatalf("Metadata expected %q, got %q", tc.value, actual)
		}
	}

	_, err = store.GetMetadata(id2, "tkey2")
	if err == nil {
		t.Fatal("Expected error for getting metadata for unknown key")
	}

	id3, err := digest.FromBytes([]byte("baz"))
	if err != nil {
		t.Fatal(err)
	}

	err = store.SetMetadata(ID(id3), "tkey", []byte("tval"))
	if err == nil {
		t.Fatal("Expected error for setting metadata for unknown ID.")
	}

	_, err = store.GetMetadata(ID(id3), "tkey")
	if err == nil {
		t.Fatal("Expected error for getting metadata for unknown ID.")
	}
}

func TestFSMetadataGetSet(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "images-fs-store")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	fs, err := NewFSStoreBackend(tmpdir)
	if err != nil {
		t.Fatal(err)
	}

	testMetadataGetSet(t, fs)
}

func TestFSDelete(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "images-fs-store")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	fs, err := NewFSStoreBackend(tmpdir)
	if err != nil {
		t.Fatal(err)
	}

	testDelete(t, fs)
}

func TestFSWalker(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "images-fs-store")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	fs, err := NewFSStoreBackend(tmpdir)
	if err != nil {
		t.Fatal(err)
	}

	testWalker(t, fs)
}

func TestFSInvalidWalker(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "images-fs-store")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	fs, err := NewFSStoreBackend(tmpdir)
	if err != nil {
		t.Fatal(err)
	}

	fooID, err := fs.Set([]byte("foo"))
	if err != nil {
		t.Fatal(err)
	}

	if err := ioutil.WriteFile(filepath.Join(tmpdir, contentDirName, "sha256/foobar"), []byte("foobar"), 0600); err != nil {
		t.Fatal(err)
	}

	n := 0
	err = fs.Walk(func(id ID) error {
		if id != fooID {
			t.Fatalf("Invalid walker ID %q, expected %q", id, fooID)
		}
		n++
		return nil
	})
	if err != nil {
		t.Fatalf("Invalid data should not have caused walker error, got %v", err)
	}
	if n != 1 {
		t.Fatalf("Expected 1 walk initialization, got %d", n)
	}
}

func testGetSet(t *testing.T, store StoreBackend) {
	type tcase struct {
		input    []byte
		expected ID
	}
	tcases := []tcase{
		{[]byte("foobar"), ID("sha256:c3ab8ff13720e8ad9047dd39466b3c8974e592c2fa383d4a3960714caef0c4f2")},
	}

	randomInput := make([]byte, 8*1024)
	_, err := rand.Read(randomInput)
	if err != nil {
		t.Fatal(err)
	}
	// skipping use of digest pkg because its used by the implementation
	h := sha256.New()
	_, err = h.Write(randomInput)
	if err != nil {
		t.Fatal(err)
	}
	tcases = append(tcases, tcase{
		input:    randomInput,
		expected: ID("sha256:" + hex.EncodeToString(h.Sum(nil))),
	})

	for _, tc := range tcases {
		id, err := store.Set([]byte(tc.input))
		if err != nil {
			t.Fatal(err)
		}
		if id != tc.expected {
			t.Fatalf("Expected ID %q, got %q", tc.expected, id)
		}
	}

	for _, emptyData := range [][]byte{nil, {}} {
		_, err := store.Set(emptyData)
		if err == nil {
			t.Fatal("Expected error for nil input.")
		}
	}

	for _, tc := range tcases {
		data, err := store.Get(tc.expected)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Compare(data, tc.input) != 0 {
			t.Fatalf("Expected data %q, got %q", tc.input, data)
		}
	}

	for _, key := range []ID{"foobar:abc", "sha256:abc", "sha256:c3ab8ff13720e8ad9047dd39466b3c8974e592c2fa383d4a3960714caef0c4f2a"} {
		_, err := store.Get(key)
		if err == nil {
			t.Fatalf("Expected error for ID %q.", key)
		}
	}

}

func testDelete(t *testing.T, store StoreBackend) {
	id, err := store.Set([]byte("foo"))
	if err != nil {
		t.Fatal(err)
	}
	id2, err := store.Set([]byte("bar"))
	if err != nil {
		t.Fatal(err)
	}

	err = store.Delete(id)
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.Get(id)
	if err == nil {
		t.Fatalf("Expected getting deleted item %q to fail", id)
	}
	_, err = store.Get(id2)
	if err != nil {
		t.Fatal(err)
	}

	err = store.Delete(id2)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Get(id2)
	if err == nil {
		t.Fatalf("Expected getting deleted item %q to fail", id2)
	}
}

func testWalker(t *testing.T, store StoreBackend) {
	id, err := store.Set([]byte("foo"))
	if err != nil {
		t.Fatal(err)
	}
	id2, err := store.Set([]byte("bar"))
	if err != nil {
		t.Fatal(err)
	}

	tcases := make(map[ID]struct{})
	tcases[id] = struct{}{}
	tcases[id2] = struct{}{}
	n := 0
	err = store.Walk(func(id ID) error {
		delete(tcases, id)
		n++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if n != 2 {
		t.Fatalf("Expected 2 walk initializations, got %d", n)
	}
	if len(tcases) != 0 {
		t.Fatalf("Expected empty unwalked set, got %+v", tcases)
	}

	// stop on error
	tcases = make(map[ID]struct{})
	tcases[id] = struct{}{}
	err = store.Walk(func(id ID) error {
		return errors.New("")
	})
	if err == nil {
		t.Fatalf("Exected error from walker.")
	}
}
