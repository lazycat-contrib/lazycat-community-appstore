package lazycatpkg

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestSynchronousInstallLPKRequest(t *testing.T) {
	req := synchronousInstallLPKRequest(InstallRequest{
		DownloadURL: "https://download.example/lark.lpk",
		SHA256:      "checksum",
		PackageID:   "community.lazycat.app.lark",
		Name:        "Lark Music",
	})

	if !req.GetWaitUnitDone() {
		t.Fatal("synchronous install request does not wait for completion")
	}
	if req.GetPkgId() != "community.lazycat.app.lark" {
		t.Fatalf("package ID = %q", req.GetPkgId())
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

func TestWithIdentityForwardsUserAndDevice(t *testing.T) {
	ctx := withIdentity(context.Background(), Identity{UserID: "local", DeviceID: "pc-1"})
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("outgoing metadata is missing")
	}
	if got := md.Get("x-hc-user-id"); len(got) != 1 || got[0] != "local" {
		t.Fatalf("user metadata = %#v", got)
	}
	if got := md.Get("x-hc-device-id"); len(got) != 1 || got[0] != "pc-1" {
		t.Fatalf("device metadata = %#v", got)
	}
}
