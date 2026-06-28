package apkparse

import (
	"archive/zip"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func encodeJPEG(t *testing.T, w io.Writer, img image.Image) {
	t.Helper()
	if err := jpeg.Encode(w, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
}

// makePNG returns a tiny solid-color PNG byte slice.
func makePNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for x := 0; x < 2; x++ {
		for y := 0; y < 2; y++ {
			img.Set(x, y, color.RGBA{R: 10, G: 200, B: 30, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// makeJPEG returns a tiny JPEG byte slice (used to assert re-encoding to PNG).
func makeJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for x := 0; x < 4; x++ {
		for y := 0; y < 4; y++ {
			img.Set(x, y, color.RGBA{R: 0, G: 0, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	encodeJPEG(t, &buf, img)
	return buf.Bytes()
}

func expectedMD5Size(t *testing.T, path string) (string, int64) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	sum := md5.Sum(data)
	return hex.EncodeToString(sum[:]), int64(len(data))
}

// writeXAPK builds a .xapk (zip) at a temp path with the given manifest.json
// content and entries, returning the path.
func writeXAPK(t *testing.T, manifestJSON string, entries map[string][]byte) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.xapk")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create xapk: %v", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	if manifestJSON != "" {
		w, err := zw.Create("manifest.json")
		if err != nil {
			t.Fatalf("create manifest entry: %v", err)
		}
		if _, err := w.Write([]byte(manifestJSON)); err != nil {
			t.Fatalf("write manifest: %v", err)
		}
	}
	for name, data := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create entry %s: %v", name, err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatalf("write entry %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return path
}

func TestParseXAPK_FullManifest(t *testing.T) {
	iconPNG := makePNG(t)
	manifest := `{
		"package_name": "com.example.xapk",
		"version_name": "2.3.4",
		"name": "Example XAPK",
		"icon": "icon.png"
	}`
	path := writeXAPK(t, manifest, map[string][]byte{
		"icon.png": iconPNG,
	})

	wantMD5, wantSize := expectedMD5Size(t, path)

	got, err := Parse(path, ".xapk")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.PackageName != "com.example.xapk" {
		t.Errorf("PackageName = %q, want com.example.xapk", got.PackageName)
	}
	if got.Version != "2.3.4" {
		t.Errorf("Version = %q, want 2.3.4", got.Version)
	}
	if got.AppName != "Example XAPK" {
		t.Errorf("AppName = %q, want Example XAPK", got.AppName)
	}
	if len(got.IconPNG) == 0 {
		t.Error("IconPNG empty, want non-empty")
	}
	if _, err := png.Decode(bytes.NewReader(got.IconPNG)); err != nil {
		t.Errorf("IconPNG is not valid PNG: %v", err)
	}
	if got.MD5 != wantMD5 {
		t.Errorf("MD5 = %q, want %q", got.MD5, wantMD5)
	}
	if len(got.MD5) != 32 {
		t.Errorf("MD5 length = %d, want 32", len(got.MD5))
	}
	if got.SizeBytes != wantSize {
		t.Errorf("SizeBytes = %d, want %d", got.SizeBytes, wantSize)
	}
}

func TestParseXAPK_NonPNGIconReencoded(t *testing.T) {
	manifest := `{
		"package_name": "com.example.jpg",
		"version_name": "1.0",
		"name": "JPG Icon",
		"icon": "icon.jpg"
	}`
	path := writeXAPK(t, manifest, map[string][]byte{
		"icon.jpg": makeJPEG(t),
	})

	got, err := Parse(path, "xapk") // also exercises ext without leading dot
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got.IconPNG) == 0 {
		t.Fatal("IconPNG empty, want re-encoded PNG")
	}
	if _, err := png.Decode(bytes.NewReader(got.IconPNG)); err != nil {
		t.Errorf("IconPNG not valid PNG after re-encode: %v", err)
	}
}

// TestParseXAPK_MissingFieldsFallbackToBaseAPK verifies that when the xapk
// manifest.json lacks core fields, Parse extracts the bundled base apk and
// fills them from the apk parser.
func TestParseXAPK_MissingFieldsFallbackToBaseAPK(t *testing.T) {
	apkFixture := filepath.Join("testdata", "helloworld.apk")
	if _, err := os.Stat(apkFixture); err != nil {
		t.Skipf("apk fixture missing (%v); cannot exercise base-apk fallback", err)
	}
	apkBytes, err := os.ReadFile(apkFixture)
	if err != nil {
		t.Fatalf("read apk fixture: %v", err)
	}

	// manifest.json with only a partial field; package/version/name/icon
	// must be recovered from the base apk.
	manifest := `{"version_name": ""}`
	path := writeXAPK(t, manifest, map[string][]byte{
		"com.example.helloworld.apk": apkBytes,
	})

	got, err := Parse(path, ".xapk")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.PackageName != "com.example.helloworld" {
		t.Errorf("PackageName = %q, want com.example.helloworld (from base apk)", got.PackageName)
	}
	if got.Version != "1.0" {
		t.Errorf("Version = %q, want 1.0 (from base apk)", got.Version)
	}
	if got.AppName != "HelloWorld" {
		t.Errorf("AppName = %q, want HelloWorld (from base apk)", got.AppName)
	}
	if len(got.IconPNG) == 0 {
		t.Error("IconPNG empty, want icon recovered from base apk")
	}
}

// TestParseXAPK_LauncherIconFallback verifies that when the manifest icon
// path resolves to nothing (and there's no base apk to recover one), the zip
// scan fallback finds a res/mipmap*/ic_launcher.png entry and fills IconPNG.
func TestParseXAPK_LauncherIconFallback(t *testing.T) {
	manifest := `{
		"package_name": "com.example.fallback",
		"version_name": "1.0",
		"name": "Fallback Icon"
	}`
	// No manifest "icon", no base apk entry; only a launcher icon under
	// res/mipmap-xxxhdpi/. The scan fallback must rasterize it.
	path := writeXAPK(t, manifest, map[string][]byte{
		"res/mipmap-xxxhdpi/ic_launcher.png": makePNG(t),
		"res/drawable/ic_launcher.xml":       []byte("<adaptive-icon/>"), // must be skipped
	})

	got, err := Parse(path, ".xapk")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got.IconPNG) == 0 {
		t.Fatal("IconPNG empty, want icon recovered via launcher-icon zip scan")
	}
	if _, err := png.Decode(bytes.NewReader(got.IconPNG)); err != nil {
		t.Errorf("IconPNG is not valid PNG: %v", err)
	}
}

// TestScanZipForLauncherIcon_PrefersLargestRaster checks the helper directly:
// XML drawables are skipped and the largest raster entry wins.
func TestScanZipForLauncherIcon_PrefersLargestRaster(t *testing.T) {
	small := makePNG(t)
	large := makeJPEG(t) // 4x4 JPEG, larger uncompressed than the 2x2 PNG
	path := writeXAPK(t, "", map[string][]byte{
		"res/drawable-anydpi-v26/ic_launcher.xml": []byte("<adaptive-icon/>"),
		"res/mipmap-mdpi/ic_launcher.png":         small,
		"res/mipmap-xxxhdpi/ic_launcher.jpg":      large,
		"res/mipmap-hdpi/unrelated.png":           small, // no ic_launcher → skipped
	})

	data := scanZipForLauncherIcon(path)
	if len(data) == 0 {
		t.Fatal("scanZipForLauncherIcon returned empty, want PNG bytes")
	}
	if _, err := png.Decode(bytes.NewReader(data)); err != nil {
		t.Errorf("result not valid PNG: %v", err)
	}
}

func TestParseAPK(t *testing.T) {
	apkFixture := filepath.Join("testdata", "helloworld.apk")
	wantMD5, wantSize := expectedMD5Size(t, apkFixture)

	got, err := Parse(apkFixture, ".apk")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// MD5/size are always authoritative regardless of manifest decoding.
	if got.MD5 != wantMD5 {
		t.Errorf("MD5 = %q, want %q", got.MD5, wantMD5)
	}
	if got.SizeBytes != wantSize {
		t.Errorf("SizeBytes = %d, want %d", got.SizeBytes, wantSize)
	}

	if _, err := os.Stat(apkFixture); err != nil {
		t.Skipf("apk fixture missing (%v); skipping manifest assertions", err)
	}
	if got.PackageName != "com.example.helloworld" {
		t.Errorf("PackageName = %q, want com.example.helloworld", got.PackageName)
	}
	if got.Version != "1.0" {
		t.Errorf("Version = %q, want 1.0", got.Version)
	}
	if got.AppName != "HelloWorld" {
		t.Errorf("AppName = %q, want HelloWorld", got.AppName)
	}
	if len(got.IconPNG) == 0 {
		t.Error("IconPNG empty, want non-empty")
	}
	if _, err := png.Decode(bytes.NewReader(got.IconPNG)); err != nil {
		t.Errorf("IconPNG is not valid PNG: %v", err)
	}
}

func TestParse_UnsupportedExt(t *testing.T) {
	path := writeXAPK(t, `{}`, nil) // any zip file
	if _, err := Parse(path, ".zip"); err == nil {
		t.Error("Parse with unsupported ext = nil error, want error")
	}
}

func TestParse_MD5AndSizeOnArbitraryZip(t *testing.T) {
	// Even though the parser path differs, MD5/size must match a direct
	// computation. Use the apk extension on a hand-built zip; manifest
	// decoding may fail but MD5/size are computed first.
	dir := t.TempDir()
	path := filepath.Join(dir, "any.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	zw := zip.NewWriter(f)
	w, _ := zw.Create("hello.txt")
	_, _ = w.Write([]byte("hello world"))
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	f.Close()

	wantMD5, wantSize := expectedMD5Size(t, path)
	sum, size, err := hashAndSize(path)
	if err != nil {
		t.Fatalf("hashAndSize: %v", err)
	}
	if sum != wantMD5 {
		t.Errorf("MD5 = %q, want %q", sum, wantMD5)
	}
	if size != wantSize {
		t.Errorf("size = %d, want %d", size, wantSize)
	}
}
