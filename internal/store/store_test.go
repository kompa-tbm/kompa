package store

import (
	"path/filepath"
	"testing"
	"time"
)

func samplePkg(name, version string) *InstalledPackage {
	return &InstalledPackage{
		Name:        name,
		Version:     version,
		OS:          "linux",
		Arch:        "amd64",
		InstallPath: "/kompa/packages/" + name + "/" + version,
		Files:       []string{"bin/" + name, "lib/lib" + name + ".so"},
		SHA256:      "aabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccdd",
		Size:        1024 * 1024,
		InstalledAt: time.Now().UTC(),
		ReleaseTag:  "v1",
		Active:      true,
		Binaries:    []string{name},
	}
}

func openTestDB(t *testing.T) *DB {
	t.Helper()
	tmp := t.TempDir()
	db, err := Open(filepath.Join(tmp, "db.json"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	return db
}

func TestOpen_NewDB(t *testing.T) {
	db := openTestDB(t)
	pkgs := db.List()
	if len(pkgs) != 0 {
		t.Errorf("new DB List() = %d packages, want 0", len(pkgs))
	}
}

func TestInstall_And_Get(t *testing.T) {
	db := openTestDB(t)
	pkg := samplePkg("gcc", "14.2.0")

	if err := db.Install(pkg); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	got := db.Get("gcc", "14.2.0", "linux", "amd64")
	if got == nil {
		t.Fatal("Get() returned nil after Install()")
	}
	if got.Name != "gcc" || got.Version != "14.2.0" {
		t.Errorf("Get() = {%s %s}, want {gcc 14.2.0}", got.Name, got.Version)
	}
	if !got.Active {
		t.Error("Get() Active = false, want true")
	}
}

func TestInstall_Persists(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "db.json")

	db1, _ := Open(dbPath)
	if err := db1.Install(samplePkg("lua", "5.4.7")); err != nil {
		t.Fatal(err)
	}

	// Reopen.
	db2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("reopen error: %v", err)
	}
	got := db2.Get("lua", "5.4.7", "linux", "amd64")
	if got == nil {
		t.Fatal("package not persisted across DB reopen")
	}
}

func TestRemove(t *testing.T) {
	db := openTestDB(t)
	pkg := samplePkg("sqlite", "3.46.1")
	_ = db.Install(pkg)

	if err := db.Remove("sqlite", "3.46.1", "linux", "amd64"); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if db.Get("sqlite", "3.46.1", "linux", "amd64") != nil {
		t.Error("Get() after Remove() should return nil")
	}
}

func TestRemove_NotInstalled(t *testing.T) {
	db := openTestDB(t)
	err := db.Remove("nonexistent", "1.0", "linux", "amd64")
	if err == nil {
		t.Error("Remove() on non-installed package: expected error, got nil")
	}
}

func TestIsInstalled(t *testing.T) {
	db := openTestDB(t)
	pkg := samplePkg("zlib", "1.3.1")
	_ = db.Install(pkg)

	if !db.IsInstalled("zlib", "linux", "amd64") {
		t.Error("IsInstalled() = false after install, want true")
	}
	if db.IsInstalled("zlib", "darwin", "arm64") {
		t.Error("IsInstalled() = true for wrong platform, want false")
	}
	if db.IsInstalled("nonexistent", "linux", "amd64") {
		t.Error("IsInstalled() = true for nonexistent package, want false")
	}
}

func TestGetLatest(t *testing.T) {
	db := openTestDB(t)

	p1 := samplePkg("gcc", "13.3.0")
	p1.InstalledAt = time.Now().Add(-time.Hour)
	_ = db.Install(p1)

	p2 := samplePkg("gcc", "14.2.0")
	p2.InstalledAt = time.Now()
	_ = db.Install(p2)

	latest := db.GetLatest("gcc", "linux", "amd64")
	if latest == nil {
		t.Fatal("GetLatest() returned nil")
	}
	if latest.Version != "14.2.0" {
		t.Errorf("GetLatest().Version = %q, want 14.2.0", latest.Version)
	}
}

func TestSetActive(t *testing.T) {
	db := openTestDB(t)

	p1 := samplePkg("gcc", "13.3.0")
	p1.Active = true
	_ = db.Install(p1)

	p2 := samplePkg("gcc", "14.2.0")
	p2.Active = false
	_ = db.Install(p2)

	// Activate 14.2.0.
	if err := db.SetActive("gcc", "14.2.0", "linux", "amd64"); err != nil {
		t.Fatalf("SetActive() error = %v", err)
	}

	active := db.GetActive("gcc", "linux", "amd64")
	if active == nil {
		t.Fatal("GetActive() returned nil after SetActive()")
	}
	if active.Version != "14.2.0" {
		t.Errorf("GetActive().Version = %q, want 14.2.0", active.Version)
	}

	// 13.3.0 should now be inactive.
	old := db.Get("gcc", "13.3.0", "linux", "amd64")
	if old == nil {
		t.Fatal("Get(13.3.0) returned nil")
	}
	if old.Active {
		t.Error("13.3.0 should be inactive after SetActive(14.2.0)")
	}
}

func TestSetActive_NotInstalled(t *testing.T) {
	db := openTestDB(t)
	err := db.SetActive("gcc", "99.0", "linux", "amd64")
	if err == nil {
		t.Error("SetActive() on missing version: expected error, got nil")
	}
}

func TestList_Sorted(t *testing.T) {
	db := openTestDB(t)
	_ = db.Install(samplePkg("zlib", "1.3.1"))
	_ = db.Install(samplePkg("gcc", "14.2.0"))
	_ = db.Install(samplePkg("lua", "5.4.7"))

	list := db.List()
	if len(list) != 3 {
		t.Fatalf("List() len = %d, want 3", len(list))
	}
	names := []string{list[0].Name, list[1].Name, list[2].Name}
	expected := []string{"gcc", "lua", "zlib"}
	for i, n := range names {
		if n != expected[i] {
			t.Errorf("List()[%d].Name = %q, want %q", i, n, expected[i])
		}
	}
}

func TestListByName(t *testing.T) {
	db := openTestDB(t)
	_ = db.Install(samplePkg("gcc", "13.3.0"))
	_ = db.Install(samplePkg("gcc", "14.2.0"))
	_ = db.Install(samplePkg("lua", "5.4.7"))

	gccVersions := db.ListByName("gcc")
	if len(gccVersions) != 2 {
		t.Errorf("ListByName(gcc) len = %d, want 2", len(gccVersions))
	}

	luaVersions := db.ListByName("lua")
	if len(luaVersions) != 1 {
		t.Errorf("ListByName(lua) len = %d, want 1", len(luaVersions))
	}

	none := db.ListByName("nonexistent")
	if len(none) != 0 {
		t.Errorf("ListByName(nonexistent) len = %d, want 0", len(none))
	}
}

func TestUpdateFiles(t *testing.T) {
	db := openTestDB(t)
	pkg := samplePkg("lua", "5.4.7")
	_ = db.Install(pkg)

	newFiles := []string{"bin/lua", "bin/luac", "lib/liblua.a", "include/lua.h"}
	if err := db.UpdateFiles("lua", "5.4.7", "linux", "amd64", newFiles); err != nil {
		t.Fatalf("UpdateFiles() error = %v", err)
	}

	got := db.Get("lua", "5.4.7", "linux", "amd64")
	if len(got.Files) != len(newFiles) {
		t.Errorf("Files after UpdateFiles = %v, want %v", got.Files, newFiles)
	}
}
