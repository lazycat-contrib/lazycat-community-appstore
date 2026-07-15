package lazycatpkg

import (
	"context"
	"strings"
	"time"

	gohelper "gitee.com/linakesi/lzc-sdk/lang/go"
	"gitee.com/linakesi/lzc-sdk/lang/go/sys"
	"google.golang.org/grpc/metadata"
)

const installTimeout = 60 * time.Second

type Identity struct {
	UserID   string
	DeviceID string
}

type InstallRequest struct {
	DownloadURL string
	SHA256      string
	PackageID   string
	Name        string
}

type InstallResult struct {
	Mode   string `json:"mode"`
	TaskID string `json:"taskId,omitempty"`
	Status string `json:"status,omitempty"`
	Detail string `json:"detail,omitempty"`
}

func InstallLPK(ctx context.Context, identity Identity, req InstallRequest) (InstallResult, error) {
	ctx = withIdentity(ctx, identity)
	ctx, cancel := context.WithTimeout(ctx, installTimeout)
	defer cancel()

	gw, err := gohelper.NewAPIGateway(ctx)
	if err != nil {
		return InstallResult{}, err
	}
	defer func() { _ = gw.Close() }()

	resp, err := gw.PkgManager.InstallLPK(ctx, synchronousInstallLPKRequest(req))
	if err != nil {
		return InstallResult{}, err
	}
	result := InstallResult{Mode: "lazycat-go-sdk"}
	if task := resp.GetTaskInfo(); task != nil {
		result.TaskID = task.GetTaskId()
		result.Status = task.GetStatus().String()
		result.Detail = task.GetDetail()
	}
	return result, nil
}

func withIdentity(ctx context.Context, identity Identity) context.Context {
	pairs := make([]string, 0, 4)
	if userID := strings.TrimSpace(identity.UserID); userID != "" {
		pairs = append(pairs, "x-hc-user-id", userID)
	}
	if deviceID := strings.TrimSpace(identity.DeviceID); deviceID != "" {
		pairs = append(pairs, "x-hc-device-id", deviceID)
	}
	if len(pairs) == 0 {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, pairs...)
}

func synchronousInstallLPKRequest(req InstallRequest) *sys.InstallLPKRequest {
	wait := true
	in := &sys.InstallLPKRequest{LpkUrl: req.DownloadURL, WaitUnitDone: &wait}
	if req.SHA256 != "" {
		in.Sha256 = &req.SHA256
	}
	if req.PackageID != "" {
		in.PkgId = &req.PackageID
	}
	if req.Name != "" {
		in.TmpTitle = &req.Name
	}
	return in
}
