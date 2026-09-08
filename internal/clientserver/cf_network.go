package clientserver

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/clientsetting"
	"lazycat.community/appstore/ent/clientsource"
	"lazycat.community/appstore/internal/cfnetwork"
)

const settingCFEnabled = "cf_enabled"
const settingCFEndpoint = "cf_endpoint"

type CFPresetDTO struct {
	Endpoint   string `json:"endpoint"`
	SourceName string `json:"sourceName"`
}

func cfPresetsKey(id int) string { return "cf_presets_" + strconv.Itoa(id) }

func saveCFPresets(ctx context.Context, tx *ent.Tx, source *ent.ClientSource, endpoints []string) error {
	key := cfPresetsKey(source.ID)
	value := strings.Join(endpoints, "\n")
	record, err := tx.ClientSetting.Query().Where(clientsetting.UserIDEQ(source.UserID), clientsetting.KeyEQ(key)).Only(ctx)
	if ent.IsNotFound(err) {
		return tx.ClientSetting.Create().SetUserID(source.UserID).SetKey(key).SetValue(value).Exec(ctx)
	}
	if err != nil {
		return err
	}
	return tx.ClientSetting.UpdateOneID(record.ID).SetValue(value).Exec(ctx)
}

func (s *Server) saveCFSettings(r *http.Request, input ClientSettingsUpdateDTO) error {
	if input.CFEndpoint != nil {
		if err := s.setClientSetting(r, settingCFEndpoint, *input.CFEndpoint); err != nil {
			return err
		}
	}
	if input.CFEnabled != nil {
		return s.setClientSetting(r, settingCFEnabled, strconv.FormatBool(*input.CFEnabled))
	}
	return nil
}

func (s *Server) applyCFSettings(ctx context.Context, userID string, dto *ClientSettingsDTO) {
	dto.CFEnabled = s.clientSetting(ctx, userID, settingCFEnabled) == "true"
	dto.CFEndpoint = s.clientSetting(ctx, userID, settingCFEndpoint)
	if dto.CFEndpoint == "" {
		dto.CFEndpoint = cfnetwork.DefaultEndpoint
	}
	dto.CFPresets = []CFPresetDTO{{Endpoint: cfnetwork.DefaultEndpoint, SourceName: ""}}
	sources, err := s.db.ClientSource.Query().Where(clientsource.UserIDEQ(userID)).Order(ent.Asc(clientsource.FieldID)).All(ctx)
	if err != nil {
		return
	}
	seen := map[string]bool{cfnetwork.DefaultEndpoint: true}
	for _, source := range sources {
		endpoints, _ := cfnetwork.ParseList(s.clientSetting(ctx, userID, cfPresetsKey(source.ID)))
		for _, endpoint := range endpoints {
			if seen[endpoint] {
				continue
			}
			dto.CFPresets = append(dto.CFPresets, CFPresetDTO{Endpoint: endpoint, SourceName: source.Name})
			seen[endpoint] = true
		}
	}
}

func (s *Server) sourceHTTPClient(ctx context.Context, source *ent.ClientSource, base *http.Client) *http.Client {
	origin, err := url.Parse(source.URL)
	if err != nil || origin.Scheme != "https" || s.clientSetting(ctx, source.UserID, settingCFEnabled) != "true" {
		return base
	}
	endpoint := s.clientSetting(ctx, source.UserID, settingCFEndpoint)
	if endpoint == "" {
		endpoint = cfnetwork.DefaultEndpoint
	}
	endpoint, err = cfnetwork.Normalize(endpoint)
	if err != nil {
		return base
	}
	clone := *base
	direct := base.Transport
	if direct == nil {
		direct = http.DefaultTransport
	}
	clone.Transport = cfnetwork.OriginTransport{Origin: origin.Host, Preferred: s.cfTransports.Transport(endpoint), Direct: direct}
	return &clone
}
