package clientserver

import "testing"

func TestAsyncInstallLPKRequestDoesNotPinInstalledPackageID(t *testing.T) {
	req := asyncInstallLPKRequest(InstallRequestDTO{
		DownloadURL: "https://download.example/lark.lpk",
		SHA256:      "checksum",
		PackageID:   "community.lazycat.app.lark",
		Name:        "Lark Music",
	})

	if req.GetWaitUnitDone() {
		t.Fatal("async install request waits for completion")
	}
	if req.PkgId != nil {
		t.Fatalf("async install request pins package ID %q", req.GetPkgId())
	}
	if req.GetLpkUrl() != "https://download.example/lark.lpk" {
		t.Fatalf("download URL = %q", req.GetLpkUrl())
	}
	if req.GetSha256() != "checksum" {
		t.Fatalf("sha256 = %q", req.GetSha256())
	}
	if req.GetTmpTitle() != "Lark Music" {
		t.Fatalf("temporary title = %q", req.GetTmpTitle())
	}
}
