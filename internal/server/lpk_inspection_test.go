package server

import (
	"testing"
	"time"

	"lazycat.community/appstore/ent/lpkinspectionjob"
)

func TestLPKInspectionJobLifecycle(t *testing.T) {
	store := newTestApp(t)
	job := store.server.db.LPKInspectionJob.Create().
		SetAppID(1).
		SetUserID(1).
		SetDownloadURL("https://example.test/app.lpk").
		SetTrigger(lpkinspectionjob.TriggerAPI_TOKEN_FIRST_SUBMISSION).
		SetState(lpkinspectionjob.StatePENDING).
		SetDeadlineAt(time.Now().Add(30 * time.Second)).
		SaveX(t.Context())
	if job.Attempts != 0 || job.State != lpkinspectionjob.StatePENDING {
		t.Fatalf("job = %#v", job)
	}
}
