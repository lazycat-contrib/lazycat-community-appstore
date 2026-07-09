package clientserver

import (
	"context"
	"errors"
	"time"

	gohelper "gitee.com/linakesi/lzc-sdk/lang/go"
	"gitee.com/linakesi/lzc-sdk/lang/go/sys"
	"google.golang.org/grpc/metadata"
)

type lazyCatPackageManager struct{}

type unavailablePackageManager struct{}

func NewLazyCatPackageManager() PackageManager {
	return lazyCatPackageManager{}
}

func lazycatContext(ctx context.Context, userID string) context.Context {
	if userID == "" || userID == "local" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "x-hc-user-id", userID)
}

func (lazyCatPackageManager) QueryInstalled(ctx context.Context, userID string) ([]InstalledApplicationDTO, error) {
	ctx, cancel := context.WithTimeout(lazycatContext(ctx, userID), 10*time.Second)
	defer cancel()
	gw, err := gohelper.NewAPIGateway(ctx)
	if err != nil {
		return nil, err
	}
	defer gw.Close()
	resp, err := gw.PkgManager.QueryApplication(ctx, &sys.QueryApplicationRequest{})
	if err != nil {
		return nil, err
	}
	out := make([]InstalledApplicationDTO, 0, len(resp.GetInfoList()))
	for _, item := range resp.GetInfoList() {
		out = append(out, InstalledApplicationDTO{
			AppID:          item.GetAppid(),
			Title:          item.GetTitle(),
			Version:        item.GetVersion(),
			Status:         item.GetStatus().String(),
			InstanceStatus: item.GetInstanceStatus().String(),
			Icon:           item.GetIcon(),
		})
	}
	return out, nil
}

func (lazyCatPackageManager) InstallLPK(ctx context.Context, userID string, req InstallRequestDTO) (InstallResultDTO, error) {
	ctx, cancel := context.WithTimeout(lazycatContext(ctx, userID), 60*time.Second)
	defer cancel()
	gw, err := gohelper.NewAPIGateway(ctx)
	if err != nil {
		return InstallResultDTO{}, err
	}
	defer gw.Close()
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
	resp, err := gw.PkgManager.InstallLPK(ctx, in)
	if err != nil {
		return InstallResultDTO{}, err
	}
	result := InstallResultDTO{Mode: "lazycat-go-sdk"}
	if task := resp.GetTaskInfo(); task != nil {
		result.TaskID = task.GetTaskId()
		result.Status = task.GetStatus().String()
		result.Detail = task.GetDetail()
	}
	return result, nil
}

func (unavailablePackageManager) QueryInstalled(context.Context, string) ([]InstalledApplicationDTO, error) {
	return nil, errors.New("system SDK is unavailable")
}

func (unavailablePackageManager) InstallLPK(context.Context, string, InstallRequestDTO) (InstallResultDTO, error) {
	return InstallResultDTO{}, errors.New("system SDK is unavailable")
}
