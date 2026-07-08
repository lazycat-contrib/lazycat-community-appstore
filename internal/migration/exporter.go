package migration

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"time"

	"lazycat.community/appstore/ent"
)

type Exporter struct {
	db            *ent.Client
	storage       StorageResolver
	serverVersion string
	now           func() time.Time
}

func NewExporter(db *ent.Client, storage StorageResolver, serverVersion string) *Exporter {
	return &Exporter{
		db:            db,
		storage:       storage,
		serverVersion: serverVersion,
		now:           func() time.Time { return time.Now().UTC() },
	}
}

func (e *Exporter) Export(ctx context.Context, w io.Writer, options Options) (*Manifest, error) {
	if e == nil || e.db == nil {
		return nil, fmt.Errorf("migration exporter is not configured")
	}
	options = NormalizeOptions(options)
	if !options.IncludeSite && !options.IncludePeople && !options.IncludeApps && !options.IncludeFiles {
		return nil, fmt.Errorf("at least one migration module is required")
	}

	var siteData SiteData
	var peopleData PeopleData
	var appsData AppsData
	var err error
	counts := map[string]int{}

	if options.IncludeSite {
		siteData, err = collectSiteData(ctx, e.db)
		if err != nil {
			return nil, fmt.Errorf("collect site data: %w", err)
		}
		mergeCounts(counts, siteCounts(siteData))
	}
	if options.IncludePeople {
		peopleData, err = collectPeopleData(ctx, e.db)
		if err != nil {
			return nil, fmt.Errorf("collect people data: %w", err)
		}
		mergeCounts(counts, peopleCounts(peopleData))
	}
	if options.IncludeApps {
		appsData, err = collectAppsData(ctx, e.db)
		if err != nil {
			return nil, fmt.Errorf("collect app data: %w", err)
		}
		mergeCounts(counts, appsCounts(appsData))
	}

	var payloads []filePayload
	warnings := []string{}
	if options.IncludeFiles {
		payloads, warnings = collectFilePayloads(ctx, e.storage, peopleData, appsData, siteData)
		counts["files"] = len(payloads)
	}

	manifest := &Manifest{
		FormatVersion: FormatVersion,
		ServerVersion: e.serverVersion,
		CreatedAt:     e.now(),
		Modules:       options.Modules(),
		Counts:        counts,
		Warnings:      warnings,
	}
	for _, payload := range payloads {
		manifest.Files = append(manifest.Files, payload.Manifest)
		manifest.TotalFileBytes += payload.Manifest.Size
	}

	zw := zip.NewWriter(w)
	if err := writeJSONEntry(zw, manifestName, manifest); err != nil {
		_ = zw.Close()
		return nil, err
	}
	if options.IncludeSite {
		if err := writeJSONEntry(zw, "data/site.json", siteData); err != nil {
			_ = zw.Close()
			return nil, err
		}
	}
	if options.IncludePeople {
		if err := writeJSONEntry(zw, "data/people.json", peopleData); err != nil {
			_ = zw.Close()
			return nil, err
		}
	}
	if options.IncludeApps {
		if err := writeJSONEntry(zw, "data/apps.json", appsData); err != nil {
			_ = zw.Close()
			return nil, err
		}
	}
	if options.IncludeFiles {
		if err := writeFilePayloads(zw, payloads); err != nil {
			_ = zw.Close()
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return manifest, nil
}
