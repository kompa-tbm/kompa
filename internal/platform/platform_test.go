package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCurrent(t *testing.T) {
	p := Current()
	if p.OS == Unknown {
		t.Errorf("Current().OS = Unknown, want a known OS")
	}
	if p.Arch == UnknownArch {
		t.Errorf("Current().Arch = UnknownArch, want a known arch")
	}
}

func TestPlatformString(t *testing.T) {
	p := Platform{OS: Linux, Arch: AMD64}
	if got := p.String(); got != "linux-amd64" {
		t.Errorf("Platform.String() = %q, want %q", got, "linux-amd64")
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		input   string
		wantOS  OS
		wantArch Arch
		wantErr bool
	}{
		{"linux-amd64", Linux, AMD64, false},
		{"darwin-arm64", Darwin, ARM64, false},
		{"windows-amd64", Windows, AMD64, false},
		{"linux-386", Linux, I386, false},
		{"bad", Unknown, UnknownArch, true},
		{"linux-unknown", Unknown, UnknownArch, true},
		{"unknown-amd64", Unknown, UnknownArch, true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := Parse(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Parse(%q) = no error, want error", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("Parse(%q) error = %v", tt.input, err)
				return
			}
			if got.OS != tt.wantOS {
				t.Errorf("Parse(%q).OS = %q, want %q", tt.input, got.OS, tt.wantOS)
			}
			if got.Arch != tt.wantArch {
				t.Errorf("Parse(%q).Arch = %q, want %q", tt.input, got.Arch, tt.wantArch)
			}
		})
	}
}

func TestNormalizeArch(t *testing.T) {
	tests := []struct {
		input string
		want  Arch
	}{
		{"amd64", AMD64},
		{"x86_64", AMD64},
		{"arm64", ARM64},
		{"aarch64", ARM64},
		{"386", I386},
		{"i386", I386},
		{"i686", I386},
		{"x86", I386},
		{"riscv64", RISCV64},
		{"mips", UnknownArch},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeArch(tt.input)
			if got != tt.want {
				t.Errorf("normalizeArch(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGetDirs(t *testing.T) {
	// Override via env var.
	tmp := t.TempDir()
	t.Setenv("KOMPA_HOME", tmp)

	dirs, err := GetDirs()
	if err != nil {
		t.Fatalf("GetDirs() error = %v", err)
	}
	if dirs.Root != tmp {
		t.Errorf("dirs.Root = %q, want %q", dirs.Root, tmp)
	}
	if dirs.Bin != filepath.Join(tmp, "bin") {
		t.Errorf("dirs.Bin = %q, want %q", dirs.Bin, filepath.Join(tmp, "bin"))
	}
}

func TestEnsureDirs(t *testing.T) {
	tmp := t.TempDir()
	dirs := GetDirsFromRoot(tmp)

	if err := EnsureDirs(dirs); err != nil {
		t.Fatalf("EnsureDirs() error = %v", err)
	}

	for _, d := range []string{dirs.Root, dirs.Bin, dirs.Packages, dirs.Cache, dirs.Downloads} {
		fi, err := os.Stat(d)
		if err != nil {
			t.Errorf("directory %s not created: %v", d, err)
			continue
		}
		if !fi.IsDir() {
			t.Errorf("%s is not a directory", d)
		}
	}
}

func TestExeSuffix(t *testing.T) {
	suffix := ExeSuffix()
	if runtime.GOOS == "windows" {
		if suffix != ".exe" {
			t.Errorf("ExeSuffix() = %q, want .exe on windows", suffix)
		}
	} else {
		if suffix != "" {
			t.Errorf("ExeSuffix() = %q, want empty on non-windows", suffix)
		}
	}
}
