package aci

import (
	"archive/tar"
	"compress/gzip"
	"io/ioutil"
	"os"
	"testing"
)

func newTestACI() (*os.File, error) {
	tf, err := ioutil.TempFile("", "")
	if err != nil {
		return nil, err
	}

	manifestBody := `{"acKind":"ImageManifest","acVersion":"0.3.0","name":"example.com/app"}`

	gw := gzip.NewWriter(tf)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{
		Name: "manifest",
		Size: int64(len(manifestBody)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return nil, err
	}
	if _, err := tw.Write([]byte(manifestBody)); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}
	return tf, nil
}

func newEmptyTestACI() (*os.File, error) {
	tf, err := ioutil.TempFile("", "")
	if err != nil {
		return nil, err
	}
	gw := gzip.NewWriter(tf)
	tw := tar.NewWriter(gw)
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}
	return tf, nil
}

func TestManifestFromImage(t *testing.T) {
	img, err := newTestACI()
	if err != nil {
		t.Fatalf("newTestACI: unexpected error %v", err)
	}
	defer img.Close()
	defer os.Remove(img.Name())

	im, err := ManifestFromImage(img)
	if err != nil {
		t.Fatalf("ManifestFromImage: unexpected error %v", err)
	}
	if im.Name.String() != "example.com/app" {
		t.Errorf("expected %s, got %s", "example.com/app", im.Name.String())
	}

	emptyImg, err := newEmptyTestACI()
	if err != nil {
		t.Fatalf("newEmptyTestACI: unexpected error %v", err)
	}
	defer emptyImg.Close()
	defer os.Remove(emptyImg.Name())

	im, err = ManifestFromImage(emptyImg)
	if err == nil {
		t.Fatalf("ManifestFromImage: expected error")
	}
}

func TestNewCompressedTarReader(t *testing.T) {
	img, err := newTestACI()
	if err != nil {
		t.Fatalf("newTestACI: unexpected error %v", err)
	}
	defer img.Close()
	defer os.Remove(img.Name())

	cr, err := NewCompressedTarReader(img)
	if err != nil {
		t.Fatalf("NewCompressedTarReader: unexpected error %v", err)
	}

	ftype, err := DetectFileType(cr)
	if err != nil {
		t.Fatalf("DetectFileType: unexpected error %v", err)
	}

	if ftype != TypeText {
		t.Errorf("expected %v, got %v", TypeText, ftype)
	}
}

func TestNewCompressedReader(t *testing.T) {
	img, err := newTestACI()
	if err != nil {
		t.Fatalf("newTestACI: unexpected error %v", err)
	}
	defer img.Close()
	defer os.Remove(img.Name())

	cr, err := NewCompressedReader(img)
	if err != nil {
		t.Fatalf("NewCompressedReader: unexpected error %v", err)
	}

	ftype, err := DetectFileType(cr)
	if err != nil {
		t.Fatalf("DetectFileType: unexpected error %v", err)
	}

	if ftype != TypeTar {
		t.Errorf("expected %v, got %v", TypeTar, ftype)
	}
}
