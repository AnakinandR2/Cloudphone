// Package apkparse extracts metadata (package name, version, app name, icon),
// MD5 and byte size from Android application packages (.apk) and bundles
// (.xapk). It uses github.com/shogo82148/androidbinary for AXML/resource
// decoding rather than re-implementing the AXML format.
package apkparse

import (
	"archive/zip"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/shogo82148/androidbinary"
	"github.com/shogo82148/androidbinary/apk"

	// Register decoders so image.Decode can read icons that ship as
	// JPEG/PNG/WebP. Modern launcher icons are commonly WebP, so we pull in
	// the x/image/webp decoder; any still-undecodable icon is tolerated as
	// "no icon" rather than failing the whole parse.
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/webp"
)

// ParsedApp holds the metadata extracted from an apk/xapk package.
//
// IconPNG is always PNG-encoded (re-encoded if the source icon was a
// different format). It may be empty when the package has no resolvable
// icon; that is not treated as an error.
type ParsedApp struct {
	PackageName string
	Version     string
	AppName     string
	IconPNG     []byte // nil/empty when no icon could be resolved
	MD5         string // lowercase 32-hex
	SizeBytes   int64
}

// Parse reads the file at path and extracts metadata. ext selects the
// parsing strategy; it is matched case-insensitively and a leading dot is
// optional (".apk", "apk", ".XAPK" all work). When ext is empty the file
// extension of path is used.
//
// MD5 and SizeBytes are always computed by streaming the whole file once,
// independent of the metadata extraction, so they are authoritative even
// when manifest parsing partially fails.
func Parse(path string, ext string) (ParsedApp, error) {
	res := ParsedApp{}

	sum, size, err := hashAndSize(path)
	if err != nil {
		return res, err
	}
	res.MD5 = sum
	res.SizeBytes = size

	switch normalizeExt(ext, path) {
	case "xapk":
		if err := parseXAPK(path, &res); err != nil {
			return res, err
		}
	case "apk":
		if err := parseAPK(path, &res); err != nil {
			return res, err
		}
	default:
		return res, fmt.Errorf("apkparse: unsupported extension %q", ext)
	}
	return res, nil
}

// hashAndSize streams the file once, computing the MD5 (lowercase 32-hex)
// and the byte count via the number of bytes io.Copy moves.
func hashAndSize(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("apkparse: open: %w", err)
	}
	defer f.Close()

	h := md5.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, fmt.Errorf("apkparse: read: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

func normalizeExt(ext, path string) string {
	e := ext
	if e == "" {
		e = filepath.Ext(path)
	}
	e = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(e)), ".")
	return e
}

// parseAPK fills package/version/label/icon from an .apk (zip) file.
// Missing label or icon are tolerated (left empty, no error); only a
// fundamentally unreadable apk (bad zip / unparseable manifest) errors.
func parseAPK(path string, res *ParsedApp) error {
	pkg, err := apk.OpenFile(path)
	if err != nil {
		return fmt.Errorf("apkparse: open apk: %w", err)
	}
	defer pkg.Close()

	m := pkg.Manifest()
	res.PackageName = pkg.PackageName()
	if v, err := m.VersionName.String(); err == nil {
		res.Version = v
	}

	// Label resolves via the resource table; tolerate failure.
	if label, err := pkg.Label(nil); err == nil && label != "" {
		res.AppName = label
	}

	// Resolve the launcher icon via the resource table with explicit density
	// configs (highest first). pkg.Icon(nil) frequently resolves android:icon to
	// the adaptive-icon XML (anydpi-v26 → "unknown format"), and modern apks ship
	// path-shortened resources (res/XX.png) that name-based scans can't find — but
	// a density config makes the table return the density-specific raster. WebP is
	// decodable because we register golang.org/x/image/webp globally.
	res.IconPNG = iconFromResTable(pkg)

	// Last-resort fallback: scan the zip for a raster icon named *ic_launcher*.
	if len(res.IconPNG) == 0 {
		if data := scanZipForLauncherIcon(path); len(data) > 0 {
			res.IconPNG = data
		}
	}
	if len(res.IconPNG) == 0 {
		log.Printf("[apkparse] NO ICON resolved for pkg=%s (path=%s)", res.PackageName, filepath.Base(path))
	}
	return nil
}

// iconResConfigs are the density configs tried (highest density first) to coax
// the resource table into returning a density-specific raster launcher icon
// rather than the adaptive-icon XML. A trailing nil falls back to the default.
var iconResConfigs = []*androidbinary.ResTableConfig{
	{Density: 640}, {Density: 480}, {Density: 320}, {Density: 240}, {Density: 160}, nil,
}

// iconFromResTable resolves android:icon for each density config and returns the
// first one that decodes to a raster image, PNG-encoded. Empty on failure.
func iconFromResTable(pkg *apk.Apk) []byte {
	for _, cfg := range iconResConfigs {
		img, err := pkg.Icon(cfg)
		if err != nil || img == nil {
			continue
		}
		if data, err := encodePNG(img); err == nil && len(data) > 0 {
			return data
		}
	}
	return nil
}

// parseXAPK reads metadata from an .xapk bundle. The bundle is a zip whose
// manifest.json carries package_name/version_name/name and an icon path.
// When those fields are present we use them and the bundled icon; when a
// field is missing we extract the base apk entry and fall back to the apk
// parser to fill the gaps.
func parseXAPK(path string, res *ParsedApp) error {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("apkparse: open xapk: %w", err)
	}
	defer zr.Close()

	man, _ := readXAPKManifest(&zr.Reader) // tolerate missing/invalid manifest.json

	res.PackageName = man.PackageName
	res.Version = man.VersionName
	res.AppName = man.Name

	// Icon: read the entry named by the manifest (default icon.png),
	// re-encoding to PNG when it is not already PNG.
	iconName := man.Icon
	if iconName == "" {
		iconName = "icon.png"
	}
	if data, ok := readZipEntry(&zr.Reader, iconName); ok {
		if png, err := toPNG(data); err == nil {
			res.IconPNG = png
		}
	}

	// If any core field is missing, fall back to parsing the bundled base
	// apk. This also recovers a missing icon.
	if res.PackageName == "" || res.Version == "" || res.AppName == "" || len(res.IconPNG) == 0 {
		fillFromBaseAPK(&zr.Reader, man, res)
	}

	// Fallback: still no icon (manifest icon missing and base apk couldn't
	// rasterize an adaptive/WebP icon). Scan the xapk zip for a launcher icon.
	if len(res.IconPNG) == 0 {
		if data := scanZipForLauncherIcon(path); len(data) > 0 {
			res.IconPNG = data
		}
	}
	return nil
}

// xapkManifest is the subset of manifest.json we consume.
type xapkManifest struct {
	PackageName string `json:"package_name"`
	VersionName string `json:"version_name"`
	Name        string `json:"name"`
	Icon        string `json:"icon"`
	// SplitApks lists the apks bundled in the xapk. The entry with id
	// "base" (when present) is the base apk used for fallback parsing.
	SplitApks []struct {
		ID   string `json:"id"`
		File string `json:"file"`
	} `json:"split_apks"`
}

func readXAPKManifest(zr *zip.Reader) (xapkManifest, error) {
	var man xapkManifest
	data, ok := readZipEntry(zr, "manifest.json")
	if !ok {
		return man, fmt.Errorf("apkparse: manifest.json not found")
	}
	if err := json.Unmarshal(data, &man); err != nil {
		return man, fmt.Errorf("apkparse: manifest.json: %w", err)
	}
	return man, nil
}

// fillFromBaseAPK extracts the base apk entry from the xapk to a temp file
// and parses it, filling only the fields still empty in res.
func fillFromBaseAPK(zr *zip.Reader, man xapkManifest, res *ParsedApp) {
	name := baseAPKName(zr, man)
	if name == "" {
		return
	}
	data, ok := readZipEntry(zr, name)
	if !ok {
		return
	}

	tmp, err := os.CreateTemp("", "apkparse-base-*.apk")
	if err != nil {
		return
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return
	}
	if err := tmp.Close(); err != nil {
		return
	}

	var sub ParsedApp
	if err := parseAPK(tmpName, &sub); err != nil {
		return
	}
	if res.PackageName == "" {
		res.PackageName = sub.PackageName
	}
	if res.Version == "" {
		res.Version = sub.Version
	}
	if res.AppName == "" {
		res.AppName = sub.AppName
	}
	if len(res.IconPNG) == 0 {
		res.IconPNG = sub.IconPNG
	}
}

// baseAPKName resolves the base apk entry name inside the xapk: it prefers
// the split with id "base", then "<package_name>.apk", then any top-level
// ".apk" entry.
func baseAPKName(zr *zip.Reader, man xapkManifest) string {
	for _, s := range man.SplitApks {
		if s.ID == "base" && s.File != "" {
			return s.File
		}
	}
	if man.PackageName != "" {
		candidate := man.PackageName + ".apk"
		if _, ok := zipFile(zr, candidate); ok {
			return candidate
		}
	}
	for _, f := range zr.File {
		if strings.EqualFold(filepath.Ext(f.Name), ".apk") && !strings.Contains(f.Name, "/") {
			return f.Name
		}
	}
	return ""
}

func zipFile(zr *zip.Reader, name string) (*zip.File, bool) {
	for _, f := range zr.File {
		if f.Name == name {
			return f, true
		}
	}
	return nil, false
}

func readZipEntry(zr *zip.Reader, name string) ([]byte, bool) {
	f, ok := zipFile(zr, name)
	if !ok {
		return nil, false
	}
	rc, err := f.Open()
	if err != nil {
		return nil, false
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, false
	}
	return data, true
}

// scanZipForLauncherIcon opens the apk/xapk at path as a zip and looks for a
// raster launcher icon among res/mipmap*/ and res/drawable* entries whose name
// contains "ic_launcher" (case-insensitive). It only considers decodable
// raster formats (.png/.webp/.jpg/.jpeg) — adaptive-icon XML drawables are
// skipped — prefers the largest such entry, decodes it and re-encodes to PNG.
// All failures are tolerated and yield a nil result.
func scanZipForLauncherIcon(path string) []byte {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil
	}
	defer zr.Close()

	var best *zip.File
	for _, f := range zr.File {
		if !isLauncherIconEntry(f.Name) {
			continue
		}
		if best == nil || f.UncompressedSize64 > best.UncompressedSize64 {
			best = f
		}
	}
	if best == nil {
		return nil
	}

	rc, err := best.Open()
	if err != nil {
		return nil
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil
	}

	png, err := toPNG(data)
	if err != nil {
		return nil
	}
	return png
}

// isLauncherIconEntry reports whether a zip entry name is a raster launcher
// icon under res/mipmap*/ or res/drawable*. Only .png/.webp/.jpg/.jpeg are
// accepted (XML adaptive-icon drawables are excluded).
func isLauncherIconEntry(name string) bool {
	lower := strings.ToLower(filepath.ToSlash(name))
	if !strings.Contains(lower, "ic_launcher") {
		return false
	}
	if !(strings.HasPrefix(lower, "res/mipmap") || strings.HasPrefix(lower, "res/drawable")) {
		return false
	}
	switch {
	case strings.HasSuffix(lower, ".png"),
		strings.HasSuffix(lower, ".webp"),
		strings.HasSuffix(lower, ".jpg"),
		strings.HasSuffix(lower, ".jpeg"):
		return true
	default:
		return false
	}
}

// toPNG returns data unchanged when it is already valid PNG, otherwise it
// decodes (any registered format) and re-encodes to PNG.
func toPNG(data []byte) ([]byte, error) {
	if _, err := png.Decode(bytes.NewReader(data)); err == nil {
		return data, nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return encodePNG(img)
}

func encodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
