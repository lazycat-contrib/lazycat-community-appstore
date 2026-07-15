package clientserver

import (
	"context"
	"errors"
	"time"

	gohelper "gitee.com/linakesi/lzc-sdk/lang/go"
	"gitee.com/linakesi/lzc-sdk/lang/go/sys"
	"google.golang.org/grpc/metadata"

	"lazycat.community/appstore/internal/lazycatpkg"
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
	defer func() { _ = gw.Close() }()
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
	identityUserID := userID
	if identityUserID == clientIdentityLocal {
		identityUserID = ""
	}
	result, err := lazycatpkg.InstallLPK(ctx, lazycatpkg.Identity{UserID: identityUserID}, lazycatpkg.InstallRequest{
		DownloadURL: req.DownloadURL,
		SHA256:      req.SHA256,
		PackageID:   req.PackageID,
		Name:        req.Name,
	})
	if err != nil {
		return InstallResultDTO{}, err
	}
	return InstallResultDTO(result), nil
}

func (lazyCatPackageManager) GetInstallTask(ctx context.Context, userID, taskID string) (InstallTaskDTO, error) {
	ctx, cancel := context.WithTimeout(lazycatContext(ctx, userID), 10*time.Second)
	defer cancel()
	gw, err := gohelper.NewAPIGateway(ctx)
	if err != nil {
		return InstallTaskDTO{}, err
	}
	defer func() { _ = gw.Close() }()
	resp, err := gw.PkgManager.QueryPendingTask(ctx, &sys.QueryPendingTaskRequest{})
	if err != nil {
		return InstallTaskDTO{}, err
	}
	for _, task := range resp.GetInfos() {
		if task.GetTaskId() != taskID {
			continue
		}
		return InstallTaskDTO{
			TaskID:         task.GetTaskId(),
			Status:         task.GetStatus().String(),
			DownloadedSize: task.GetDownloadedSize(),
			TotalSize:      task.TotalSize,
			Detail:         task.GetDetail(),
		}, nil
	}
	return InstallTaskDTO{}, errors.New("install task not found")
}

func (lazyCatPackageManager) CancelInstall(ctx context.Context, userID, taskID string) error {
	ctx, cancel := context.WithTimeout(lazycatContext(ctx, userID), 10*time.Second)
	defer cancel()
	gw, err := gohelper.NewAPIGateway(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = gw.Close() }()
	_, err = gw.PkgManager.CancelPendingTask(ctx, &sys.CancelPendingTaskRequest{TaskId: taskID})
	return err
}

func (unavailablePackageManager) QueryInstalled(context.Context, string) ([]InstalledApplicationDTO, error) {
	return nil, errors.New("system SDK is unavailable")
}

func (unavailablePackageManager) InstallLPK(context.Context, string, InstallRequestDTO) (InstallResultDTO, error) {
	return InstallResultDTO{}, errors.New("system SDK is unavailable")
}

func (unavailablePackageManager) GetInstallTask(context.Context, string, string) (InstallTaskDTO, error) {
	return InstallTaskDTO{}, errors.New("system SDK is unavailable")
}

func (unavailablePackageManager) CancelInstall(context.Context, string, string) error {
	return errors.New("system SDK is unavailable")
}
